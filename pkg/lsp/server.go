package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/scttfrdmn/giri/internal/ssautil"
	"github.com/scttfrdmn/giri/pkg/cache"
	"github.com/scttfrdmn/giri/pkg/interpreter"
	"github.com/scttfrdmn/giri/pkg/report"
)

// Server is a Giri LSP diagnostics server. It analyzes the workspace's main
// packages (the `giri ./...` equivalent) on file open and save, publishing
// findings as diagnostics. One Server serves one client over one connection.
type Server struct {
	config  interpreter.Config
	noCache bool
	// logf receives human-readable server notices (e.g. the GOEXPERIMENT hint,
	// analysis errors). It is separate from the LSP connection so it works
	// before initialize and in tests. Defaults to stderr.
	logf func(format string, args ...interface{})

	conn *conn

	mu sync.Mutex // guards the fields below and serializes analysis runs
	// root is the workspace directory analysis runs from (resolved at initialize).
	root string
	// published tracks absolute file paths we last sent non-empty diagnostics
	// for, so a subsequent run can clear (publish empty) files whose findings
	// were resolved.
	published map[string]bool
	// shuttingDown is set by the shutdown request; exit then terminates.
	shuttingDown bool
}

// NewServer creates a diagnostics server with the given analysis config. When
// noCache is true the on-disk result cache is bypassed. logf, if non-nil,
// receives server notices; otherwise notices go to stderr.
func NewServer(config interpreter.Config, noCache bool, logf func(string, ...interface{})) *Server {
	if logf == nil {
		logf = func(format string, args ...interface{}) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
	}
	return &Server{
		config:    config,
		noCache:   noCache,
		logf:      logf,
		published: make(map[string]bool),
	}
}

// Serve runs the LSP message loop over r/w until the client disconnects or
// sends exit. It returns a process exit code: 0 for a clean shutdown/exit
// sequence or EOF, 1 if exit arrived without a prior shutdown request (per LSP).
func (s *Server) Serve(r io.Reader, w io.Writer) int {
	s.conn = newConn(r, w)
	for {
		msg, err := s.conn.readMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0 // clean client disconnect
			}
			var pe *parseError
			if errors.As(err, &pe) {
				_ = s.conn.respondError(nil, codeParseError, pe.Error())
				continue
			}
			// Transport-level failure: nothing more we can do.
			s.logf("lsp: read error: %v", err)
			return 1
		}

		exit, code := s.handle(msg)
		if exit {
			return code
		}
	}
}

// handle dispatches a single incoming message. It returns exit=true when the
// server should terminate, carrying the process exit code.
func (s *Server) handle(msg *message) (exit bool, code int) {
	switch msg.Method {
	case "initialize":
		s.handleInitialize(msg)
	case "initialized":
		// Notification, no response. Nothing to do.
	case "textDocument/didOpen":
		var p DidOpenTextDocumentParams
		if s.unmarshalParams(msg, &p) {
			s.analyze()
		}
	case "textDocument/didSave":
		var p DidSaveTextDocumentParams
		if s.unmarshalParams(msg, &p) {
			s.analyze()
		}
	case "textDocument/didClose":
		// Diagnostics for a closed file remain valid; nothing to recompute.
	case "shutdown":
		s.mu.Lock()
		s.shuttingDown = true
		s.mu.Unlock()
		_ = s.conn.respond(msg.ID, nil)
	case "exit":
		s.mu.Lock()
		clean := s.shuttingDown
		s.mu.Unlock()
		if clean {
			return true, 0
		}
		return true, 1
	default:
		// Respond to unknown requests (those with an ID) with MethodNotFound;
		// ignore unknown notifications.
		if msg.ID != nil {
			_ = s.conn.respondError(msg.ID, codeMethodNotFound, "unsupported method: "+msg.Method)
		}
	}
	return false, 0
}

// unmarshalParams decodes a request's params into v, responding with an error
// for a request (has ID) on failure. It returns true on success.
func (s *Server) unmarshalParams(msg *message, v interface{}) bool {
	if len(msg.Params) == 0 {
		return true // no params is valid for our notifications
	}
	if err := json.Unmarshal(msg.Params, v); err != nil {
		if msg.ID != nil {
			_ = s.conn.respondError(msg.ID, codeInvalidRequest, "invalid params: "+err.Error())
		}
		s.logf("lsp: bad params for %s: %v", msg.Method, err)
		return false
	}
	return true
}

// handleInitialize processes the initialize request: resolves the workspace
// root, changes into it (analysis loads packages relative to the working
// directory), and advertises capabilities.
func (s *Server) handleInitialize(msg *message) {
	var p InitializeParams
	_ = s.unmarshalParams(msg, &p)

	root := resolveRoot(p)
	s.mu.Lock()
	s.root = root
	s.mu.Unlock()

	if root != "" {
		if err := os.Chdir(root); err != nil {
			s.logf("lsp: cannot enter workspace root %q: %v", root, err)
		}
	}

	// GOEXPERIMENT=arenas is required to load programs that import "arena".
	// We do not mutate the client's environment; surface a hint instead.
	if !arenasEnabled() {
		s.notifyLog(messageTypeWarning,
			"giri: GOEXPERIMENT=arenas is not set — arena programs will not be analyzed. "+
				"Launch the server with GOEXPERIMENT=arenas in its environment.")
	}

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: TextDocumentSyncOptions{
				OpenClose: true,
				Change:    syncFull,
				Save:      SaveOptions{IncludeText: false},
			},
		},
		ServerInfo: ServerInfo{Name: "giri", Version: report.Version},
	}
	_ = s.conn.respond(msg.ID, result)
}

// analyze runs Giri over the workspace's main packages and publishes
// diagnostics per file. Runs are serialized: on-save events never overlap.
func (s *Server) analyze() {
	s.mu.Lock()
	defer s.mu.Unlock()

	byFile, err := s.runAnalysis()
	if err != nil {
		// A load failure (e.g. a syntax error mid-edit) is expected and transient;
		// log it but keep prior diagnostics rather than clearing everything.
		s.logf("lsp: analysis failed: %v", err)
		return
	}

	// Publish diagnostics for every file that has findings.
	for path, diags := range byFile {
		s.publish(path, diags)
	}
	// Clear files that had findings last run but are now clean.
	for path := range s.published {
		if _, still := byFile[path]; !still {
			s.publish(path, nil)
		}
	}

	// Update the published set to exactly the files that now carry findings.
	s.published = make(map[string]bool, len(byFile))
	for path := range byFile {
		s.published[path] = true
	}
}

// runAnalysis loads and interprets the workspace's main packages, returning
// diagnostics grouped by absolute file path. It mirrors cmd/giri's main-programs
// path, including the deterministic cache protocol, so LSP and CLI results
// (and cache entries) are identical. Active findings only — suppressed
// (//giri:ignore) findings never become diagnostics.
func (s *Server) runAnalysis() (map[string][]Diagnostic, error) {
	progs, err := ssautil.LoadAllPrograms([]string{"./..."})
	if err != nil {
		return nil, err
	}

	// The cache is only sound for deterministic single-run analysis, exactly as
	// in cmd/giri. The LSP always runs single-run roundrobin (see NewServer's
	// config), so caching is enabled unless the user disabled it.
	deterministic := s.config.ScheduleStrategy == interpreter.ScheduleRoundRobin
	cacheDir, cacheOK := "", false
	if !s.noCache && deterministic {
		cacheDir, cacheOK = cache.Dir()
	}
	cfgFingerprint := cache.Fingerprint(s.config)

	byFile := make(map[string][]Diagnostic)
	add := func(findings []report.Finding) {
		for _, f := range findings {
			path, d, ok := findingToDiagnostic(f)
			if !ok {
				continue
			}
			byFile[path] = append(byFile[path], d)
		}
	}

	for _, prog := range progs {
		var key string
		if cacheOK && prog.SourceHash != "" {
			key = cache.Key(prog.SourceHash, cfgFingerprint, report.Version, prog.GoVersion)
			if entry, hit := cache.Load(cacheDir, key); hit {
				add(entry.Active) // suppressed findings intentionally skipped
				continue
			}
		}

		result := interpreter.Run(prog, s.config)
		active := report.FindingsFrom(result.Violations)
		add(active)

		if cacheOK && key != "" {
			entry := &cache.Entry{
				Active:          active,
				Suppressed:      report.FindingsFrom(result.SuppressedViolations),
				SuppressedCount: result.SuppressedCount,
				MemStats:        result.MemStats,
			}
			if storeErr := cache.Store(cacheDir, key, entry); storeErr != nil {
				s.logf("lsp: cache store failed: %v", storeErr)
			}
		}
	}
	return byFile, nil
}

// publish sends a textDocument/publishDiagnostics notification for one file.
// A nil/empty diagnostics slice clears the file's diagnostics.
func (s *Server) publish(path string, diags []Diagnostic) {
	if diags == nil {
		diags = []Diagnostic{} // LSP requires an array, not null, to clear
	}
	err := s.conn.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         pathToURI(path),
		Diagnostics: diags,
	})
	if err != nil {
		s.logf("lsp: publish failed for %s: %v", path, err)
	}
}

// notifyLog sends a window/logMessage notification, falling back to logf when
// the connection is not yet established.
func (s *Server) notifyLog(typ int, message string) {
	if s.conn == nil {
		s.logf("%s", message)
		return
	}
	_ = s.conn.notify("window/logMessage", LogMessageParams{Type: typ, Message: message})
}
