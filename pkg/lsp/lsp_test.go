package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/report"
)

// --- JSON-RPC framing ---

func TestConnRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	c := newConn(nil, &buf)
	if err := c.notify("test/method", map[string]int{"x": 7}); err != nil {
		t.Fatalf("notify: %v", err)
	}

	// Read it back through a fresh conn over the written bytes.
	rc := newConn(&buf, io.Discard)
	m, err := rc.readMessage()
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if m.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q, want 2.0", m.JSONRPC)
	}
	if m.Method != "test/method" {
		t.Errorf("method = %q, want test/method", m.Method)
	}
	var p struct{ X int }
	if err := json.Unmarshal(m.Params, &p); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if p.X != 7 {
		t.Errorf("params.x = %d, want 7", p.X)
	}
}

func TestReadMessageMissingContentLength(t *testing.T) {
	rc := newConn(strings.NewReader("\r\n{}"), io.Discard)
	if _, err := rc.readMessage(); err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}

// --- diagnostic mapping ---

func TestFindingToDiagnostic(t *testing.T) {
	tests := []struct {
		name      string
		finding   report.Finding
		wantOK    bool
		wantLine  int
		wantStart int
		wantSev   DiagnosticSeverity
		wantCode  string
	}{
		{
			name: "line and column",
			finding: report.Finding{
				Severity: report.SeverityError,
				Category: "nil-pointer-deref",
				Message:  "nil deref",
				Location: "/abs/foo.go:12:5",
			},
			wantOK: true, wantLine: 11, wantStart: 4,
			wantSev: severityError, wantCode: "nil-pointer-deref",
		},
		{
			name: "line only spans whole line",
			finding: report.Finding{
				Severity: report.SeverityWarning,
				Category: "integer-truncation",
				Message:  "trunc",
				Location: "/abs/bar.go:8",
			},
			wantOK: true, wantLine: 7, wantStart: 0,
			wantSev: severityWarning, wantCode: "integer-truncation",
		},
		{
			name:    "no location",
			finding: report.Finding{Category: "deadlock", Message: "deadlock"},
			wantOK:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, d, ok := findingToDiagnostic(tt.finding)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if d.Range.Start.Line != tt.wantLine {
				t.Errorf("start line = %d, want %d", d.Range.Start.Line, tt.wantLine)
			}
			if d.Range.Start.Character != tt.wantStart {
				t.Errorf("start char = %d, want %d", d.Range.Start.Character, tt.wantStart)
			}
			if d.Severity != tt.wantSev {
				t.Errorf("severity = %d, want %d", d.Severity, tt.wantSev)
			}
			if d.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", d.Code, tt.wantCode)
			}
			if d.Source != "giri" {
				t.Errorf("source = %q, want giri", d.Source)
			}
		})
	}
}

func TestURIRoundTrip(t *testing.T) {
	// A POSIX absolute path round-trips through the URI helpers.
	path := "/home/user/proj/main.go"
	uri := pathToURI(path)
	if !strings.HasPrefix(uri, "file://") {
		t.Fatalf("uri = %q, want file:// prefix", uri)
	}
	if got := uriToPath(uri); got != path {
		t.Errorf("round trip = %q, want %q", got, path)
	}
}

// --- initialize capabilities ---

func TestInitializeCapabilities(t *testing.T) {
	req := request(t, 1, "initialize", InitializeParams{})
	out := driveSession(t, interpreter.DefaultConfig(), req)

	msgs := decodeMessages(t, out)
	if len(msgs) == 0 {
		t.Fatal("no response to initialize")
	}
	var res InitializeResult
	remarshal(t, msgs[0].Result, &res)
	if res.ServerInfo.Name != "giri" {
		t.Errorf("serverInfo.name = %q, want giri", res.ServerInfo.Name)
	}
	sync := res.Capabilities.TextDocumentSync
	if !sync.OpenClose {
		t.Error("openClose not advertised")
	}
	// Save must not request the document text — Giri re-reads files from disk.
	if sync.Save.IncludeText {
		t.Error("save.includeText = true, want false")
	}
}

// --- end-to-end: didOpen publishes diagnostics for a buggy program ---

func TestDidOpenPublishesDiagnostics(t *testing.T) {
	root := writeBuggyWorkspace(t)

	initReq := request(t, 1, "initialize", InitializeParams{
		RootURI: pathToURI(root),
	})
	openNote := notification(t, "textDocument/didOpen", DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        pathToURI(filepath.Join(root, "main.go")),
			LanguageID: "go",
		},
	})

	// -no-cache so the run is hermetic regardless of any shared cache state.
	out := driveSessionNoCache(t, interpreter.DefaultConfig(), initReq+openNote)

	msgs := decodeMessages(t, out)
	var found bool
	for _, m := range msgs {
		if m.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var p PublishDiagnosticsParams
		remarshal(t, rawParams(t, m), &p)
		if len(p.Diagnostics) == 0 {
			continue
		}
		for _, d := range p.Diagnostics {
			if d.Code == "division-by-zero" && d.Source == "giri" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no division-by-zero diagnostic published; got:\n%s", out)
	}
}

// --- helpers ---

// writeBuggyWorkspace creates a temp module whose main.go divides by zero, and
// returns its root directory.
func writeBuggyWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module buggy\n\ngo 1.26\n")
	// Division by zero is only detected from a direct literal-0 divisor at the
	// division site (constant args through a call are not tracked through SSA),
	// so the zero is bound to a local used directly in the BinOp.
	writeFile(t, filepath.Join(root, "main.go"), `package main

func main() {
	x := 0
	y := 10 / x
	println(y)
}
`)
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// driveSession feeds framed input through a Server and returns everything it
// wrote. The server's working directory is restored afterward.
func driveSession(t *testing.T, cfg interpreter.Config, input string) string {
	return runSession(t, cfg, false, input)
}

func driveSessionNoCache(t *testing.T, cfg interpreter.Config, input string) string {
	return runSession(t, cfg, true, input)
}

func runSession(t *testing.T, cfg interpreter.Config, noCache bool, input string) string {
	t.Helper()
	// The server chdir's into the workspace root on initialize; restore cwd so
	// tests don't leak directory state into one another.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	var out bytes.Buffer
	srv := NewServer(cfg, noCache, func(string, ...interface{}) {})
	srv.Serve(strings.NewReader(input), &out)
	return out.String()
}

// request frames a JSON-RPC request with an integer id.
func request(t *testing.T, id int, method string, params interface{}) string {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	idRaw := json.RawMessage(fmt.Sprintf("%d", id))
	return frame(t, message{JSONRPC: "2.0", ID: &idRaw, Method: method, Params: raw})
}

// notification frames a JSON-RPC notification (no id).
func notification(t *testing.T, method string, params interface{}) string {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return frame(t, message{JSONRPC: "2.0", Method: method, Params: raw})
}

func frame(t *testing.T, m message) string {
	t.Helper()
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

// decodeMessages parses all framed messages from a server's output stream.
func decodeMessages(t *testing.T, s string) []message {
	t.Helper()
	var msgs []message
	c := &conn{r: bufio.NewReader(strings.NewReader(s)), w: io.Discard}
	for {
		m, err := c.readMessage()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode: %v", err)
		}
		msgs = append(msgs, *m)
	}
	return msgs
}

// remarshal re-encodes v and decodes it into dst, used to read a response's
// interface{} Result as a concrete type.
func remarshal(t *testing.T, v interface{}, dst interface{}) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatal(err)
	}
}

// rawParams returns a message's Params as an interface for remarshal.
func rawParams(t *testing.T, m message) interface{} {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal(m.Params, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
