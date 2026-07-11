// Additional unit tests to raise coverage for pkg/report (issue #108):
// all classifyError branches, unsupported format, text no-violations path,
// ViolationWithStack wrapping, summary counts, multi-goroutine report.
package report_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/scttfrdmn/giri/pkg/report"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

// --- classifyError branch coverage ---

func TestClassify_UseAfterFree(t *testing.T) {
	err := &shadow.UseAfterFreeError{AllocID: 1, AccessSite: "main.go:10"}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "use-after-free" {
		t.Errorf("want use-after-free, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_UseAfterFreeArena(t *testing.T) {
	err := &shadow.UseAfterFreeError{AllocID: 1, ArenaID: 42, AccessSite: "main.go:11"}
	rpt := report.Build([]error{err}, nil)
	f := rpt.Findings[0]
	if f.Category != "use-after-free" {
		t.Errorf("category: want use-after-free, got %q", f.Category)
	}
	if !strings.Contains(f.Hint, "safearena") {
		t.Errorf("hint should mention safearena for arena UAF; got %q", f.Hint)
	}
}

func TestClassify_DoubleFree(t *testing.T) {
	err := &shadow.DoubleFreeError{AllocID: 1, SecondFree: "main.go:20"}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "double-free" {
		t.Errorf("want double-free, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_OutOfBounds(t *testing.T) {
	err := &shadow.OutOfBoundsError{AllocID: 1, Offset: 100, AllocSize: 8, Site: "main.go:30"}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "out-of-bounds" {
		t.Errorf("want out-of-bounds, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_UnsafePointer(t *testing.T) {
	err := &shadow.UnsafePointerViolation{Rule: shadow.RuleConversion, Site: "main.go:40"}
	rpt := report.Build([]error{err}, nil)
	if !strings.HasPrefix(rpt.Findings[0].Category, "unsafe-pointer-") {
		t.Errorf("want unsafe-pointer-*, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_UninitializedRead(t *testing.T) {
	err := &shadow.UninitializedReadError{Site: "main.go:50"}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "uninitialized-read" {
		t.Errorf("want uninitialized-read, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_EscapedPointer(t *testing.T) {
	err := &shadow.EscapedPointerError{
		AllocID: 1, EscapeSite: "main.go:60", EscapeKind: "return",
	}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "arena-escape" {
		t.Errorf("want arena-escape, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_DataRace(t *testing.T) {
	err := &shadow.DataRaceError{AllocID: 1, Write2Site: "main.go:70"}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "data-race" {
		t.Errorf("want data-race, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_TypeAssertion(t *testing.T) {
	err := &shadow.TypeAssertionError{Site: "main.go:80"}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "type-assertion-failure" {
		t.Errorf("want type-assertion-failure, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_GoroutineLeak(t *testing.T) {
	err := &shadow.GoroutineLeakError{GID: 2, BlockSite: "main.go:90"}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "goroutine-leak" {
		t.Errorf("want goroutine-leak, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_Deadlock(t *testing.T) {
	err := &shadow.DeadlockError{}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "deadlock" {
		t.Errorf("want deadlock, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_WaitGroupNegative(t *testing.T) {
	err := &shadow.WaitGroupNegativeError{Site: "main.go:100"}
	rpt := report.Build([]error{err}, nil)
	if rpt.Findings[0].Category != "waitgroup" {
		t.Errorf("want waitgroup, got %q", rpt.Findings[0].Category)
	}
}

func TestClassify_Unknown(t *testing.T) {
	// An unrecognized error type should produce an "unknown" category.
	err := fmt.Errorf("some unexpected error")
	rpt := report.Build([]error{err}, nil)
	cat := rpt.Findings[0].Category
	if cat != "unknown" && cat != "other" {
		t.Errorf("want unknown or other for unrecognized error, got %q", cat)
	}
}

// --- Unsupported format ---

func TestWrite_UnsupportedFormat(t *testing.T) {
	rpt := report.Build(nil, nil)
	var buf bytes.Buffer
	err := rpt.Write(&buf, report.Format(999))
	if err == nil {
		t.Error("Write with unsupported format should return error")
	}
}

// --- Text output: no violations ---

func TestWriteText_NoViolations(t *testing.T) {
	rpt := report.Build(nil, nil)
	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatText); err != nil {
		t.Fatalf("Write text (no violations): %v", err)
	}
	if !strings.Contains(buf.String(), "No violations") {
		t.Errorf("expected 'No violations' in output; got: %q", buf.String())
	}
}

// --- Summary counts ---

func TestSummaryCounts(t *testing.T) {
	violations := []error{
		&shadow.OutOfBoundsError{AllocID: 1, Offset: 1, AllocSize: 4, Site: "s:1"},
		&shadow.DataRaceError{AllocID: 2, Write2Site: "s:2"},
		&shadow.DataRaceError{AllocID: 3, Write2Site: "s:3"},
	}
	rpt := report.Build(violations, nil)
	if rpt.Summary.TotalFindings != 3 {
		t.Errorf("TotalFindings: want 3, got %d", rpt.Summary.TotalFindings)
	}
	if rpt.Summary.ByCategory["data-race"] != 2 {
		t.Errorf("ByCategory[data-race]: want 2, got %d", rpt.Summary.ByCategory["data-race"])
	}
	if rpt.Summary.ByCategory["out-of-bounds"] != 1 {
		t.Errorf("ByCategory[out-of-bounds]: want 1, got %d", rpt.Summary.ByCategory["out-of-bounds"])
	}
}

// --- JSON schema completeness ---

func TestWriteJSON_SummaryFields(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.DataRaceError{AllocID: 1, Write2Site: "main.go:10"},
	}, nil)
	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatJSON); err != nil {
		t.Fatalf("Write JSON: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	summary, ok := out["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("missing 'summary' in JSON output")
	}
	if summary["total_findings"].(float64) != 1 {
		t.Errorf("total_findings: want 1, got %v", summary["total_findings"])
	}
}

// --- Text output with stack trace ---

func TestWriteText_WithStackTrace(t *testing.T) {
	// Wrap the error in a ViolationWithStack-like structure by providing a
	// stackTracer. For report_test (external package) we can't directly
	// construct ViolationWithStack, so we test via report.Build which accepts
	// plain errors. The stack trace field is populated by the interpreter; here
	// we verify that a finding with a non-empty StackTrace is rendered.
	rpt := report.Build([]error{
		&shadow.DataRaceError{AllocID: 1, Write2Site: "main.go:10"},
	}, nil)
	// Manually inject a stack trace into the finding for rendering test.
	rpt.Findings[0].StackTrace = "goroutine 1:\n  main.go:10"

	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatText); err != nil {
		t.Fatalf("Write text with stack: %v", err)
	}
	if !strings.Contains(buf.String(), "stack trace") {
		t.Errorf("expected 'stack trace' in text output; got: %q", buf.String())
	}
}

// --- Detector coverage via report.Build ---

func TestBuild_EmptyInput(t *testing.T) {
	rpt := report.Build(nil, nil)
	if rpt == nil {
		t.Fatal("Build(nil,nil) returned nil")
	}
	if rpt.ExitCode() != 0 {
		t.Errorf("empty build: want exit 0, got %d", rpt.ExitCode())
	}
}

// --- HTML output ---

func TestWriteHTML_NoViolations(t *testing.T) {
	rpt := report.Build(nil, nil)
	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatHTML); err != nil {
		t.Fatalf("Write HTML (no violations): %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("HTML output should contain <!DOCTYPE html>")
	}
	if !strings.Contains(html, "No violations") {
		t.Error("HTML output should contain 'No violations'")
	}
}

func TestWriteHTML_WithFindings(t *testing.T) {
	violations := []error{
		&shadow.DataRaceError{AllocID: 1, Write2Site: "main.go:10", Write1Site: "main.go:5"},
		&shadow.ContextCancelLeakError{Site: "main.go:20", GID: 1},
	}
	rpt := report.Build(violations, nil)
	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatHTML); err != nil {
		t.Fatalf("Write HTML: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "data-race") {
		t.Error("HTML should contain 'data-race'")
	}
	if !strings.Contains(html, "context-cancel-leak") {
		t.Error("HTML should contain 'context-cancel-leak'")
	}
	if !strings.Contains(html, "badge-error") {
		t.Error("HTML should have error badge for data-race")
	}
	if !strings.Contains(html, "badge-warning") {
		t.Error("HTML should have warning badge for context-cancel-leak")
	}
}

func TestClassify_ContextCancelLeak(t *testing.T) {
	err := &shadow.ContextCancelLeakError{Site: "main.go:15", GID: 1}
	rpt := report.Build([]error{err}, nil)
	f := rpt.Findings[0]
	if f.Category != "context-cancel-leak" {
		t.Errorf("category: want context-cancel-leak, got %q", f.Category)
	}
	if f.Severity != report.SeverityWarning {
		t.Errorf("severity: want WARNING, got %v", f.Severity)
	}
}

// TestCategoryOfMatchesReport guards the invariant that shadow.CategoryOf (used
// by the interpreter for //giri:ignore category filtering, #229) produces the
// same category slug as report.CategoryFor for every violation error type. If a
// new error type is added, extend both classifiers and this table together.
func TestCategoryOfMatchesReport(t *testing.T) {
	errs := []error{
		&shadow.UseAfterFreeError{AccessSite: "m.go:1"},
		&shadow.DoubleFreeError{SecondFree: "m.go:1"},
		&shadow.OutOfBoundsError{Site: "m.go:1"},
		&shadow.UnsafePointerViolation{Rule: shadow.RuleConversion, Site: "m.go:1"},
		&shadow.UninitializedReadError{Site: "m.go:1"},
		&shadow.EscapedPointerError{EscapeSite: "m.go:1"},
		&shadow.DataRaceError{Write2Site: "m.go:1"},
		&shadow.NilPointerDerefError{Site: "m.go:1"},
		&shadow.TypeAssertionError{Site: "m.go:1"},
		&shadow.GoroutineLeakError{BlockSite: "m.go:1"},
		&shadow.DeadlockError{},
		&shadow.WaitGroupNegativeError{Site: "m.go:1"},
		&shadow.DoubleCloseError{Site: "m.go:1"},
		&shadow.ResourceDoubleCloseError{Site: "m.go:1"},
		&shadow.NilMapWriteError{Site: "m.go:1"},
		&shadow.DivisionByZeroError{Site: "m.go:1"},
		&shadow.ContextCancelLeakError{Site: "m.go:1"},
		&shadow.MutexUnlockError{Site: "m.go:1"},
		&shadow.NegativeShiftError{Site: "m.go:1"},
		&shadow.IntegerTruncationError{Site: "m.go:1"},
		&shadow.NilChannelError{Site: "m.go:1"},
		&shadow.InvalidMakeArgError{Site: "m.go:1"},
		&shadow.InvalidUnsafeArgError{Site: "m.go:1"},
	}
	for _, err := range errs {
		got := shadow.CategoryOf(err)
		want := report.CategoryFor(err)
		if got != want {
			t.Errorf("%T: CategoryOf=%q, report.CategoryFor=%q", err, got, want)
		}
		if got == "" {
			t.Errorf("%T: CategoryOf returned empty; not classified", err)
		}
	}
}
