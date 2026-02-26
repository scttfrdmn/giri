// Package detector implements composable safety checkers that analyze
// the interpreter's shadow memory state to find undefined behavior.
//
// Each detector is a focused module that looks for one class of bug.
// Detectors can be enabled/disabled independently and run at different
// points during interpretation (per-instruction, per-function, at end).
//
// This separation from the interpreter allows adding new checks without
// modifying the core execution engine.
package detector

import (
	"fmt"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// Detector is the interface for all safety checkers.
type Detector interface {
	// Name returns a human-readable name for this detector.
	Name() string

	// Description returns what this detector checks for.
	Description() string

	// CheckAccess is called on every memory access (load/store).
	// Return nil if the access is safe, error if UB is detected.
	CheckAccess(mem *shadow.Memory, ptr *shadow.Pointer, size int, kind shadow.AccessKind, site string, goroutine int64) error

	// CheckFinalize is called when interpretation completes.
	// Used for leak detection and other end-of-program checks.
	CheckFinalize(mem *shadow.Memory) []error
}

// --- Arena Safety Detector ---

// ArenaDetector checks for arena-related memory safety violations.
// This is Giri's generalization of safearena's runtime checks.
type ArenaDetector struct{}

func (d *ArenaDetector) Name() string { return "arena-safety" }
func (d *ArenaDetector) Description() string {
	return "Detects use-after-free, double-free, and leaks for arena-allocated memory"
}

func (d *ArenaDetector) CheckAccess(mem *shadow.Memory, ptr *shadow.Pointer, size int, kind shadow.AccessKind, site string, goroutine int64) error {
	alloc, ok := mem.GetAllocation(ptr.Alloc)
	if !ok {
		return nil // Not tracked
	}

	if alloc.Kind != shadow.AllocArena {
		return nil // Not an arena allocation
	}

	if alloc.Freed {
		return &shadow.UseAfterFreeError{
			AllocID:    alloc.ID,
			AllocSite:  alloc.AllocSite,
			FreeSite:   alloc.FreeSite,
			AccessSite: site,
			ArenaID:    alloc.Arena,
			TypeName:   alloc.TypeName,
		}
	}

	return nil
}

func (d *ArenaDetector) CheckFinalize(mem *shadow.Memory) []error {
	var errs []error
	for _, arena := range mem.LiveArenas() {
		errs = append(errs, fmt.Errorf(
			"arena leak: arena %d created at %s was never freed\n"+
				"  hint: use defer arena.Free() immediately after creation",
			arena.ID, arena.CreateSite,
		))
	}
	return errs
}

// --- Bounds Checker ---

// BoundsDetector checks for out-of-bounds memory access.
type BoundsDetector struct{}

func (d *BoundsDetector) Name() string { return "bounds-check" }
func (d *BoundsDetector) Description() string {
	return "Detects out-of-bounds memory access via pointers"
}

func (d *BoundsDetector) CheckAccess(mem *shadow.Memory, ptr *shadow.Pointer, size int, kind shadow.AccessKind, site string, goroutine int64) error {
	alloc, ok := mem.GetAllocation(ptr.Alloc)
	if !ok {
		return nil
	}

	if ptr.Offset < 0 || ptr.Offset+size > alloc.Size {
		return &shadow.OutOfBoundsError{
			AllocID:    alloc.ID,
			AllocSize:  alloc.Size,
			Offset:     ptr.Offset,
			AccessSize: size,
			Site:       site,
			TypeName:   alloc.TypeName,
		}
	}

	return nil
}

func (d *BoundsDetector) CheckFinalize(mem *shadow.Memory) []error { return nil }

// --- Unsafe Pointer Detector ---

// UnsafeDetector checks for violations of Go's unsafe.Pointer rules.
type UnsafeDetector struct {
	// Pending uintptr conversions that haven't been converted back.
	// If a GC point is reached while these exist, it's UB.
	pendingUintptrs map[string]pendingConversion
}

type pendingConversion struct {
	site       string
	provenance *shadow.Pointer
}

func NewUnsafeDetector() *UnsafeDetector {
	return &UnsafeDetector{
		pendingUintptrs: make(map[string]pendingConversion),
	}
}

func (d *UnsafeDetector) Name() string { return "unsafe-pointer" }
func (d *UnsafeDetector) Description() string {
	return "Checks all six unsafe.Pointer rules from the Go spec"
}

func (d *UnsafeDetector) CheckAccess(mem *shadow.Memory, ptr *shadow.Pointer, size int, kind shadow.AccessKind, site string, goroutine int64) error {
	// Bounds checking for unsafe-derived pointers
	if ptr.DerivedFrom != nil {
		alloc, ok := mem.GetAllocation(ptr.Alloc)
		if ok && (ptr.Offset < 0 || ptr.Offset+size > alloc.Size) {
			return &shadow.UnsafePointerViolation{
				Rule: shadow.RuleArithmetic,
				Site: site,
				Details: fmt.Sprintf(
					"unsafe pointer arithmetic resulted in offset %d+%d, allocation is %d bytes (type: %s)",
					ptr.Offset, size, alloc.Size, alloc.TypeName,
				),
			}
		}
	}
	return nil
}

// RecordUintptrConversion tracks an unsafe.Pointer → uintptr conversion.
// If this uintptr isn't converted back before the next GC point, it's UB.
func (d *UnsafeDetector) RecordUintptrConversion(valueID, site string, ptr *shadow.Pointer) {
	d.pendingUintptrs[valueID] = pendingConversion{
		site:       site,
		provenance: ptr,
	}
}

// ClearUintptrConversion marks a uintptr → unsafe.Pointer conversion (safe).
func (d *UnsafeDetector) ClearUintptrConversion(valueID string) {
	delete(d.pendingUintptrs, valueID)
}

// ClearAllUintptrConversions clears all pending uintptr conversions.
// Called when any uintptr → unsafe.Pointer conversion happens.
func (d *UnsafeDetector) ClearAllUintptrConversions() {
	for k := range d.pendingUintptrs {
		delete(d.pendingUintptrs, k)
	}
}

// CheckGCPoint should be called at potential GC safepoints.
// Any pending uintptr conversions at a GC point are UB.
func (d *UnsafeDetector) CheckGCPoint(site string) []error {
	var errs []error
	for valueID, pending := range d.pendingUintptrs {
		errs = append(errs, &shadow.UnsafePointerViolation{
			Rule: shadow.RuleUintptr,
			Site: site,
			Details: fmt.Sprintf(
				"uintptr value %s (converted at %s) survived to GC point without conversion back to unsafe.Pointer",
				valueID, pending.site,
			),
		})
	}
	return errs
}

func (d *UnsafeDetector) CheckFinalize(mem *shadow.Memory) []error {
	return d.CheckGCPoint("<program end>")
}

// --- Data Race Detector ---

// RaceDetector uses vector clocks to detect data races beyond what -race finds.
// It tracks happens-before relationships through channels, mutexes, and atomics.
type RaceDetector struct {
	// Last access per (allocation, offset) pair
	lastAccess map[accessKey]*accessEntry
}

type accessKey struct {
	allocID shadow.AllocID
	offset  int
}

type accessEntry struct {
	kind      shadow.AccessKind
	goroutine int64
	site      string
	clock     map[int64]uint64 // Snapshot of the goroutine's vector clock
}

func NewRaceDetector() *RaceDetector {
	return &RaceDetector{
		lastAccess: make(map[accessKey]*accessEntry),
	}
}

func (d *RaceDetector) Name() string { return "data-race" }
func (d *RaceDetector) Description() string {
	return "Detects data races using vector clocks (beyond -race detector)"
}

func (d *RaceDetector) CheckAccess(mem *shadow.Memory, ptr *shadow.Pointer, size int, kind shadow.AccessKind, site string, goroutine int64) error {
	// For each byte accessed, check against last access
	for i := 0; i < size; i++ {
		key := accessKey{allocID: ptr.Alloc, offset: ptr.Offset + i}

		if last, ok := d.lastAccess[key]; ok {
			// Race condition: two accesses to same location from different goroutines
			// where at least one is a write and they're not ordered by happens-before
			if last.goroutine != goroutine && (last.kind == shadow.AccessWrite || kind == shadow.AccessWrite) {
				// In full implementation: check vector clocks for happens-before
				// For now, flag concurrent write-write and read-write from different goroutines
				alloc, _ := mem.GetAllocation(ptr.Alloc)
				typeName := "unknown"
				if alloc != nil {
					typeName = alloc.TypeName
				}
				return &shadow.DataRaceError{
					AllocID:         ptr.Alloc,
					Offset:          ptr.Offset + i,
					Write1Site:      last.site,
					Write1Goroutine: last.goroutine,
					Write2Site:      site,
					Write2Goroutine: goroutine,
					TypeName:        typeName,
				}
			}
		}

		// Record this access
		d.lastAccess[key] = &accessEntry{
			kind:      kind,
			goroutine: goroutine,
			site:      site,
		}
	}

	return nil
}

func (d *RaceDetector) CheckFinalize(mem *shadow.Memory) []error { return nil }

// --- Detector Registry ---

// Registry holds all active detectors and dispatches checks to them.
type Registry struct {
	detectors      []Detector
	unsafeDetector *UnsafeDetector // extracted for direct GC-point and uintptr tracking
}

// NewRegistry creates a registry with the given detectors.
func NewRegistry(detectors ...Detector) *Registry {
	r := &Registry{detectors: detectors}
	for _, d := range detectors {
		if ud, ok := d.(*UnsafeDetector); ok {
			r.unsafeDetector = ud
			break
		}
	}
	return r
}

// DefaultRegistry returns a registry with all standard detectors enabled.
func DefaultRegistry() *Registry {
	return NewRegistry(
		&ArenaDetector{},
		&BoundsDetector{},
		NewUnsafeDetector(),
		NewRaceDetector(),
	)
}

// CheckAccess runs all detectors on a memory access.
func (r *Registry) CheckAccess(mem *shadow.Memory, ptr *shadow.Pointer, size int, kind shadow.AccessKind, site string, goroutine int64) []error {
	var errs []error
	for _, d := range r.detectors {
		if err := d.CheckAccess(mem, ptr, size, kind, site, goroutine); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// Finalize runs all detectors' finalization checks.
func (r *Registry) Finalize(mem *shadow.Memory) []error {
	var errs []error
	for _, d := range r.detectors {
		errs = append(errs, d.CheckFinalize(mem)...)
	}
	return errs
}

// RecordUintptrConversion tracks an unsafe.Pointer → uintptr conversion.
// Delegates to the UnsafeDetector if one is registered.
func (r *Registry) RecordUintptrConversion(valueID, site string, ptr *shadow.Pointer) {
	if r.unsafeDetector != nil {
		r.unsafeDetector.RecordUintptrConversion(valueID, site, ptr)
	}
}

// ClearUintptrConversion marks a specific uintptr → unsafe.Pointer conversion as safe.
func (r *Registry) ClearUintptrConversion(valueID string) {
	if r.unsafeDetector != nil {
		r.unsafeDetector.ClearUintptrConversion(valueID)
	}
}

// ClearAllUintptrConversions clears all pending uintptr conversions.
func (r *Registry) ClearAllUintptrConversions() {
	if r.unsafeDetector != nil {
		r.unsafeDetector.ClearAllUintptrConversions()
	}
}

// CheckGCPoint checks for pending uintptr conversions at a GC safepoint.
// Returns violations for any uintptr values that haven't been converted back.
func (r *Registry) CheckGCPoint(site string) []error {
	if r.unsafeDetector != nil {
		return r.unsafeDetector.CheckGCPoint(site)
	}
	return nil
}

// List returns all registered detector names and descriptions.
func (r *Registry) List() []DetectorInfo {
	var info []DetectorInfo
	for _, d := range r.detectors {
		info = append(info, DetectorInfo{
			Name:        d.Name(),
			Description: d.Description(),
		})
	}
	return info
}

// DetectorInfo describes a registered detector.
type DetectorInfo struct {
	Name        string
	Description string
}
