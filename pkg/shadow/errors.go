package shadow

import "fmt"

// UseAfterFreeError is reported when accessing memory after it was freed.
type UseAfterFreeError struct {
	AllocID    AllocID
	AllocSite  string
	FreeSite   string
	AccessSite string
	ArenaID    ArenaID // Non-zero if freed via arena
	TypeName   string
}

func (e *UseAfterFreeError) Error() string {
	if e.ArenaID != 0 {
		return fmt.Sprintf(
			"use-after-free: access to %s (alloc %d) at %s\n"+
				"  allocated at: %s\n"+
				"  freed at:     %s (arena %d freed)",
			e.TypeName, e.AllocID, e.AccessSite,
			e.AllocSite,
			e.FreeSite, e.ArenaID,
		)
	}
	return fmt.Sprintf(
		"use-after-free: access to %s (alloc %d) at %s\n"+
			"  allocated at: %s\n"+
			"  freed at:     %s",
		e.TypeName, e.AllocID, e.AccessSite,
		e.AllocSite,
		e.FreeSite,
	)
}

// DoubleFreeError is reported when freeing already-freed memory.
type DoubleFreeError struct {
	AllocID    AllocID
	FirstFree  string
	SecondFree string
	AllocSite  string
}

func (e *DoubleFreeError) Error() string {
	return fmt.Sprintf(
		"double-free: allocation %d freed again at %s\n"+
			"  allocated at:  %s\n"+
			"  first free at: %s",
		e.AllocID, e.SecondFree,
		e.AllocSite,
		e.FirstFree,
	)
}

// ArenaDoubleFreeError is reported when freeing an already-freed arena.
type ArenaDoubleFreeError struct {
	ArenaID    ArenaID
	FirstFree  string
	SecondFree string
	CreateSite string
}

func (e *ArenaDoubleFreeError) Error() string {
	return fmt.Sprintf(
		"arena double-free: arena %d freed again at %s\n"+
			"  created at:    %s\n"+
			"  first free at: %s",
		e.ArenaID, e.SecondFree,
		e.CreateSite,
		e.FirstFree,
	)
}

// OutOfBoundsError is reported when accessing memory outside allocation bounds.
type OutOfBoundsError struct {
	AllocID    AllocID
	AllocSize  int
	Offset     int
	AccessSize int
	Site       string
	TypeName   string
}

func (e *OutOfBoundsError) Error() string {
	return fmt.Sprintf(
		"out-of-bounds: access to %s (alloc %d) at offset %d+%d, but allocation is only %d bytes\n"+
			"  at: %s",
		e.TypeName, e.AllocID, e.Offset, e.AccessSize, e.AllocSize,
		e.Site,
	)
}

// UninitializedReadError is reported when reading uninitialized memory.
type UninitializedReadError struct {
	AllocID  AllocID
	Offset   int
	Site     string
	TypeName string
}

func (e *UninitializedReadError) Error() string {
	return fmt.Sprintf(
		"uninitialized read: reading %s (alloc %d) at byte offset %d before initialization\n"+
			"  at: %s",
		e.TypeName, e.AllocID, e.Offset,
		e.Site,
	)
}

// InvalidAccessError is a catch-all for other invalid memory accesses.
type InvalidAccessError struct {
	Kind    string
	Site    string
	Details string
}

func (e *InvalidAccessError) Error() string {
	return fmt.Sprintf("invalid access (%s) at %s: %s", e.Kind, e.Site, e.Details)
}

// UnsafePointerViolation is reported when unsafe.Pointer rules are broken.
type UnsafePointerViolation struct {
	Rule    UnsafeRule
	Site    string
	Details string
}

// UnsafeRule identifies which of Go's six unsafe.Pointer rules was violated.
type UnsafeRule int

const (
	// RuleConversion: *T1 → unsafe.Pointer → *T2 (must respect alignment)
	RuleConversion UnsafeRule = iota + 1
	// RuleUintptr: unsafe.Pointer → uintptr (must convert back before GC can move)
	RuleUintptr
	// RuleArithmetic: unsafe.Pointer → uintptr → arithmetic → unsafe.Pointer
	// (must stay within original allocation)
	RuleArithmetic
	// RuleSyscall: uintptr in syscall args (special case, allowed)
	RuleSyscall
	// RuleReflect: reflect.Value.Pointer/UnsafeAddr → unsafe.Pointer
	RuleReflect
	// RuleSliceHeader: reflect.SliceHeader/StringHeader manipulation
	RuleSliceHeader
)

func (r UnsafeRule) String() string {
	switch r {
	case RuleConversion:
		return "rule 1: pointer conversion alignment"
	case RuleUintptr:
		return "rule 2: uintptr must convert back immediately"
	case RuleArithmetic:
		return "rule 3: pointer arithmetic must stay within allocation"
	case RuleSyscall:
		return "rule 4: syscall uintptr argument"
	case RuleReflect:
		return "rule 5: reflect pointer conversion"
	case RuleSliceHeader:
		return "rule 6: SliceHeader/StringHeader manipulation"
	default:
		return "unknown rule"
	}
}

func (e *UnsafePointerViolation) Error() string {
	return fmt.Sprintf(
		"unsafe.Pointer violation (%s) at %s\n  %s",
		e.Rule, e.Site, e.Details,
	)
}

// EscapedPointerError is reported when an arena pointer outlives its arena.
type EscapedPointerError struct {
	AllocID    AllocID
	ArenaID    ArenaID
	AllocSite  string
	EscapeSite string
	EscapeKind string // "return", "global", "channel", "closure"
}

func (e *EscapedPointerError) Error() string {
	return fmt.Sprintf(
		"arena pointer escape: %s (alloc %d, arena %d) escapes via %s at %s\n"+
			"  allocated at: %s\n"+
			"  hint: use Clone() to copy to heap before the arena is freed",
		e.EscapeKind, e.AllocID, e.ArenaID, e.EscapeKind, e.EscapeSite,
		e.AllocSite,
	)
}

// NilPointerDerefError is reported when a nil pointer is dereferenced.
type NilPointerDerefError struct {
	Site string
	GID  int64
}

func (e *NilPointerDerefError) Error() string {
	return fmt.Sprintf(
		"nil pointer dereference (goroutine %d) at %s",
		e.GID, e.Site,
	)
}

// DataRaceError is reported when concurrent unsynchronized access is detected.
type DataRaceError struct {
	AllocID         AllocID
	Offset          int
	Write1Site      string
	Write1Goroutine int64
	Write2Site      string
	Write2Goroutine int64
	TypeName        string
}

func (e *DataRaceError) Error() string {
	return fmt.Sprintf(
		"data race on %s (alloc %d) at offset %d\n"+
			"  goroutine %d at: %s\n"+
			"  goroutine %d at: %s",
		e.TypeName, e.AllocID, e.Offset,
		e.Write1Goroutine, e.Write1Site,
		e.Write2Goroutine, e.Write2Site,
	)
}

// TypeAssertionError is reported when a non-comma-ok type assertion fails at runtime.
// In a real Go program this would panic; Giri records it as a violation.
type TypeAssertionError struct {
	Site         string
	ConcreteType string // the actual dynamic type held by the interface
	AssertedType string // the type being asserted to
	GID          int64
}

func (e *TypeAssertionError) Error() string {
	return fmt.Sprintf(
		"type-assertion failed: interface holds %s, not %s (goroutine %d) at %s",
		e.ConcreteType, e.AssertedType, e.GID, e.Site,
	)
}
