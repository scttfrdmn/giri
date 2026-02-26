// Package report formats Giri's findings into human-readable and
// machine-parseable output.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
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

	case *shadow.NilPointerDerefError:
		return Finding{
			Severity: SeverityError,
			Category: "nil-pointer-deref",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Check for nil before dereferencing. Map lookups and type assertions return zero values for absent keys.",
		}

	case *shadow.TypeAssertionError:
		return Finding{
			Severity: SeverityError,
			Category: "type-assertion-failure",
			Message:  e.Error(),
			Location: e.Site,
			Hint: "Use the comma-ok form (v, ok := x.(T)) to handle assertion failures " +
				"safely, or ensure the interface always holds the expected concrete type.",
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
	case FormatSARIF:
		return r.writeSARIF(w)
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

// --- SARIF 2.1.0 support ---

type sarifRoot struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	ShortDescription sarifMessage `json:"shortDescription"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           *sarifRegion  `json:"region,omitempty"`
}

type sarifArtifact struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

func (r *Report) writeSARIF(w io.Writer) error {
	// Deduplicate rules by ruleId.
	ruleMap := make(map[string]sarifRule)
	for _, f := range r.Findings {
		ruleID := "giri/" + f.Category
		if _, exists := ruleMap[ruleID]; !exists {
			ruleMap[ruleID] = sarifRule{
				ID:               ruleID,
				Name:             sarifRuleName(f.Category),
				ShortDescription: sarifMessage{Text: sarifCategoryDesc(f.Category)},
			}
		}
	}
	rules := make([]sarifRule, 0, len(ruleMap))
	for _, rule := range ruleMap {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })

	// Build results.
	results := make([]sarifResult, 0, len(r.Findings))
	for _, f := range r.Findings {
		res := sarifResult{
			RuleID:  "giri/" + f.Category,
			Level:   sarifSeverityLevel(f.Severity),
			Message: sarifMessage{Text: f.Message},
		}
		if loc := parseSARIFLocation(f.Location); loc != nil {
			res.Locations = []sarifLocation{*loc}
		}
		results = append(results, res)
	}

	root := sarifRoot{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "giri",
					Version:        "0.6.0",
					InformationURI: "https://github.com/scttfrdmn/giri",
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(root)
}

// parseSARIFLocation extracts a SARIF location from a site string.
// Site strings from the interpreter look like "/abs/path/file.go:42:10" or
// "/abs/path/file.go:42". Returns nil if parsing fails.
func parseSARIFLocation(site string) *sarifLocation {
	if site == "" {
		return nil
	}
	// Try "file:line:col" then "file:line".
	filePath, line := splitFileLine(site)
	if filePath == "" || line <= 0 {
		return nil
	}
	uri := filepath.ToSlash(filePath)
	return &sarifLocation{
		PhysicalLocation: sarifPhysicalLocation{
			ArtifactLocation: sarifArtifact{
				URI:       uri,
				URIBaseID: "%SRCROOT%",
			},
			Region: &sarifRegion{StartLine: line},
		},
	}
}

// splitFileLine parses "file:line" or "file:line:col" into (file, line).
func splitFileLine(s string) (string, int) {
	// Strip trailing column if present (file:line:col).
	lastColon := strings.LastIndex(s, ":")
	if lastColon < 0 {
		return "", 0
	}
	tail := s[lastColon+1:]
	col, err := strconv.Atoi(tail)
	if err != nil || col <= 0 {
		// Not a column — treat as line.
		line, err2 := strconv.Atoi(tail)
		if err2 != nil || line <= 0 {
			return "", 0
		}
		return s[:lastColon], line
	}
	// tail is a column; strip it and look for line.
	rest := s[:lastColon]
	prevColon := strings.LastIndex(rest, ":")
	if prevColon < 0 {
		return "", 0
	}
	line, err := strconv.Atoi(rest[prevColon+1:])
	if err != nil || line <= 0 {
		return "", 0
	}
	return rest[:prevColon], line
}

func sarifSeverityLevel(s Severity) string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "note"
	}
}

// sarifRuleName converts a kebab-case category to PascalCase.
func sarifRuleName(cat string) string {
	parts := strings.FieldsFunc(cat, func(r rune) bool {
		return r == '-' || r == '_' || r == '/'
	})
	var b strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

func sarifCategoryDesc(cat string) string {
	switch {
	case cat == "use-after-free":
		return "Memory accessed after it was freed"
	case cat == "double-free":
		return "Memory freed more than once"
	case cat == "out-of-bounds":
		return "Memory access outside allocation bounds"
	case cat == "uninitialized-read":
		return "Memory read before initialization"
	case cat == "arena-escape":
		return "Arena-allocated pointer escapes the arena lifetime"
	case cat == "data-race":
		return "Concurrent unsynchronized memory access"
	case cat == "nil-pointer-deref":
		return "Nil pointer dereference"
	case strings.HasPrefix(cat, "unsafe-pointer"):
		return "Violation of Go's unsafe.Pointer conversion rules"
	default:
		return "Undefined behavior detected by Giri"
	}
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
