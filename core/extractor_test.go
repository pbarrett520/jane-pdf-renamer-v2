package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Real Jane exports live in pdf_examples/ at the repo root (gitignored — PHI).
// These tests run against them when present and skip otherwise, asserting
// structure rather than patient names.
func examplePDFs(t *testing.T) []string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join("..", "pdf_examples", "*.pdf"))
	if len(matches) == 0 {
		t.Skip("no example PDFs available (pdf_examples/ is empty or missing)")
	}
	return matches
}

func TestExtractTextReturnsText(t *testing.T) {
	for _, pdf := range examplePDFs(t) {
		text, err := ExtractText(pdf)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(pdf), err)
		}
		if len(text) == 0 {
			t.Errorf("%s: extracted text is empty", filepath.Base(pdf))
		}
	}
}

func TestExtractTextContainsChartKeyword(t *testing.T) {
	for _, pdf := range examplePDFs(t) {
		text, err := ExtractText(pdf)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(text, "Chart") {
			t.Errorf("%s: extracted text missing 'Chart' keyword", filepath.Base(pdf))
		}
	}
}

func TestExtractedTextParsesWithFullConfidence(t *testing.T) {
	for _, pdf := range examplePDFs(t) {
		text, err := ExtractText(pdf)
		if err != nil {
			t.Fatal(err)
		}
		info := PatientInfoParser{}.Parse(text, filepath.Base(pdf))
		if info.Confidence < 1.0 {
			t.Errorf("%s: confidence = %v, want 1.0", filepath.Base(pdf), info.Confidence)
		}
		if !info.IsComplete() {
			t.Errorf("%s: parsed info incomplete", filepath.Base(pdf))
		}
	}
}

func TestExtractTextNonexistentFileErrors(t *testing.T) {
	if _, err := ExtractText("/nonexistent/file.pdf"); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestExtractTextDoesNotWriteToDisk(t *testing.T) {
	pdfs := examplePDFs(t)

	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	pdfAbs, err := filepath.Abs(pdfs[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	if _, err := ExtractText(pdfAbs); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("extraction wrote %d file(s) to disk", len(entries))
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	got := normalizeWhitespace("  hello   world \r\nfoo\rbar\n  spaced  out  ")
	want := "hello world\nfoo\nbar\nspaced out"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
