package lsp

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/scttfrdmn/giri/pkg/report"
)

// findingToDiagnostic converts one report.Finding into an LSP Diagnostic and
// the absolute file path it belongs to. ok is false when the finding has no
// usable location (nothing to squiggle).
//
// Locations from the interpreter look like "/abs/path/file.go:42" or
// "/abs/path/file.go:42:10"; parsing is delegated to report.ParseLocation so
// the LSP and the SARIF/text reporters agree. LSP positions are 0-based, so the
// 1-based line/column are decremented. When no column is present the diagnostic
// spans the whole line (character 0 to a large end column) so the squiggle is
// visible without needing the source text.
func findingToDiagnostic(f report.Finding) (path string, d Diagnostic, ok bool) {
	file, line, col := report.ParseLocation(f.Location)
	if file == "" || line <= 0 {
		return "", Diagnostic{}, false
	}

	startChar := 0
	endChar := lineEndColumn // whole-line span when no column is known
	if col > 0 {
		startChar = col - 1
		endChar = col - 1
	}
	rng := Range{
		Start: Position{Line: line - 1, Character: startChar},
		End:   Position{Line: line - 1, Character: endChar},
	}

	msg := f.Message
	if f.Hint != "" {
		msg += "\n\nhint: " + f.Hint
	}

	return file, Diagnostic{
		Range:    rng,
		Severity: severityFor(f.Severity),
		Code:     f.Category,
		Source:   "giri",
		Message:  msg,
	}, true
}

// lineEndColumn is used as the end character for a whole-line diagnostic when
// the finding carries no column. It is intentionally large so the squiggle
// covers the line regardless of its length; editors clamp to the real end.
const lineEndColumn = 1 << 20

// severityFor maps a report severity to the LSP diagnostic severity enum.
func severityFor(s report.Severity) DiagnosticSeverity {
	switch s {
	case report.SeverityError:
		return severityError
	case report.SeverityWarning:
		return severityWarning
	default:
		return severityInformation
	}
}

// pathToURI converts an absolute filesystem path to a file:// URI. It is the
// inverse of uriToPath for the paths Giri emits (always absolute, from the
// go/token FileSet).
func pathToURI(path string) string {
	// filepath.ToSlash handles Windows separators; url.URL builds a proper
	// file:// URI with the leading slash and percent-encoding.
	u := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/")}
	return u.String()
}

// uriToPath converts a file:// URI to a filesystem path. Non-file URIs and
// unparseable input yield "".
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	p := u.Path
	// On Windows a URI path is "/C:/..."; strip the leading slash. On POSIX the
	// leading slash is part of the absolute path and must be kept.
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}
