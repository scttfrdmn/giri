// Additional unit tests for detector coverage gaps (issue #108):
// CheckFinalize paths, RecordReflectConversion, ClearAllUintptrConversions,
// DefaultRegistry.
package detector_test

import (
	"testing"

	"github.com/scttfrdmn/giri/pkg/detector"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

// --- BoundsDetector.CheckFinalize ---

func TestBoundsDetector_CheckFinalize_ReturnsNil(t *testing.T) {
	d := &detector.BoundsDetector{}
	errs := d.CheckFinalize(nil)
	if errs != nil {
		t.Errorf("BoundsDetector.CheckFinalize: want nil, got %v", errs)
	}
}

// --- RaceDetector.CheckFinalize ---

func TestRaceDetector_CheckFinalize_ReturnsNil(t *testing.T) {
	d := detector.NewRaceDetector()
	errs := d.CheckFinalize(nil)
	if errs != nil {
		t.Errorf("RaceDetector.CheckFinalize: want nil, got %v", errs)
	}
}

// --- UnsafeDetector: RecordReflectConversion / CheckFinalize ---

func TestUnsafeDetector_RecordReflectConversion(t *testing.T) {
	d := detector.NewUnsafeDetector()
	ptr := &shadow.Pointer{Alloc: 1, Offset: 0}
	d.RecordReflectConversion("v1", "main.go:10", ptr)

	// CheckFinalize should report the pending conversion.
	errs := d.CheckFinalize(nil)
	if len(errs) != 1 {
		t.Fatalf("want 1 error from pending reflect conversion, got %d", len(errs))
	}
}

func TestUnsafeDetector_ClearAllUintptrConversions(t *testing.T) {
	d := detector.NewUnsafeDetector()
	ptr := &shadow.Pointer{Alloc: 1, Offset: 0}
	d.RecordUintptrConversion("v1", "main.go:5", ptr)
	d.RecordUintptrConversion("v2", "main.go:6", ptr)

	// Clearing all should leave no pending conversions.
	d.ClearAllUintptrConversions()
	errs := d.CheckFinalize(nil)
	if len(errs) != 0 {
		t.Errorf("after ClearAll: want 0 errors, got %d", len(errs))
	}
}

// --- DefaultRegistry ---

func TestDefaultRegistry_NotNil(t *testing.T) {
	reg := detector.DefaultRegistry()
	if reg == nil {
		t.Fatal("DefaultRegistry returned nil")
	}
}

func TestDefaultRegistry_List(t *testing.T) {
	reg := detector.DefaultRegistry()
	list := reg.List()
	if len(list) == 0 {
		t.Error("DefaultRegistry.List: expected at least one detector")
	}
	for _, info := range list {
		if info.Name == "" {
			t.Error("detector info has empty name")
		}
		if info.Description == "" {
			t.Error("detector info has empty description")
		}
	}
}

func TestDefaultRegistry_CheckAccess_NoError(t *testing.T) {
	mem := shadow.NewMemory()
	id := mem.Allocate(shadow.AllocHeap, 64, "*int", "t:alloc")
	ptr := &shadow.Pointer{Alloc: id, Offset: 0}
	reg := detector.DefaultRegistry()
	errs := reg.CheckAccess(mem, ptr, 8, shadow.AccessRead, "t:read", 1, nil)
	if errs != nil {
		t.Errorf("expected no errors on valid access, got %v", errs)
	}
}

func TestDefaultRegistry_Finalize_NoLeaks(t *testing.T) {
	mem := shadow.NewMemory()
	reg := detector.DefaultRegistry()
	errs := reg.Finalize(mem)
	if len(errs) != 0 {
		t.Errorf("Finalize with no allocations: want 0 errors, got %v", errs)
	}
}

// --- ArenaDetector.Name and Description ---

func TestArenaDetector_NameDescription(t *testing.T) {
	d := &detector.ArenaDetector{}
	if d.Name() == "" {
		t.Error("ArenaDetector.Name() should not be empty")
	}
	if d.Description() == "" {
		t.Error("ArenaDetector.Description() should not be empty")
	}
}
