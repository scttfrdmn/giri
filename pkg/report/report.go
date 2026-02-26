// Package report formats Giri's findings into human-readable and
// machine-parseable output.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/scttfrdmn/giri/pkg/shadow"
)

// Format controls output format.
type Format int

const (
	FormatText  Format = iota // Human-readable terminal output
	FormatJSON                // Machine-parseable JSON
	FormatSARIF               // SARIF format for IDE integration
)

// Finding represents a single detected violation.
type Finding struct {
	Severity     Severity `json:"severity"`
	Category     string   `json:"category"`
	Message      string   `json:"message"`
	Location     string   `json:"location,omitempty"`
	AllocSite    string   `json:"alloc_site,omitempty"`
	FreeSite     string   `json:"free_site,omitempty"`
	Hint         string   `json:"hint,omitempty"`
	DetectorName string   `json:"detector"`
}

// Severity levels for findings.
type Severity int

const (
	SeverityError   Severity = iota // Definite UB
	SeverityWarning                 // Likely bug
	SeverityInfo                    // Informational
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARNING"
	case SeverityInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

// Report holds all findings from a Giri run.
type Report struct {
	Findings []Finding           `json:"findings"`
	Stats    *shadow.MemoryStats `json:"memory_stats,omitempty"`
	Summary  Summary             `json:"summary"`
}

// Summary provides aggregate counts.
type Summary struct {
	TotalFindings int            `json:"total_findings"`
	BySeverity    map[string]int `json:"by_severity"`
	ByCategory    map[string]int `json:"by_category"`
}

// Build creates a Report from a list of errors (violations from the interpreter).
func Build(violations []error, memStats *shadow.MemoryStats) *Report {
	r := &Report{
		Stats: memStats,
		Summary: Summary{
			BySeverity: make(map[string]int),
			ByCategory: make(map[string]int),
		},
	}

	for _, err := range violations {
		f := classifyError(err)
		r.Findings = append(r.Findings, f)

		r.Summary.TotalFindings++
		r.Summary.BySeverity[f.Severity.String()]++
		r.Summary.ByCategory[f.Category]++
	}

	return r
}

// classifyError converts a raw error into a structured Finding.
func classifyError(err error) Finding {
	switch e := err.(type) {
	case *shadow.UseAfterFreeError:
		hint := "Use Clone() to copy values to heap before arena.Free(), or restructure to ensure pointer doesn't outlive its allocation."
		if e.ArenaID != 0 {
			hint = "Arena-allocated pointer was used after arena.Free(). Use safearena.Clone() to copy to heap."
		}
		return Finding{
			Severity:  SeverityError,
			Category:  "use-after-free",
			Message:   e.Error(),
			Location:  e.AccessSite,
			AllocSite: e.AllocSite,
			FreeSite:  e.FreeSite,
			Hint:      hint,
		}

	case *shadow.DoubleFreeError:
		return Finding{
			Severity:  SeverityError,
			Category:  "double-free",
			Message:   e.Error(),
			Location:  e.SecondFree,
			AllocSite: e.AllocSite,
			Hint:      "Ensure Free() is only called once. Use defer for automatic cleanup.",
		}

	case *shadow.OutOfBoundsError:
		return Finding{
			Severity: SeverityError,
			Category: "out-of-bounds",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Check slice/array bounds before access. Consider using len() to validate indices.",
		}

	case *shadow.UnsafePointerViolation:
		return Finding{
			Severity: SeverityError,
			Category: fmt.Sprintf("unsafe-pointer-%s", e.Rule),
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Review the six rules at https://pkg.go.dev/unsafe#Pointer. Consider using safer alternatives.",
		}

	case *shadow.UninitializedReadError:
		return Finding{
			Severity: SeverityError,
			Category: "uninitialized-read",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Initialize memory before reading. Use zero-value initialization or explicit assignment.",
		}

	case *shadow.EscapedPointerError:
		return Finding{
			Severity:  SeverityError,
			Category:  "arena-escape",
			Message:   e.Error(),
			Location:  e.EscapeSite,
			AllocSite: e.AllocSite,
			Hint:      fmt.Sprintf("Arena pointer escapes via %s. Copy to heap with Clone() before the arena is freed.", e.EscapeKind),
		}

	case *shadow.DataRaceError:
		return Finding{
			Severity: SeverityError,
			Category: "data-race",
			Message:  e.Error(),
			Location: e.Write2Site,
			Hint:     "Protect shared data with sync.Mutex, use channels for communication, or use sync/atomic for simple values.",
		}

	default:
		return Finding{
			Severity: SeverityWarning,
			Category: "other",
			Message:  err.Error(),
		}
	}
}

// --- Output Formatters ---

// Write outputs the report in the specified format.
func (r *Report) Write(w io.Writer, format Format) error {
	switch format {
	case FormatText:
		return r.writeText(w)
	case FormatJSON:
		return r.writeJSON(w)
	default:
		return fmt.Errorf("unsupported format: %d", format)
	}
}

func (r *Report) writeText(w io.Writer) error {
	// Header
	fmt.Fprintf(w, "╔══════════════════════════════════════════════╗\n")
	fmt.Fprintf(w, "║  Giri - Go IR Interpreter                    ║\n")
	fmt.Fprintf(w, "║  Undefined Behavior Detection Report          ║\n")
	fmt.Fprintf(w, "╚══════════════════════════════════════════════╝\n\n")

	if len(r.Findings) == 0 {
		fmt.Fprintf(w, "No violations detected.\n\n")
	} else {
		fmt.Fprintf(w, "Found %d violation(s):\n\n", len(r.Findings))

		for i, f := range r.Findings {
			fmt.Fprintf(w, "── [%d] %s: %s ──\n", i+1, f.Severity, f.Category)
			fmt.Fprintf(w, "%s\n", f.Message)
			if f.Hint != "" {
				fmt.Fprintf(w, "\n  hint: %s\n", f.Hint)
			}
			fmt.Fprintln(w)
		}
	}

	// Summary
	fmt.Fprintf(w, "── Summary ──\n")
	for sev, count := range r.Summary.BySeverity {
		fmt.Fprintf(w, "  %s: %d\n", sev, count)
	}

	if r.Stats != nil {
		fmt.Fprintf(w, "\n── Memory ──\n")
		fmt.Fprintf(w, "  %s\n", r.Stats)
	}

	return nil
}

func (r *Report) writeJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// ExitCode returns the appropriate process exit code.
// 0 = no errors, 1 = errors found, 2 = internal error.
func (r *Report) ExitCode() int {
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			return 1
		}
	}
	return 0
}

// HasErrors returns true if any ERROR-severity findings exist.
func (r *Report) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

// FilterByCategory returns findings matching the given category prefix.
func (r *Report) FilterByCategory(prefix string) []Finding {
	var matches []Finding
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Category, prefix) {
			matches = append(matches, f)
		}
	}
	return matches
}
