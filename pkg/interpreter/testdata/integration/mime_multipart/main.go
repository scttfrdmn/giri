// mime_multipart exercises mime and mime/multipart intercepts (#111).
//
// Expected: 0 violations.
package main

import (
	"bytes"
	"mime"
	"mime/multipart"
	"strings"
)

func useMime() {
	// TypeByExtension: lookup common extensions.
	_ = mime.TypeByExtension(".html")
	_ = mime.TypeByExtension(".json")
	_ = mime.TypeByExtension(".txt")
	_ = mime.TypeByExtension(".unknown")

	// AddExtensionType: register a custom MIME type.
	_ = mime.AddExtensionType(".myext", "application/x-myext")

	// FormatMediaType: serialize a media type.
	formatted := mime.FormatMediaType("text/plain", map[string]string{"charset": "utf-8"})
	_ = formatted

	// ParseMediaType: parse a media type header.
	mediatype, params, _ := mime.ParseMediaType("text/html; charset=utf-8")
	_ = mediatype
	_ = params
}

func useMultipart() {
	// Build a simple multipart body using a Writer.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	_ = mw.WriteField("username", "alice")
	_, _ = mw.CreateFormField("password")
	_ = mw.Close()

	boundary := mw.Boundary()
	contentType := mw.FormDataContentType()
	_ = boundary
	_ = contentType

	// Parse the multipart body using a Reader.
	mr := multipart.NewReader(strings.NewReader(body.String()), boundary)

	// Read parts.
	part, _ := mr.NextPart()
	_ = part
}

func main() {
	useMime()
	useMultipart()
}
