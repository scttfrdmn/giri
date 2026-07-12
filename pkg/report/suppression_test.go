// Tests for suppression output, text grouping, and --max-violations (#230).
package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/scttfrdmn/giri/pkg/report"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

// TestAddSuppressed_SummaryAndExitCode verifies suppressed findings are flagged,
// counted in Summary.Suppressed, and excluded from the active severity counts
// and the exit code — even when the suppressed finding is an ERROR.
func TestAddSuppressed_SummaryAndExitCode(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.IntegerTruncationError{Site: "main.go:5"}, // active WARNING
	}, nil)
	rpt.AddSuppressed([]error{
		&shadow.OutOfBoundsError{AllocID: 1, Offset: 10, AllocSize: 4, Site: "main.go:9"}, // suppressed ERROR
	})

	if rpt.Summary.Suppressed != 1 {
		t.Errorf("Summary.Suppressed: want 1, got %d", rpt.Summary.Suppressed)
	}
	// The active WARNING is counted; the suppressed ERROR is not.
	if rpt.Summary.BySeverity["ERROR"] != 0 {
		t.Errorf("suppressed ERROR must not appear in BySeverity[ERROR]; got %d", rpt.Summary.BySeverity["ERROR"])
	}
	// A suppressed ERROR must not flip the exit code.
	if rpt.ExitCode() != 0 {
		t.Errorf("exit code with only a suppressed ERROR: want 0, got %d", rpt.ExitCode())
	}

	// Find the suppressed finding and check its flags.
	var found bool
	for _, f := range rpt.Findings {
		if f.Category == "out-of-bounds" {
			found = true
			if !f.Suppressed {
				t.Error("out-of-bounds finding should be marked Suppressed")
			}
			if f.SuppressReason == "" {
				t.Error("suppressed finding should carry a SuppressReason")
			}
		}
	}
	if !found {
		t.Error("suppressed finding not present in report")
	}
}

// TestSARIF_Suppressions verifies suppressed results carry a suppressions array
// (kind inSource) while active results do not.
func TestSARIF_Suppressions(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.IntegerTruncationError{Site: "main.go:5"},
	}, nil)
	rpt.AddSuppressed([]error{
		&shadow.NilChannelError{Site: "main.go:9"},
	})

	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatSARIF); err != nil {
		t.Fatalf("write sarif: %v", err)
	}

	var root struct {
		Runs []struct {
			Results []struct {
				RuleID       string `json:"ruleId"`
				Suppressions []struct {
					Kind string `json:"kind"`
				} `json:"suppressions"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &root); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}
	if len(root.Runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(root.Runs))
	}
	results := root.Runs[0].Results
	if len(results) != 2 {
		t.Fatalf("want 2 results (active + suppressed), got %d", len(results))
	}
	for _, res := range results {
		switch res.RuleID {
		case "giri/nil-channel": // suppressed
			if len(res.Suppressions) != 1 || res.Suppressions[0].Kind != "inSource" {
				t.Errorf("suppressed result: want suppressions [inSource], got %+v", res.Suppressions)
			}
		case "giri/integer-truncation": // active
			if len(res.Suppressions) != 0 {
				t.Errorf("active result should have no suppressions, got %+v", res.Suppressions)
			}
		}
	}
}

// TestJSON_SuppressedField verifies the suppressed flag flows to JSON output.
func TestJSON_SuppressedField(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.IntegerTruncationError{Site: "main.go:5"},
	}, nil)
	rpt.AddSuppressed([]error{
		&shadow.NilChannelError{Site: "main.go:9"},
	})

	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatJSON); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var out struct {
		Findings []struct {
			Category   string `json:"category"`
			Suppressed bool   `json:"suppressed"`
		} `json:"findings"`
		Summary struct {
			Suppressed int `json:"suppressed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if out.Summary.Suppressed != 1 {
		t.Errorf("summary.suppressed: want 1, got %d", out.Summary.Suppressed)
	}
	for _, f := range out.Findings {
		want := f.Category == "nil-channel"
		if f.Suppressed != want {
			t.Errorf("%s: suppressed=%v, want %v", f.Category, f.Suppressed, want)
		}
	}
}

// TestText_Grouping verifies findings sharing a (category, file) render under
// one group header.
func TestText_Grouping(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.IntegerTruncationError{Site: "a.go:5"},
		&shadow.IntegerTruncationError{Site: "a.go:7"},
	}, nil)

	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatText); err != nil {
		t.Fatalf("write text: %v", err)
	}
	out := buf.String()
	if n := strings.Count(out, "── integer-truncation ("); n != 1 {
		t.Errorf("want exactly 1 group header for two same-category/file findings, got %d\n%s", n, out)
	}
}

// TestText_MaxViolations verifies the active-finding cap and the "N more" line.
func TestText_MaxViolations(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.IntegerTruncationError{Site: "a.go:5"},
		&shadow.IntegerTruncationError{Site: "a.go:7"},
		&shadow.IntegerTruncationError{Site: "a.go:9"},
	}, nil)
	rpt.MaxViolations = 1

	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatText); err != nil {
		t.Fatalf("write text: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "and 2 more") {
		t.Errorf("want '… and 2 more' with max-violations=1, got:\n%s", out)
	}
}
