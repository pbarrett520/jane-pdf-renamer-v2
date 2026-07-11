// PDF text extraction.
//
// Uses go-pdfium in WebAssembly mode (no CGO, cross-compiles cleanly).
// HIPAA compliant: never writes extracted text to disk or logs.
package core

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

var (
	pdfiumOnce     sync.Once
	pdfiumInstance pdfium.Pdfium
	pdfiumInitErr  error
	pdfiumMu       sync.Mutex // pdfium instance is not goroutine-safe
)

func getPdfium() (pdfium.Pdfium, error) {
	pdfiumOnce.Do(func() {
		pool, err := webassembly.Init(webassembly.Config{
			MinIdle:  1,
			MaxIdle:  1,
			MaxTotal: 1,
		})
		if err != nil {
			pdfiumInitErr = fmt.Errorf("initializing pdfium: %w", err)
			return
		}
		pdfiumInstance, pdfiumInitErr = pool.GetInstance(30 * time.Second)
	})
	return pdfiumInstance, pdfiumInitErr
}

// ExtractText extracts all text from a PDF file, with normalized whitespace.
// Text is returned in memory only; nothing is cached, logged, or written to disk.
func ExtractText(pdfPath string) (string, error) {
	if _, err := os.Stat(pdfPath); err != nil {
		return "", fmt.Errorf("PDF file not found: %s", pdfPath)
	}

	instance, err := getPdfium()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return "", err
	}

	pdfiumMu.Lock()
	defer pdfiumMu.Unlock()

	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return "", fmt.Errorf("cannot open PDF: %w", err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return "", fmt.Errorf("cannot read page count: %w", err)
	}

	var parts []string
	for i := 0; i < pageCount.PageCount; i++ {
		pageText, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i},
			},
		})
		if err != nil {
			return "", fmt.Errorf("cannot extract text from page %d: %w", i+1, err)
		}
		parts = append(parts, pageText.Text)
	}

	return normalizeWhitespace(strings.Join(parts, "\n")), nil
}

// normalizeWhitespace preserves line breaks, collapses runs of spaces, and
// strips leading/trailing whitespace from each line. PDFium emits \r\n line
// breaks; these are normalized to \n.
func normalizeWhitespace(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.Join(lines, "\n")
}
