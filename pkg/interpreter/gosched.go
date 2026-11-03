package interpreter

import (
	"fmt"
	"runtime/debug"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// Blocking and resumption of interpreted goroutines (#294).
//
// The interpreter is recursive on the Go stack: execFunction → execBlock →
// execInstruction, with intercepts re-entering execFunction, and no stored
// instruction pointer. An interpreted goroutine that blocks must keep that
// stack, so each one that has started runs on its own host goroutine. Exactly
// one host goroutine runs at a time; the others are parked on their wake
// channel. Control passes like a baton: a goroutine runs until it blocks or
// exits, then picks the next runnable goroutine and hands over.
//
// The driver is the host goroutine that called Run, runTestFn or
// Session.call. It runs the entry goroutine (main, a test function, a session
// call) itself, so a program that never starts a goroutine never creates a
// host goroutine. When nothing is runnable (quiescence), control returns to
// the driver.
//
// Between handing the baton over and receiving it back, a host goroutine
// touches no interpreter state. The channel operations order every access, so
// the interpreter is race-free under the Go race detector.

// wakeMsg is what a parked host goroutine is woken with.
type wakeMsg uint8

const (
	wakeRun       wakeMsg = iota // run: the goroutine's wait is over
	wakeAbort                    // unwind without running anything more
	wakeQuiescent                // driver only: nothing is runnable
	wakeExited                   // driver only: an aborted goroutine has unwound
	wakeCrash                    // driver only: a host goroutine panicked
)

// hostG is the host goroutine of a started interpreted goroutine.
type hostG struct {
	wake chan wakeMsg
}

// waitInfo describes what a goroutine is about to wait for.
type waitInfo struct {
	site   string
	kind   string // GoroutineLeakError.BlockKind
	chanID ChanID
	onSend bool
	// ready reports whether the wait is over. nil means never: a nil
	// channel, an empty select.
	ready func() bool
}

// errHostCrash carries an internal panic from a host goroutine to the driver.
type errHostCrash struct {
	val   any
	stack []byte
}

func (e *errHostCrash) Error() string {
	return fmt.Sprintf("interpreter: goroutine panicked: %v\n%s", e.val, e.stack)
}

// runDriver runs body, which interprets goroutine gid on the calling host
// goroutine, then schedules every other goroutine until none is runnable and
// aborts those still waiting. No host goroutine outlives runDriver.
func (interp *Interpreter) runDriver(gid int64, body func()) {
	if interp.driverHost == nil {
		interp.driverHost = &hostG{wake: make(chan wakeMsg, 1)}
	}
	if interp.hosts == nil {
		interp.hosts = make(map[int64]*hostG)
	}
	interp.driverGID = gid
	interp.hosts[gid] = interp.driverHost
	body()
	interp.finishGoroutine(gid)
	interp.drainAll()
	interp.abortAll()
	delete(interp.hosts, gid)
	interp.runQueue = interp.runQueue[:0]
}

// block parks goroutine gid until w.ready holds, running other goroutines
// meanwhile. It returns true once the wait is over, and false if the
// goroutine was aborted instead: the caller must then return without
// completing the operation, and the goroutine unwinds (its Status stays
// GoroutineBlocked, which every execution loop checks).
func (interp *Interpreter) block(gid int64, w waitInfo) bool {
	g := interp.goroutine(gid)
	if g == nil || g.Aborted {
		return false
	}
	if w.ready != nil && w.ready() {
		return true
	}
	g.Status = GoroutineBlocked
	g.BlockSite, g.BlockKind, g.BlockChanID, g.BlockOnSend = w.site, w.kind, w.chanID, w.onSend
	g.wait = w.ready
	if w.ready == nil {
		g.wait = never
	}

	// Inside a probe (a callback giri runs early, which native Go would run
	// later), the goroutines that could end the wait may not exist yet:
	// unwind to the probe instead of parking.
	if interp.probeDepth > 0 {
		return false
	}

	site := interp.currentSite
	me := interp.hosts[gid]
	var msg wakeMsg
	if next := interp.pickNext(gid); next >= 0 {
		interp.transfer(next)
		msg = <-me.wake
	} else if gid == interp.driverGID {
		msg = wakeQuiescent
	} else {
		interp.driverHost.wake <- wakeQuiescent
		msg = <-me.wake
	}

	switch msg {
	case wakeRun:
		g.Status = GoroutineRunning
		g.wait = nil
		interp.currentSite = site
		return true
	case wakeQuiescent:
		// The driver's goroutine waits and nothing can run: every goroutine
		// is asleep, which native Go reports as a fatal deadlock.
		interp.reportDeadlock()
		interp.halting = true
		interp.abortParked()
		g.Aborted = true
		return false
	case wakeCrash:
		panic(interp.crash)
	}
	g.Aborted = true // wakeAbort
	return false
}

// never is the readiness predicate of a wait that can't end.
func never() bool { return false }

// pickNext returns the goroutine to run next, or -1: unstarted goroutines and
// waiting goroutines whose wait is over, chosen by the scheduler.
func (interp *Interpreter) pickNext(self int64) int64 {
	if interp.halting || interp.sched == nil {
		return -1
	}
	n := len(interp.runQueue)
	if n == 0 && (len(interp.hosts) == 0 || len(interp.hosts) == 1 && interp.hosts[self] != nil) {
		return -1
	}
	cands := make([]int64, 0, n+len(interp.hosts))
	for _, t := range interp.runQueue {
		cands = append(cands, t.gid)
	}
	for id := range interp.hosts {
		if id == self {
			continue
		}
		if g := interp.goroutines[id]; g != nil && g.Status == GoroutineBlocked && !g.Aborted && g.wait != nil && g.wait() {
			cands = append(cands, id)
		}
	}
	if len(cands) == 0 {
		return -1
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i] < cands[j] })
	return interp.sched.Next(cands)
}

// transfer hands the baton to goroutine next, starting its host goroutine if
// it hasn't run yet. The caller must not touch interpreter state afterwards
// until it is woken.
func (interp *Interpreter) transfer(next int64) {
	if h := interp.hosts[next]; h != nil {
		h.wake <- wakeRun
		return
	}
	t, ok := interp.takeTask(next)
	if !ok {
		panic(fmt.Sprintf("interpreter: scheduler picked goroutine %d, which is neither queued nor waiting", next))
	}
	h := &hostG{wake: make(chan wakeMsg, 1)}
	interp.hosts[next] = h
	go interp.hostMain(h, t)
}

// takeTask removes goroutine gid's task from the run queue.
func (interp *Interpreter) takeTask(gid int64) (goroutineTask, bool) {
	for i, t := range interp.runQueue {
		if t.gid == gid {
			last := len(interp.runQueue) - 1
			interp.runQueue[i] = interp.runQueue[last]
			interp.runQueue = interp.runQueue[:last]
			return t, true
		}
	}
	return goroutineTask{}, false
}

// hostMain runs one interpreted goroutine on its own host goroutine.
func (interp *Interpreter) hostMain(h *hostG, t goroutineTask) {
	defer func() {
		if r := recover(); r != nil {
			interp.crash = &errHostCrash{val: r, stack: debug.Stack()}
			interp.driverHost.wake <- wakeCrash
		}
	}()
	interp.execFunction(t.gid, t.fn, t.args)
	g := interp.goroutine(t.gid)
	delete(interp.hosts, t.gid)
	if g != nil && g.Aborted {
		interp.driverHost.wake <- wakeExited
		return
	}
	if t.onExit != nil {
		t.onExit(t.gid)
	}
	interp.finishGoroutine(t.gid)
	if next := interp.pickNext(t.gid); next >= 0 {
		interp.transfer(next)
		return
	}
	interp.driverHost.wake <- wakeQuiescent
}

// drainAll runs goroutines from the driver until none is runnable.
func (interp *Interpreter) drainAll() {
	for {
		next := interp.pickNext(interp.driverGID)
		if next < 0 {
			return
		}
		interp.transfer(next)
		if msg := <-interp.driverHost.wake; msg == wakeCrash {
			panic(interp.crash)
		}
	}
}

// abortAll aborts every goroutine still waiting, from the driver, and drops
// goroutines that never started.
func (interp *Interpreter) abortAll() {
	interp.abortParked()
	interp.runQueue = interp.runQueue[:0]
}

// abortParked wakes each parked goroutine other than the driver's with
// wakeAbort, in goroutine order, and waits for it to unwind.
func (interp *Interpreter) abortParked() {
	if len(interp.hosts) <= 1 {
		return
	}
	ids := make([]int64, 0, len(interp.hosts))
	for id := range interp.hosts {
		if id != interp.driverGID {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		interp.hosts[id].wake <- wakeAbort
		if msg := <-interp.driverHost.wake; msg == wakeCrash {
			panic(interp.crash)
		}
	}
}

// haltAll stops every goroutine: os.Exit, log.Fatal, the step limit. Nothing
// else runs, and goroutines still waiting are not reported as leaks.
func (interp *Interpreter) haltAll() {
	interp.halting = true
	for _, g := range interp.goroutines {
		g.Panicked = true
	}
}

// reportDeadlock records that every goroutine is asleep.
func (interp *Interpreter) reportDeadlock() {
	if interp.halting || interp.deadlock != nil {
		return
	}
	n := 0
	for _, g := range interp.goroutines {
		if g.Status == GoroutineBlocked && !g.Panicked && !g.Aborted {
			n++
		}
	}
	interp.deadlock = &shadow.DeadlockError{GoroutineCount: n}
}

// probe runs body, an intercept's early run of a callback that native Go
// would run later or elsewhere (a sync.OnceFunc body at creation). A wait
// inside it can't be satisfied by goroutines that don't exist yet, so it
// unwinds to here instead of parking, and the goroutine carries on.
func (interp *Interpreter) probe(gid int64, body func()) {
	interp.probeDepth++
	body()
	interp.probeDepth--
	if g := interp.goroutine(gid); g != nil && g.Status == GoroutineBlocked && !g.Aborted {
		g.Status = GoroutineRunning
		g.wait = nil
	}
}

// errgroupState returns the state of the errgroup.Group that args[0] points
// to, or nil if giri can't identify it or isn't scheduling goroutines.
func (interp *Interpreter) errgroupState(args []Value) *mutexState {
	if len(args) == 0 || interp.sched == nil {
		return nil
	}
	var key shadow.AllocID
	if ptr, ok := args[0].Raw.(*shadow.Pointer); ok {
		key = ptr.Alloc
	} else if args[0].Provenance != nil {
		key = args[0].Provenance.Alloc
	} else {
		return nil
	}
	if interp.egroups == nil {
		interp.egroups = make(map[shadow.AllocID]*mutexState)
	}
	eg := interp.egroups[key]
	if eg == nil {
		eg = &mutexState{}
		interp.egroups[key] = eg
	}
	return eg
}

// funcTask returns a task calling the function value f with no arguments.
func funcTask(f Value) (goroutineTask, bool) {
	switch fn := f.Raw.(type) {
	case *ssa.Function:
		if fn.Blocks != nil {
			return goroutineTask{fn: fn}, true
		}
	case *ClosureValue:
		return goroutineTask{fn: fn.Fn, args: append([]Value(nil), fn.FreeVars...)}, true
	}
	return goroutineTask{}, false
}
