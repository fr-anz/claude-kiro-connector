package reqconv

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/fr-anz/claudecode-kiro/internal/anthropic"
	"github.com/ledongthuc/pdf"
)

// maxDocumentTextRunes caps extracted document text so one attachment cannot
// silently consume the whole context window.
const maxDocumentTextRunes = 400_000

// pdfTextCache memoizes extraction by content hash: Claude Code resends every
// attachment on each turn, and re-parsing large PDFs per request is wasteful.
// The cache is cleared wholesale when full to keep memory bounded.
var pdfTextCache = struct {
	sync.Mutex
	m map[[32]byte]string
}{m: make(map[[32]byte]string)}

const pdfTextCacheMax = 32

// Kiro's conversation schema has no document field, so document blocks are
// converted to text the model can read. PDFs are text-extracted locally;
// scanned (image-only) PDFs yield no text and are reported as such.
func documentText(b anthropic.ContentBlock) string {
	var body string
	if b.Source != nil {
		switch b.Source.Type {
		case "text":
			body = b.Source.Data
		case "content":
			body = ExtractTextContent(b.Source.Content)
		case "base64":
			if b.Source.MediaType == "text/plain" {
				raw, err := base64.StdEncoding.DecodeString(b.Source.Data)
				if err == nil {
					body = string(raw)
				}
			} else {
				body = extractPDFText(b.Source.Data)
			}
		case "url":
			body = "(document URL not fetched by gateway: " + b.Source.URL + ")"
		}
	}
	if strings.TrimSpace(body) == "" {
		body = "(no extractable text; the document may be scanned images)"
	}
	if r := []rune(body); len(r) > maxDocumentTextRunes {
		body = string(r[:maxDocumentTextRunes]) + "\n[document truncated]"
	}

	var sb strings.Builder
	sb.WriteString("<document")
	if b.Title != "" {
		fmt.Fprintf(&sb, " title=%q", b.Title)
	}
	sb.WriteString(">\n")
	if b.Context != "" {
		sb.WriteString(b.Context)
		sb.WriteString("\n\n")
	}
	sb.WriteString(body)
	sb.WriteString("\n</document>")
	return sb.String()
}

// extractPDFText decodes a base64 PDF and returns its plain text, or "" when
// the PDF is unreadable. The parser is guarded against panics on malformed input.
func extractPDFText(data string) string {
	key := sha256.Sum256([]byte(data))
	pdfTextCache.Lock()
	text, ok := pdfTextCache.m[key]
	pdfTextCache.Unlock()
	if ok {
		return text
	}
	text = parsePDFText(data)
	pdfTextCache.Lock()
	if len(pdfTextCache.m) >= pdfTextCacheMax {
		clear(pdfTextCache.m)
	}
	pdfTextCache.m[key] = text
	pdfTextCache.Unlock()
	return text
}

func parsePDFText(data string) (text string) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		slog.Warn("document: invalid base64 PDF", "err", err)
		return ""
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("document: PDF parser panic", "panic", r)
			text = ""
		}
	}()
	reader, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		slog.Warn("document: unreadable PDF", "err", err)
		return ""
	}
	plain, err := reader.GetPlainText()
	if err != nil {
		slog.Warn("document: PDF text extraction failed", "err", err)
		return ""
	}
	out, err := io.ReadAll(io.LimitReader(plain, maxDocumentTextRunes*4))
	if err != nil {
		return ""
	}
	return string(out)
}
