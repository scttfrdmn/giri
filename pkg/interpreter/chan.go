package interpreter

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// Channels with values (#295), following the runtime's chan.go: a buffered
// channel is a FIFO of at most capacity items; an unbuffered channel is a
// rendezvous between a sender and a receiver, with waiting operations queued
// as sudogs. Only one goroutine runs at a time (gosched.go), so every
// operation here is atomic between waits.
//
// Happens-before edges, per the Go memory model:
//   - a send happens before the corresponding receive completes;
//   - a receive from an unbuffered channel happens before the send completes;
//   - the k-th receive from a channel of capacity C happens before the
//     (k+C)-th send completes;
//   - close happens before a receive that returns because the channel is
//     closed.

// ChanID is a unique identifier for an interpreted channel.
type ChanID uint64

// chanEntry is an interpreted channel.
type chanEntry struct {
	capacity int // 0 = unbuffered
	closed   bool
	buf      []chanItem // buffered values, oldest first

	// recvq and sendq hold waiting operations on an unbuffered channel.
	// Entries from a select that has already completed are stale and are
	// skipped.
	recvq, sendq []*sudog

	// For the k+C rule: receive clocks not yet merged by a send, and the
	// number of sends so far.
	recvClocks [][]clockEntry
	nsent      int

	closeClock map[int64]uint64

	// refill makes the channel always hold one value that receiving never
	// consumes: a stand-in for timer and signal channels until virtual time
	// (#291). The value is refillVal.
	refill    bool
	refillVal Value
}

// clockEntry is one component of a vector-clock snapshot.
type clockEntry struct {
	gid int64
	t   uint64
}

// chanItem is a buffered value with its sender's clock.
type chanItem struct {
	val   Value
	clock map[int64]uint64
}

// sudog is a goroutine waiting in a channel operation.
type sudog struct {
	gid   int64
	val   Value            // send: the value; receive: filled in by the sender
	clock map[int64]uint64 // the waiter's clock when it started waiting
	peer  map[int64]uint64 // the counterpart's clock, set when done
	done  bool             // a counterpart completed the operation

	sel     *selectWait // the select this case belongs to, or nil
	caseIdx int
}

// live reports whether s is still waiting.
func (s *sudog) live() bool { return !s.done && (s.sel == nil || !s.sel.fired) }

// selectWait is a select statement waiting on several channels.
type selectWait struct {
	fired bool   // completed (by a counterpart) or withdrawn
	sg    *sudog // the case a counterpart completed; nil if withdrawn
}

// createChannel allocates a new channel and returns its ChanID.
func (interp *Interpreter) createChannel(capacity int) ChanID {
	id := ChanID(interp.nextChanID.Add(1))
	interp.channels[id] = &chanEntry{capacity: capacity}
	return id
}

// newRefillChannel returns a channel that always holds val.
func (interp *Interpreter) newRefillChannel(val Value) ChanID {
	id := interp.createChannel(1)
	interp.channels[id].refill, interp.channels[id].refillVal = true, val
	return id
}

// popLive removes and returns the first live sudog in q, dropping stale
// ones, or nil.
func popLive(q *[]*sudog, notSel *selectWait) *sudog {
	for i, s := range *q {
		if s.live() && (s.sel == nil || s.sel != notSel) {
			*q = append((*q)[:0:0], (*q)[i+1:]...)
			return s
		}
	}
	return nil
}

// hasLive reports whether q holds a live sudog not belonging to notSel.
func hasLive(q []*sudog, notSel *selectWait) bool {
	for _, s := range q {
		if s.live() && (s.sel == nil || s.sel != notSel) {
			return true
		}
	}
	return false
}

// claim completes s on behalf of a counterpart.
func (s *sudog) claim(peer map[int64]uint64) {
	s.done = true
	s.peer = peer
	if s.sel != nil {
		s.sel.fired, s.sel.sg = true, s
	}
}

// canRecv reports whether a receive from ch would complete now (self, if
// not nil, is the receiving select, whose own send cases don't count).
func (ch *chanEntry) canRecv(self *selectWait) bool {
	if ch.refill || len(ch.buf) > 0 || ch.closed {
		return true
	}
	return ch.capacity == 0 && hasLive(ch.sendq, self)
}

// canSend reports whether a send on ch would proceed now: complete, or
// panic because ch is closed.
func (ch *chanEntry) canSend(self *selectWait) bool {
	if ch.closed {
		return true
	}
	if ch.capacity > 0 {
		return len(ch.buf) < ch.capacity && !ch.refill
	}
	return hasLive(ch.recvq, self)
}

// releaseClock snapshots goroutine gid's clock, then ticks it, when races
// are tracked: the clock a release (send, close) publishes. Ticking after
// the snapshot keeps the releaser's later accesses out of what an acquirer
// merges (#286).
func (interp *Interpreter) releaseClock(gid int64) map[int64]uint64 {
	if !interp.config.TrackRaces {
		return nil
	}
	g := interp.goroutine(gid)
	if g == nil {
		return nil
	}
	snap := snapshotClock(g)
	g.VClock.Tick(gid)
	return snap
}

// acquire merges clock into goroutine gid's clock.
func (interp *Interpreter) acquire(gid int64, clock map[int64]uint64) {
	if clock == nil {
		return
	}
	if g := interp.goroutine(gid); g != nil {
		mergeClock(g, clock)
	}
}

// chanZero is the value a receive from a closed, empty channel yields.
func (interp *Interpreter) chanZero(t types.Type) Value {
	if t == nil {
		return Value{}
	}
	v := concreteZero(t)
	if v.Raw == zeroAggregate && !interp.concrete {
		return Value{}
	}
	return v
}

// chanSend sends val on channel id from goroutine gid, waiting while it
// can't proceed. It returns false if the goroutine was aborted or the send
// panicked.
func (interp *Interpreter) chanSend(gid int64, id ChanID, val Value, site string) bool {
	ch, ok := interp.channels[id]
	if !ok {
		return true // unknown channel: as before, the send has no effect
	}
	interp.checkChannelEscape(gid, val, site)

	if ch.capacity > 0 || ch.closed {
		w := waitInfo{site: site, kind: "channel send", chanID: id, onSend: true, ready: func() bool { return ch.canSend(nil) }}
		if !interp.block(gid, w) {
			return false
		}
		if ch.closed {
			interp.sendOnClosed(gid, site)
			return false
		}
		interp.bufferedSend(gid, ch, val)
		return true
	}

	// Unbuffered: hand the value to a waiting receiver, or wait for one.
	if r := popLive(&ch.recvq, nil); r != nil {
		interp.acquire(gid, r.clock) // the receive happens before this send completes
		r.val = val
		r.claim(interp.releaseClock(gid))
		return true
	}
	s := &sudog{gid: gid, val: val, clock: interp.releaseClock(gid)}
	ch.sendq = append(ch.sendq, s)
	w := waitInfo{site: site, kind: "channel send", chanID: id, onSend: true, ready: func() bool { return s.done || ch.closed }}
	if !interp.block(gid, w) {
		s.done = true // withdraw the offer
		return false
	}
	if !s.done {
		interp.sendOnClosed(gid, site)
		return false
	}
	interp.acquire(gid, s.peer)
	return true
}

// bufferedSend appends val to ch's buffer; there is room.
func (interp *Interpreter) bufferedSend(gid int64, ch *chanEntry, val Value) {
	if ch.nsent >= ch.capacity && len(ch.recvClocks) > 0 {
		// The (nsent-C)-th receive freed this slot.
		if g := interp.goroutine(gid); g != nil {
			for _, e := range ch.recvClocks[0] {
				if e.t > g.VClock.Clocks[e.gid] {
					g.VClock.Clocks[e.gid] = e.t
				}
			}
		}
		ch.recvClocks = ch.recvClocks[1:]
	}
	ch.nsent++
	ch.buf = append(ch.buf, chanItem{val: val, clock: interp.releaseClock(gid)})
}

func (interp *Interpreter) sendOnClosed(gid int64, site string) {
	interp.runtimePanic(gid, "send on closed channel")
	interp.recordViolation(gid, fmt.Errorf("send on closed channel at %s (goroutine %d)", site, gid))
}

// checkChannelEscape reports an arena pointer sent on a channel.
func (interp *Interpreter) checkChannelEscape(gid int64, val Value, site string) {
	if !interp.config.TrackArenas || val.Provenance == nil {
		return
	}
	alloc, ok := interp.Memory.GetAllocation(val.Provenance.Alloc)
	if ok && alloc.Kind == shadow.AllocArena {
		interp.recordViolation(gid, &shadow.EscapedPointerError{
			AllocID:    alloc.ID,
			ArenaID:    alloc.Arena,
			AllocSite:  alloc.AllocSite,
			EscapeSite: site,
			EscapeKind: "channel",
		})
	}
}

// chanRecv receives from channel id on goroutine gid, waiting until a value
// is available or the channel is closed. elem is the element type, for the
// zero value of a closed channel. completed is false if the goroutine was
// aborted.
func (interp *Interpreter) chanRecv(gid int64, id ChanID, elem types.Type, site string) (val Value, ok, completed bool) {
	ch, exists := interp.channels[id]
	if !exists {
		return Value{}, true, true // unknown channel: as before
	}
	if ch.capacity == 0 && !ch.closed {
		if s := popLive(&ch.sendq, nil); s != nil {
			v := interp.recvFrom(gid, s)
			return v, true, true
		}
		r := &sudog{gid: gid, clock: interp.releaseClock(gid)}
		ch.recvq = append(ch.recvq, r)
		w := waitInfo{site: site, kind: "channel receive", chanID: id, ready: func() bool { return r.done || ch.closed }}
		if !interp.block(gid, w) {
			r.done = true // withdraw the offer
			return Value{}, false, false
		}
		if r.done {
			interp.acquire(gid, r.peer) // the send happens before this receive completes
			interp.tick(gid)
			return r.val, true, true
		}
		return interp.recvClosed(gid, ch, elem), false, true
	}

	w := waitInfo{site: site, kind: "channel receive", chanID: id, ready: func() bool { return ch.canRecv(nil) }}
	if !interp.block(gid, w) {
		return Value{}, false, false
	}
	return interp.recvReady(gid, ch, elem)
}

// recvReady completes a receive from ch, which can proceed without waiting.
func (interp *Interpreter) recvReady(gid int64, ch *chanEntry, elem types.Type) (Value, bool, bool) {
	switch {
	case ch.refill:
		return ch.refillVal, true, true
	case len(ch.buf) > 0:
		it := ch.buf[0]
		ch.buf = ch.buf[1:]
		interp.acquire(gid, it.clock)
		if ch.capacity > 0 && interp.config.TrackRaces {
			// Publish this receive for the (k+C)-th send, then tick.
			if g := interp.goroutine(gid); g != nil {
				snap := make([]clockEntry, 0, len(g.VClock.Clocks))
				for k, t := range g.VClock.Clocks {
					snap = append(snap, clockEntry{k, t})
				}
				ch.recvClocks = append(ch.recvClocks, snap)
			}
		}
		interp.tick(gid)
		return it.val, true, true
	case ch.capacity == 0 && !ch.closed:
		if s := popLive(&ch.sendq, nil); s != nil {
			return interp.recvFrom(gid, s), true, true
		}
	}
	return interp.recvClosed(gid, ch, elem), false, true
}

// recvFrom takes the value of waiting sender s.
func (interp *Interpreter) recvFrom(gid int64, s *sudog) Value {
	interp.acquire(gid, s.clock) // the send happens before this receive completes
	s.claim(interp.releaseClock(gid))
	return s.val
}

// recvClosed is a receive that returns because ch is closed and empty.
func (interp *Interpreter) recvClosed(gid int64, ch *chanEntry, elem types.Type) Value {
	interp.acquire(gid, ch.closeClock)
	return interp.chanZero(elem)
}

// tick advances gid's own clock component, when races are tracked.
func (interp *Interpreter) tick(gid int64) {
	if !interp.config.TrackRaces {
		return
	}
	if g := interp.goroutine(gid); g != nil {
		g.VClock.Tick(gid)
	}
}

// handleChannelClose closes a channel (#31). Waiting receivers then return
// the zero value and waiting senders panic.
func (interp *Interpreter) handleChannelClose(gid int64, chanID ChanID, site string) {
	ch, ok := interp.channels[chanID]
	if !ok {
		return
	}
	if ch.closed {
		interp.runtimePanic(gid, "close of closed channel")
		interp.recordViolation(gid, &shadow.DoubleCloseError{Site: site, GID: gid})
		return
	}
	ch.closed = true
	ch.closeClock = interp.releaseClock(gid)
}

// --- select ---

// selectCase is one case of a select statement, resolved once.
type selectCase struct {
	id   ChanID
	ch   *chanEntry // nil: never ready (a nil or unknown channel)
	send bool
	val  Value
	elem types.Type // receive: element type
}

func (c *selectCase) ready(self *selectWait) bool {
	switch {
	case c.ch == nil:
		return false
	case c.send:
		return c.ch.canSend(self)
	default:
		return c.ch.canRecv(self)
	}
}

// execSelect runs a select statement (#30): the first ready case in source
// order; otherwise the default case if there is one; otherwise it waits
// until a case can proceed, offering every case to counterparts meanwhile
// (#294, #295).
func (interp *Interpreter) execSelect(gid int64, frame *Frame, inst *ssa.Select, site string) {
	tuple, _ := inst.Type().(*types.Tuple)
	cases := make([]selectCase, len(inst.States))
	for i, state := range inst.States {
		c := &cases[i]
		c.send = state.Dir == types.SendOnly
		if id, ok := interp.resolveValue(frame, state.Chan).Raw.(ChanID); ok {
			c.id, c.ch = id, interp.channels[id]
		}
		if c.send {
			c.val = interp.resolveValue(frame, state.Send)
		} else if ct, ok := state.Chan.Type().Underlying().(*types.Chan); ok {
			c.elem = ct.Elem()
		}
	}
	first := func(self *selectWait) int {
		for i := range cases {
			if cases[i].ready(self) {
				return i
			}
		}
		return -1
	}
	result := func(idx int, recvOk bool, recvVal Value) {
		vals := []Value{{Raw: int64(idx)}, {Raw: recvOk}}
		k := 0
		for i := range cases {
			if cases[i].send {
				continue
			}
			var v Value
			if i == idx {
				v = recvVal
			} else if tuple != nil && 2+k < tuple.Len() {
				v = interp.chanZero(tuple.At(2 + k).Type())
			}
			vals = append(vals, v)
			k++
		}
		interp.setLocal(frame, inst, Value{Raw: vals})
	}

	chosen := first(nil)
	if chosen < 0 && !inst.Blocking {
		result(len(inst.States), false, Value{})
		return
	}
	if chosen < 0 {
		// Offer each case on an unbuffered channel to counterparts, then wait
		// until one is taken or some case becomes ready.
		sw := &selectWait{}
		for i := range cases {
			c := &cases[i]
			if c.ch == nil || c.ch.capacity > 0 {
				continue
			}
			s := &sudog{gid: gid, val: c.val, clock: interp.releaseClock(gid), sel: sw, caseIdx: i}
			if c.send {
				c.ch.sendq = append(c.ch.sendq, s)
			} else {
				c.ch.recvq = append(c.ch.recvq, s)
			}
		}
		w := waitInfo{site: site, kind: "select", ready: func() bool { return sw.fired || first(sw) >= 0 }}
		if len(cases) == 0 {
			w.ready = nil // select {}: waits forever
		}
		if !interp.block(gid, w) {
			sw.fired = true // withdraw the offers
			return
		}
		if sw.fired {
			// A counterpart completed one of the offered cases.
			s := sw.sg
			interp.acquire(gid, s.peer)
			interp.tick(gid)
			if cases[s.caseIdx].send {
				result(s.caseIdx, false, Value{})
			} else {
				result(s.caseIdx, true, s.val)
			}
			return
		}
		sw.fired = true // withdraw the offers
		chosen = first(nil)
	}

	c := &cases[chosen]
	if c.send {
		interp.chanSend(gid, c.id, c.val, site)
		result(chosen, false, Value{})
		return
	}
	v, ok, _ := interp.recvReady(gid, c.ch, c.elem)
	result(chosen, ok, v)
}
