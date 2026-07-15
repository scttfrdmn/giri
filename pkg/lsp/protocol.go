package lsp

// This file defines the subset of Language Server Protocol types that Giri's
// diagnostics server touches. Field names and JSON tags follow the LSP 3.17
// specification. Only what we read or emit is modeled; everything else in an
// incoming payload is ignored by encoding/json.

// InitializeParams is the client's initialize request payload. We only need the
// workspace root, which the client may send as rootUri (preferred) or the
// deprecated rootPath.
type InitializeParams struct {
	RootURI          string            `json:"rootUri,omitempty"`
	RootPath         string            `json:"rootPath,omitempty"`
	WorkspaceFolders []WorkspaceFolder `json:"workspaceFolders,omitempty"`
}

// WorkspaceFolder is one entry of the multi-root workspace list.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// InitializeResult is the server's response to initialize, advertising the
// capabilities we support.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

// ServerInfo identifies the server in the initialize result.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities declares which LSP features the server implements. Giri
// only needs open/save text-document sync to trigger analysis; it does not
// consume incremental edits.
type ServerCapabilities struct {
	TextDocumentSync TextDocumentSyncOptions `json:"textDocumentSync"`
}

// TextDocumentSyncOptions requests open/close notifications and save events.
// Change=1 (Full) is advertised but changes are not analyzed — analysis runs on
// open and save only.
type TextDocumentSyncOptions struct {
	OpenClose bool        `json:"openClose"`
	Change    int         `json:"change"`
	Save      SaveOptions `json:"save"`
}

// SaveOptions configures didSave notifications. We do not need the document text
// on save (IncludeText=false); Giri re-reads files from disk.
type SaveOptions struct {
	IncludeText bool `json:"includeText"`
}

// TextDocumentSyncKind values (LSP spec).
const (
	syncNone        = 0
	syncFull        = 1
	syncIncremental = 2
)

// TextDocumentIdentifier identifies a document by URI.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentItem is the document payload carried by didOpen.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// DidOpenTextDocumentParams is the textDocument/didOpen payload.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidSaveTextDocumentParams is the textDocument/didSave payload.
type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DidCloseTextDocumentParams is the textDocument/didClose payload.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// PublishDiagnosticsParams is the server→client textDocument/publishDiagnostics
// notification payload.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Diagnostic is one editor squiggle. Range is 0-based per the LSP spec.
type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity"`
	Code     string             `json:"code,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

// Range is an inclusive-start, exclusive-end span of positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position is a 0-based (line, character) location.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// DiagnosticSeverity mirrors the LSP severity enum.
type DiagnosticSeverity int

const (
	severityError       DiagnosticSeverity = 1
	severityWarning     DiagnosticSeverity = 2
	severityInformation DiagnosticSeverity = 3
	severityHint        DiagnosticSeverity = 4
)

// LogMessageParams is the window/logMessage notification payload, used to
// surface server-side notices (e.g. the GOEXPERIMENT=arenas hint) in the
// client's log.
type LogMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

// MessageType values for window/logMessage (LSP spec).
const (
	messageTypeError   = 1
	messageTypeWarning = 2
	messageTypeInfo    = 3
	messageTypeLog     = 4
)
