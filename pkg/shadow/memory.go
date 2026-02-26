// Package shadow implements shadow memory tracking for the Giri interpreter.
//
// Shadow memory maintains metadata about every allocation in the program:
// provenance (where it came from), bounds (how big it is), initialization
// state (which bytes have been written), and liveness (has it been freed).
//
// This is Giri's equivalent of Miri's allocation tracking. Every pointer
// in the interpreted program maps to an AllocID, and every AllocID maps
// to an Allocation with full metadata.
package shadow

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// AllocID uniquely identifies an allocation across the program lifetime.
type AllocID uint64

// AllocKind describes the origin of an allocation.
type AllocKind uint8

const (
	AllocHeap   AllocKind = iota // Standard GC-managed allocation (new, make)
	AllocStack                   // Stack-local variable
	AllocArena                   // arena.Arena allocation
	AllocGlobal                  // Package-level global variable
	AllocUnsafe                  // Created via unsafe.Pointer arithmetic
)

func (k AllocKind) String() string {
	switch k {
	case AllocHeap:
		return "heap"
	case AllocStack:
		return "stack"
	case AllocArena:
		return "arena"
	case AllocGlobal:
		return "global"
	case AllocUnsafe:
		return "unsafe"
	default:
		return "unknown"
	}
}

// ArenaID identifies a specific arena instance.
type ArenaID uint64

// Allocation represents a single memory allocation with full metadata.
type Allocation struct {
	ID   AllocID
	Kind AllocKind
	Size int // Size in bytes

	// Provenance
	Arena     ArenaID // Non-zero if arena-allocated
	AllocSite string  // "file:line" where allocation occurred
	TypeName  string  // Go type name (e.g., "*MyStruct", "[]byte")

	// Lifecycle
	Freed    bool   // Has this allocation been freed?
	FreeSite string // Where it was freed (for error reporting)

	// Initialization tracking (bitset: 1 = initialized)
	// nil means "fully initialized" (optimization for most allocations)
	InitBits []uint64

	// Access log for debugging (only in verbose mode)
	AccessLog []AccessRecord
}

// AccessRecord tracks a single read or write to an allocation.
type AccessRecord struct {
	Kind      AccessKind
	Offset    int
	Size      int
	Site      string // "file:line"
	Goroutine int64
}

// AccessKind distinguishes reads from writes.
type AccessKind uint8

const (
	AccessRead AccessKind = iota
	AccessWrite
)

// Pointer represents a tracked pointer in the interpreted program.
// Every pointer carries provenance — it knows which allocation it derives from
// and what offset within that allocation it points to.
type Pointer struct {
	Alloc  AllocID // Which allocation this pointer derives from
	Offset int     // Byte offset within the allocation
	// Provenance chain: if this pointer was derived from another via
	// unsafe arithmetic, we track the derivation.
	DerivedFrom *Pointer
}

// ArenaState tracks the lifecycle of a single arena instance.
type ArenaState struct {
	ID          ArenaID
	Freed       bool
	CreateSite  string
	FreeSite    string
	Allocations []AllocID // All allocations made in this arena
}

// Memory is the shadow memory system. It maps every allocation in the
// interpreted program to its metadata, and every pointer to its provenance.
//
// This is the central data structure of Giri. All detection modules
// (use-after-free, bounds checking, unsafe rules, etc.) query Memory
// to make their decisions.
type Memory struct {
	mu sync.RWMutex

	// Core allocation tracking
	allocations map[AllocID]*Allocation
	nextAlloc   atomic.Uint64

	// Arena lifecycle tracking
	arenas    map[ArenaID]*ArenaState
	nextArena atomic.Uint64

	// Pointer provenance: maps interpreter-level values to their provenance
	// In practice, keyed by the ssa.Value name or a synthetic ID
	pointers map[string]*Pointer

	// Configuration
	verbose   bool // Track access logs
	trackInit bool // Track initialization state per byte
}

// NewMemory creates a new shadow memory system.
func NewMemory(opts ...Option) *Memory {
	m := &Memory{
		allocations: make(map[AllocID]*Allocation),
		arenas:      make(map[ArenaID]*ArenaState),
		pointers:    make(map[string]*Pointer),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Option configures shadow memory behavior.
type Option func(*Memory)

// WithVerbose enables access logging for all allocations.
func WithVerbose() Option {
	return func(m *Memory) { m.verbose = true }
}

// WithInitTracking enables per-byte initialization tracking.
func WithInitTracking() Option {
	return func(m *Memory) { m.trackInit = true }
}

// --- Allocation Lifecycle ---

// Allocate registers a new allocation and returns its ID.
func (m *Memory) Allocate(kind AllocKind, size int, typeName, site string) AllocID {
	id := AllocID(m.nextAlloc.Add(1))

	alloc := &Allocation{
		ID:        id,
		Kind:      kind,
		Size:      size,
		TypeName:  typeName,
		AllocSite: site,
	}

	if m.trackInit {
		// One bit per byte, packed into uint64s
		words := (size + 63) / 64
		alloc.InitBits = make([]uint64, words)
	}

	m.mu.Lock()
	m.allocations[id] = alloc
	m.mu.Unlock()

	return id
}

// AllocateInArena registers an allocation within a specific arena.
func (m *Memory) AllocateInArena(arenaID ArenaID, size int, typeName, site string) AllocID {
	id := m.Allocate(AllocArena, size, typeName, site)

	m.mu.Lock()
	alloc := m.allocations[id]
	alloc.Arena = arenaID

	if arena, ok := m.arenas[arenaID]; ok {
		arena.Allocations = append(arena.Allocations, id)
	}
	m.mu.Unlock()

	return id
}

// Free marks an allocation as freed. Returns an error if already freed (double-free).
func (m *Memory) Free(id AllocID, site string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alloc, ok := m.allocations[id]
	if !ok {
		return fmt.Errorf("free of unknown allocation %d at %s", id, site)
	}
	if alloc.Freed {
		return &DoubleFreeError{
			AllocID:    id,
			FirstFree:  alloc.FreeSite,
			SecondFree: site,
			AllocSite:  alloc.AllocSite,
		}
	}

	alloc.Freed = true
	alloc.FreeSite = site
	return nil
}

// --- Arena Lifecycle ---

// CreateArena registers a new arena and returns its ID.
func (m *Memory) CreateArena(site string) ArenaID {
	id := ArenaID(m.nextArena.Add(1))

	m.mu.Lock()
	m.arenas[id] = &ArenaState{
		ID:         id,
		CreateSite: site,
	}
	m.mu.Unlock()

	return id
}

// FreeArena marks an arena and ALL its allocations as freed.
// Returns errors for any issues detected.
func (m *Memory) FreeArena(id ArenaID, site string) []error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arena, ok := m.arenas[id]
	if !ok {
		return []error{fmt.Errorf("free of unknown arena %d at %s", id, site)}
	}
	if arena.Freed {
		return []error{&ArenaDoubleFreeError{
			ArenaID:    id,
			FirstFree:  arena.FreeSite,
			SecondFree: site,
			CreateSite: arena.CreateSite,
		}}
	}

	arena.Freed = true
	arena.FreeSite = site

	// Poison all allocations in this arena
	var errs []error
	for _, allocID := range arena.Allocations {
		if alloc, ok := m.allocations[allocID]; ok {
			if !alloc.Freed {
				alloc.Freed = true
				alloc.FreeSite = site + " (via arena.Free)"
			}
		}
	}

	return errs
}

// --- Pointer Provenance ---

// TrackPointer associates a program value with pointer provenance.
func (m *Memory) TrackPointer(valueID string, ptr *Pointer) {
	m.mu.Lock()
	m.pointers[valueID] = ptr
	m.mu.Unlock()
}

// GetProvenance returns the provenance of a tracked pointer.
func (m *Memory) GetProvenance(valueID string) (*Pointer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ptr, ok := m.pointers[valueID]
	return ptr, ok
}

// DerivePointer creates a new pointer derived from an existing one
// (e.g., via pointer arithmetic or field access).
func (m *Memory) DerivePointer(base *Pointer, offsetDelta int) *Pointer {
	return &Pointer{
		Alloc:       base.Alloc,
		Offset:      base.Offset + offsetDelta,
		DerivedFrom: base,
	}
}

// --- Validation ---

// CheckAccess validates a memory access. Returns nil if safe, error if UB.
func (m *Memory) CheckAccess(ptr *Pointer, size int, kind AccessKind, site string, goroutine int64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alloc, ok := m.allocations[ptr.Alloc]
	if !ok {
		return &InvalidAccessError{
			Kind:    "dangling pointer",
			Site:    site,
			Details: fmt.Sprintf("pointer references unknown allocation %d", ptr.Alloc),
		}
	}

	// Check use-after-free
	if alloc.Freed {
		// Determine if this was an arena free
		if alloc.Arena != 0 {
			return &UseAfterFreeError{
				AllocID:    alloc.ID,
				AllocSite:  alloc.AllocSite,
				FreeSite:   alloc.FreeSite,
				AccessSite: site,
				ArenaID:    alloc.Arena,
				TypeName:   alloc.TypeName,
			}
		}
		return &UseAfterFreeError{
			AllocID:    alloc.ID,
			AllocSite:  alloc.AllocSite,
			FreeSite:   alloc.FreeSite,
			AccessSite: site,
			TypeName:   alloc.TypeName,
		}
	}

	// Bounds check
	if ptr.Offset < 0 || ptr.Offset+size > alloc.Size {
		return &OutOfBoundsError{
			AllocID:    alloc.ID,
			AllocSize:  alloc.Size,
			Offset:     ptr.Offset,
			AccessSize: size,
			Site:       site,
			TypeName:   alloc.TypeName,
		}
	}

	// Initialization check (reads only)
	if kind == AccessRead && m.trackInit && alloc.InitBits != nil {
		for i := 0; i < size; i++ {
			byteIdx := ptr.Offset + i
			word := byteIdx / 64
			bit := uint(byteIdx % 64)
			if word < len(alloc.InitBits) && alloc.InitBits[word]&(1<<bit) == 0 {
				return &UninitializedReadError{
					AllocID:  alloc.ID,
					Offset:   byteIdx,
					Site:     site,
					TypeName: alloc.TypeName,
				}
			}
		}
	}

	// Record access if verbose
	if m.verbose && alloc.AccessLog != nil {
		// Note: this path requires write lock; callers should upgrade
		// In practice, we'd use a lock-free append or per-goroutine buffer
	}

	return nil
}

// MarkInitialized marks a range of bytes as initialized after a write.
func (m *Memory) MarkInitialized(allocID AllocID, offset, size int) {
	if !m.trackInit {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	alloc, ok := m.allocations[allocID]
	if !ok || alloc.InitBits == nil {
		return
	}

	for i := 0; i < size; i++ {
		byteIdx := offset + i
		word := byteIdx / 64
		bit := uint(byteIdx % 64)
		if word < len(alloc.InitBits) {
			alloc.InitBits[word] |= 1 << bit
		}
	}
}

// --- Queries ---

// GetAllocation returns allocation metadata (read-only snapshot).
func (m *Memory) GetAllocation(id AllocID) (*Allocation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	alloc, ok := m.allocations[id]
	return alloc, ok
}

// GetArena returns arena state (read-only snapshot).
func (m *Memory) GetArena(id ArenaID) (*ArenaState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	arena, ok := m.arenas[id]
	return arena, ok
}

// LiveArenas returns all arenas that haven't been freed (leak detection).
func (m *Memory) LiveArenas() []*ArenaState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var live []*ArenaState
	for _, arena := range m.arenas {
		if !arena.Freed {
			live = append(live, arena)
		}
	}
	return live
}

// LiveAllocations returns all allocations that haven't been freed.
func (m *Memory) LiveAllocations() []*Allocation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var live []*Allocation
	for _, alloc := range m.allocations {
		if !alloc.Freed {
			live = append(live, alloc)
		}
	}
	return live
}

// Stats returns summary statistics about shadow memory state.
func (m *Memory) Stats() MemoryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := MemoryStats{
		TotalAllocations: len(m.allocations),
		TotalArenas:      len(m.arenas),
		TrackedPointers:  len(m.pointers),
	}
	for _, alloc := range m.allocations {
		if alloc.Freed {
			stats.FreedAllocations++
		} else {
			stats.LiveAllocations++
			stats.LiveBytes += alloc.Size
		}
	}
	for _, arena := range m.arenas {
		if !arena.Freed {
			stats.LiveArenas++
		}
	}
	return stats
}

// MemoryStats summarizes shadow memory state.
type MemoryStats struct {
	TotalAllocations int
	LiveAllocations  int
	FreedAllocations int
	LiveBytes        int
	TotalArenas      int
	LiveArenas       int
	TrackedPointers  int
}

func (s MemoryStats) String() string {
	return fmt.Sprintf("allocations: %d live / %d total (%d bytes), arenas: %d live / %d total, pointers: %d tracked",
		s.LiveAllocations, s.TotalAllocations, s.LiveBytes,
		s.LiveArenas, s.TotalArenas, s.TrackedPointers)
}
