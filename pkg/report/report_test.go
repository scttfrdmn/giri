package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/scttfrdmn/giri/pkg/report"
	"github.com/scttfrdmn/giri/pkg/shadow"
)

func TestExitCode(t *testing.T) {
	rpt := report.Build(nil, nil)
	if rpt.ExitCode() != 0 {
		t.Errorf("empty report: want exit 0, got %d", rpt.ExitCode())
	}

	rptErr := report.Build([]error{
		&shadow.OutOfBoundsError{AllocID: 1, Offset: 10, AllocSize: 4, Site: "main.go:5"},
	}, nil)
	if rptErr.ExitCode() != 1 {
		t.Errorf("report with error: want exit 1, got %d", rptErr.ExitCode())
	}
}

func TestClassifyNilPointerDeref(t *testing.T) {
	err := &shadow.NilPointerDerefError{Site: "main.go:42", GID: 1}
	rpt := report.Build([]error{err}, nil)
	if len(rpt.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d", len(rpt.Findings))
	}
	f := rpt.Findings[0]
	if !strings.Contains(f.Category, "nil") {
		t.Errorf("category %q should contain 'nil'", f.Category)
	}
	if f.Severity != report.SeverityError {
		t.Errorf("want SeverityError, got %v", f.Severity)
	}
}

func TestWriteJSON(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.OutOfBoundsError{AllocID: 1, Offset: 10, AllocSize: 4, Site: "main.go:5"},
	}, nil)
	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatJSON); err != nil {
		t.Fatalf("Write JSON: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	findings, ok := out["findings"].([]interface{})
	if !ok || len(findings) != 1 {
		t.Errorf("want 1 finding in JSON, got %v", out["findings"])
	}
}

func TestWriteSARIF(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.OutOfBoundsError{
			AllocID: 1, Offset: 10, AllocSize: 4,
			Site: "/home/user/project/main.go:42:10",
		},
		&shadow.NilPointerDerefError{Site: "/home/user/project/pkg/foo.go:17:3", GID: 1},
	}, nil)

	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatSARIF); err != nil {
		t.Fatalf("Write SARIF: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid SARIF JSON: %v\n%s", err, buf.String())
	}

	// Check top-level fields.
	if out["version"] != "2.1.0" {
		t.Errorf("SARIF version: want 2.1.0, got %v", out["version"])
	}

	runs, ok := out["runs"].([]interface{})
	if !ok || len(runs) != 1 {
		t.Fatalf("want 1 run, got %v", out["runs"])
	}
	run := runs[0].(map[string]interface{})

	// Check tool.
	tool := run["tool"].(map[string]interface{})
	driver := tool["driver"].(map[string]interface{})
	if driver["name"] != "giri" {
		t.Errorf("driver name: want giri, got %v", driver["name"])
	}

	// Check rules deduplicated (2 different categories → 2 rules).
	rules := driver["rules"].([]interface{})
	if len(rules) != 2 {
		t.Errorf("want 2 rules, got %d", len(rules))
	}

	// Check results.
	results := run["results"].([]interface{})
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}

	// First result should have a location with line 42.
	res0 := results[0].(map[string]interface{})
	if res0["level"] != "error" {
		t.Errorf("level: want error, got %v", res0["level"])
	}
	locs, ok := res0["locations"].([]interface{})
	if !ok || len(locs) == 0 {
		t.Fatalf("want locations in first result")
	}
	physLoc := locs[0].(map[string]interface{})["physicalLocation"].(map[string]interface{})
	region := physLoc["region"].(map[string]interface{})
	if region["startLine"].(float64) != 42 {
		t.Errorf("startLine: want 42, got %v", region["startLine"])
	}
}

func TestWriteText(t *testing.T) {
	rpt := report.Build([]error{
		&shadow.OutOfBoundsError{AllocID: 1, Offset: 10, AllocSize: 4, Site: "main.go:5"},
	}, nil)
	var buf bytes.Buffer
	if err := rpt.Write(&buf, report.FormatText); err != nil {
		t.Fatalf("Write text: %v", err)
	}
	if !strings.Contains(buf.String(), "violation") {
		t.Errorf("text output should mention 'violation'")
	}
}
