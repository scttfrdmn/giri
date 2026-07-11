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
	FormatHTML                // Self-contained HTML report
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
	StackTrace   string   `json:"stack_trace,omitempty"`
	GoroutineID  int64    `json:"goroutine_id,omitempty"`
	// ReproSeed is non-zero when this violation was found by RunN's PCT sweep.
	// Reproduce the exact run with: giri -strategy pct -seed <ReproSeed> ./...
	ReproSeed int64 `json:"repro_seed,omitempty"`
}

// stackTracer is implemented by ViolationWithStack in pkg/interpreter.
// Defined here to avoid an import cycle (report → interpreter is forbidden).
// ViolationWithStack satisfies this interface implicitly via its StackTrace()
// and GoroutineID() methods.
type stackTracer interface {
	StackTrace() string
	GoroutineID() int64
	Unwrap() error
}

// reproSeeder is optionally implemented by violations that were discovered
// via RunN's PCT multi-run sweep. When present, the replay seed is surfaced
// in the text report as a "Replay:" line.
type reproSeeder interface {
	ReproSeedValue() int64
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
		// Extract stack trace, goroutine ID, and replay seed before classifying.
		var stackTrace string
		var goroutineID int64
		var reproSeed int64
		underlying := err
		if st, ok := err.(stackTracer); ok {
			stackTrace = st.StackTrace()
			goroutineID = st.GoroutineID()
			underlying = st.Unwrap()
		}
		if rs, ok := err.(reproSeeder); ok {
			reproSeed = rs.ReproSeedValue()
		}

		f := classifyError(underlying)
		f.StackTrace = stackTrace
		f.GoroutineID = goroutineID
		f.ReproSeed = reproSeed
		r.Findings = append(r.Findings, f)

		r.Summary.TotalFindings++
		r.Summary.BySeverity[f.Severity.String()]++
		r.Summary.ByCategory[f.Category]++
	}

	return r
}

// classifyError converts a raw error into a structured Finding.
// The err received here is already unwrapped (via stackTracer.Unwrap) so a
// direct type switch is correct; no wrapping indirection exists at this point.
func classifyError(err error) Finding {
	switch e := err.(type) { //nolint:errorlint
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

	case *shadow.GoroutineLeakError:
		return Finding{
			Severity:    SeverityError,
			Category:    "goroutine-leak",
			Message:     e.Error(),
			Location:    e.BlockSite,
			GoroutineID: e.GID,
			Hint: "Ensure every goroutine that reads from a channel has a corresponding " +
				"sender, or use select with a default clause / done channel to avoid " +
				"permanent blocking.",
		}

	case *shadow.DeadlockError:
		return Finding{
			Severity: SeverityError,
			Category: "deadlock",
			Message:  e.Error(),
			Hint: "All goroutines are blocked. Check for circular channel dependencies, " +
				"missing sends, or missing closes that leave receivers waiting forever.",
		}

	case *shadow.WaitGroupNegativeError:
		return Finding{
			Severity: SeverityError,
			Category: "waitgroup",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Each Done() call must be matched by an Add(1). Check that Add is called before spawning goroutines and that Done is not called more times than Add.",
		}

	case *shadow.DoubleCloseError:
		return Finding{
			Severity: SeverityError,
			Category: "closed-channel",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Close a channel only once. Use a sync.Once or a done channel pattern to avoid double-close.",
		}

	case *shadow.NilMapWriteError:
		return Finding{
			Severity: SeverityError,
			Category: "nil-map-write",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Initialize maps with make() before writing: m := make(map[K]V).",
		}

	case *shadow.DivisionByZeroError:
		return Finding{
			Severity: SeverityError,
			Category: "division-by-zero",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Check for zero divisor before dividing. Consider using a guard: if b == 0 { ... }.",
		}

	case *shadow.ContextCancelLeakError:
		return Finding{
			Severity: SeverityWarning,
			Category: "context-cancel-leak",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Always call the cancel function returned by context.WithCancel/WithTimeout/WithDeadline, typically with: defer cancel()",
		}

	case *shadow.MutexUnlockError:
		return Finding{
			Severity: SeverityError,
			Category: "mutex-unlock",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Ensure each Unlock() call is preceded by a matching Lock(). Use defer mu.Unlock() immediately after mu.Lock() to avoid mismatches.",
		}

	case *shadow.NegativeShiftError:
		return Finding{
			Severity: SeverityError,
			Category: "negative-shift",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Shift count must be non-negative. Guard with: if n >= 0 { x << n }. Consider using unsigned types for shift counts.",
		}

	case *shadow.IntegerTruncationError:
		return Finding{
			Severity: SeverityWarning,
			Category: "integer-truncation",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "The converted value does not fit the destination type and wraps around. Validate the range before converting, or use a wider type. If the wrap-around is intentional, this detector is opt-in and can be disabled.",
		}

	case *shadow.NilChannelError:
		return Finding{
			Severity: SeverityError,
			Category: "nil-channel",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "Initialize channels with make(chan T) before use. close(nil) panics; send/receive on nil channel blocks forever.",
		}

	case *shadow.InvalidMakeArgError:
		return Finding{
			Severity: SeverityError,
			Category: "make-invalid",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "make() length and capacity arguments must be non-negative. Check for negative values before calling make().",
		}

	case *shadow.InvalidUnsafeArgError:
		return Finding{
			Severity: SeverityError,
			Category: "unsafe-slice",
			Message:  e.Error(),
			Location: e.Site,
			Hint:     "unsafe.Slice requires a non-nil pointer and a non-negative length. Validate both arguments before calling unsafe.Slice.",
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
	case FormatHTML:
		return r.writeHTML(w)
	default:
		return fmt.Errorf("unsupported format: %d", format)
	}
}

// textWriter wraps an io.Writer and captures the first write error so that
// individual fmt.Fprintf calls do not need to check errors one by one.
type textWriter struct {
	w   io.Writer
	err error
}

func (tw *textWriter) printf(format string, args ...any) {
	if tw.err != nil {
		return
	}
	_, tw.err = fmt.Fprintf(tw.w, format, args...)
}

func (tw *textWriter) println() {
	if tw.err != nil {
		return
	}
	_, tw.err = fmt.Fprintln(tw.w)
}

func (r *Report) writeText(w io.Writer) error {
	tw := &textWriter{w: w}

	// Header
	tw.printf("╔══════════════════════════════════════════════╗\n")
	tw.printf("║  Giri - Go IR Interpreter                    ║\n")
	tw.printf("║  Undefined Behavior Detection Report          ║\n")
	tw.printf("╚══════════════════════════════════════════════╝\n\n")

	if len(r.Findings) == 0 {
		tw.printf("No violations detected.\n\n")
	} else {
		tw.printf("Found %d violation(s):\n\n", len(r.Findings))

		for i, f := range r.Findings {
			tw.printf("── [%d] %s: %s ──\n", i+1, f.Severity, f.Category)
			tw.printf("%s\n", f.Message)
			if f.StackTrace != "" {
				tw.printf("\n  stack trace:\n")
				for _, line := range strings.Split(strings.TrimRight(f.StackTrace, "\n"), "\n") {
					tw.printf("    %s\n", line)
				}
			}
			if f.ReproSeed != 0 {
				tw.printf("\n  replay: giri -strategy pct -seed %d ./...\n", f.ReproSeed)
			}
			if f.Hint != "" {
				tw.printf("\n  hint: %s\n", f.Hint)
			}
			tw.println()
		}
	}

	// Summary
	tw.printf("── Summary ──\n")
	for sev, count := range r.Summary.BySeverity {
		tw.printf("  %s: %d\n", sev, count)
	}

	if r.Stats != nil {
		tw.printf("\n── Memory ──\n")
		tw.printf("  %s\n", r.Stats)
	}

	return tw.err
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

// writeHTML produces a self-contained HTML report with inline CSS.
// No external resources are required; the output is a single .html file.
func (r *Report) writeHTML(w io.Writer) error {
	tw := &textWriter{w: w}

	tw.printf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Giri Report</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Segoe UI',system-ui,sans-serif;background:#f8f9fa;color:#212529;padding:1.5rem}
h1{font-size:1.5rem;margin-bottom:.25rem}
.subtitle{color:#6c757d;font-size:.9rem;margin-bottom:1.5rem}
.summary{background:#fff;border:1px solid #dee2e6;border-radius:.5rem;padding:1rem 1.5rem;margin-bottom:1.5rem;display:flex;gap:2rem;flex-wrap:wrap}
.summary-item{display:flex;flex-direction:column;align-items:center}
.summary-count{font-size:1.75rem;font-weight:700}
.summary-label{font-size:.75rem;color:#6c757d;text-transform:uppercase;letter-spacing:.05em}
.no-violations{background:#d1e7dd;border:1px solid #a3cfbb;border-radius:.5rem;padding:1rem 1.5rem;color:#0a3622;font-weight:500;margin-bottom:1.5rem}
.finding{background:#fff;border:1px solid #dee2e6;border-radius:.5rem;margin-bottom:1rem;overflow:hidden}
.finding-header{display:flex;align-items:center;gap:.75rem;padding:.75rem 1rem;border-bottom:1px solid #dee2e6}
.badge{font-size:.75rem;font-weight:600;padding:.2em .6em;border-radius:.25rem;text-transform:uppercase}
.badge-error{background:#f8d7da;color:#842029}
.badge-warning{background:#fff3cd;color:#664d03}
.badge-info{background:#cff4fc;color:#055160}
.finding-num{font-size:.8rem;color:#6c757d}
.finding-category{font-weight:600;font-size:.95rem}
.finding-body{padding:1rem}
.finding-message{font-family:'Cascadia Code','JetBrains Mono',monospace;font-size:.85rem;background:#f8f9fa;border:1px solid #e9ecef;border-radius:.25rem;padding:.75rem;white-space:pre-wrap;word-break:break-all;margin-bottom:.75rem}
.finding-location{font-size:.85rem;color:#495057;margin-bottom:.5rem}
.finding-location span{font-family:'Cascadia Code','JetBrains Mono',monospace;color:#0d6efd}
.finding-hint{font-size:.85rem;color:#495057;border-left:3px solid #0d6efd;padding:.5rem .75rem;background:#e8f0fe;margin-bottom:.5rem}
.stack-toggle{background:none;border:1px solid #dee2e6;border-radius:.25rem;font-size:.8rem;cursor:pointer;padding:.25rem .5rem;color:#6c757d;margin-bottom:.5rem}
.stack-trace{display:none;font-family:'Cascadia Code','JetBrains Mono',monospace;font-size:.8rem;background:#f8f9fa;border:1px solid #e9ecef;border-radius:.25rem;padding:.75rem;white-space:pre-wrap}
.replay{font-size:.8rem;font-family:'Cascadia Code','JetBrains Mono',monospace;color:#6f42c1;margin-top:.5rem}
.stats{color:#6c757d;font-size:.85rem;margin-top:1.5rem}
</style>
</head>
<body>
<h1>Giri &mdash; Undefined Behavior Report</h1>
<p class="subtitle">Go IR Interpreter &middot; <a href="https://github.com/scttfrdmn/giri">github.com/scttfrdmn/giri</a></p>
`)

	// Summary bar
	tw.printf(`<div class="summary">
  <div class="summary-item"><span class="summary-count">%d</span><span class="summary-label">Total</span></div>
`, r.Summary.TotalFindings)
	for _, sev := range []string{"ERROR", "WARNING", "INFO"} {
		if n := r.Summary.BySeverity[sev]; n > 0 {
			tw.printf(`  <div class="summary-item"><span class="summary-count">%d</span><span class="summary-label">%s</span></div>
`, n, sev)
		}
	}
	tw.printf("</div>\n")

	if len(r.Findings) == 0 {
		tw.printf(`<div class="no-violations">&#10003; No violations detected.</div>`)
	}

	for i, f := range r.Findings {
		badgeClass := "badge-info"
		switch f.Severity {
		case SeverityError:
			badgeClass = "badge-error"
		case SeverityWarning:
			badgeClass = "badge-warning"
		}
		tw.printf(`<div class="finding">
  <div class="finding-header">
    <span class="finding-num">#%d</span>
    <span class="badge %s">%s</span>
    <span class="finding-category">%s</span>
  </div>
  <div class="finding-body">
    <div class="finding-message">%s</div>
`, i+1, badgeClass, htmlEscape(f.Severity.String()), htmlEscape(f.Category), htmlEscape(f.Message))

		if f.Location != "" {
			tw.printf(`    <div class="finding-location">Location: <span>%s</span></div>`+"\n", htmlEscape(f.Location))
		}
		if f.Hint != "" {
			tw.printf(`    <div class="finding-hint">%s</div>`+"\n", htmlEscape(f.Hint))
		}
		if f.ReproSeed != 0 {
			tw.printf(`    <div class="replay">replay: giri -strategy pct -seed %d ./...</div>`+"\n", f.ReproSeed)
		}
		if f.StackTrace != "" {
			tw.printf(`    <button class="stack-toggle" onclick="var s=this.nextElementSibling;s.style.display=s.style.display==='block'?'none':'block'">&#9654; Stack trace</button>
    <div class="stack-trace">%s</div>
`, htmlEscape(f.StackTrace))
		}
		tw.printf("  </div>\n</div>\n")
	}

	if r.Stats != nil {
		tw.printf(`<div class="stats">%s</div>`+"\n", htmlEscape(r.Stats.String()))
	}

	tw.printf("</body>\n</html>\n")
	return tw.err
}

// htmlReplacer replaces &, <, > with HTML entities in a single pass.
var htmlReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// htmlEscape replaces &, <, > with HTML entities.
func htmlEscape(s string) string {
	return htmlReplacer.Replace(s)
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

// CategoryFor returns the report category string for a violation error,
// unwrapping ViolationWithStack if present. This allows test code to check
// precise category names (e.g. "nil-pointer-deref") rather than substrings
// of error messages (e.g. "nil pointer"). (#132)
func CategoryFor(err error) string {
	underlying := err
	if st, ok := err.(stackTracer); ok {
		underlying = st.Unwrap()
	}
	return classifyError(underlying).Category
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
