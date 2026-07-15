// Package lsp implements a minimal Language Server Protocol server that
// publishes Giri's undefined-behavior findings as editor diagnostics (#232).
//
// The server speaks LSP over stdio using JSON-RPC 2.0 with the standard
// Content-Length header framing. It implements only the handful of methods
// needed to surface diagnostics — initialize/initialized, the didOpen/didSave/
// didClose text-document notifications, and shutdown/exit — and reuses Giri's
// existing analysis pipeline (ssautil → interpreter → pkg/cache → pkg/report)
// so diagnostics match the CLI exactly. There is no external LSP/JSON-RPC
// dependency; the wire handling lives here in jsonrpc.go.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// message is the union of a JSON-RPC 2.0 request, response, and notification.
// Requests and notifications carry Method (+ Params); responses carry Result or
// Error. A notification is a request with no ID. We keep one struct for both
// directions to keep the transport tiny.
type message struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *responseError   `json:"error,omitempty"`
}

// responseError is a JSON-RPC 2.0 error object.
type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC 2.0 standard error codes (subset).
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInternalError  = -32603
)

// conn is a JSON-RPC 2.0 connection over a Content-Length-framed byte stream.
// Reads and writes are each serialized internally by the caller (the server
// runs a single read loop and writes responses/notifications from it or under a
// lock), so conn itself holds no locks.
type conn struct {
	r *bufio.Reader
	w io.Writer
}

func newConn(r io.Reader, w io.Writer) *conn {
	return &conn{r: bufio.NewReader(r), w: w}
}

// readMessage reads one framed JSON-RPC message. It returns io.EOF when the
// input stream is exhausted (a clean client disconnect).
func (c *conn) readMessage() (*message, error) {
	contentLength := -1
	// Read headers until the blank separator line.
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // end of headers
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue // tolerate malformed header lines
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q: %w", value, err)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return nil, err
	}
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, &parseError{err}
	}
	return &m, nil
}

// parseError signals a malformed JSON body so the read loop can respond with a
// JSON-RPC parse error rather than tearing down the connection.
type parseError struct{ err error }

func (e *parseError) Error() string { return e.err.Error() }

// write frames and writes a single JSON-RPC message.
func (c *conn) write(m *message) error {
	m.JSONRPC = "2.0"
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.w.Write(body)
	return err
}

// notify sends a notification (a request with no ID) with the given method and
// params.
func (c *conn) notify(method string, params interface{}) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return c.write(&message{Method: method, Params: raw})
}

// respond sends a successful response to the request identified by id.
func (c *conn) respond(id *json.RawMessage, result interface{}) error {
	return c.write(&message{ID: id, Result: result})
}

// respondError sends an error response to the request identified by id.
func (c *conn) respondError(id *json.RawMessage, code int, msg string) error {
	return c.write(&message{ID: id, Error: &responseError{Code: code, Message: msg}})
}
