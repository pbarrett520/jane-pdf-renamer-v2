package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testInfo() PatientInfo {
	d := dateOf(2025, time.December, 18)
	return PatientInfo{
		FirstName:       "Test",
		LastName:        "Patient",
		AppointmentDate: &d,
		Confidence:      1.0,
	}
}

func writeTempPDF(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenerateFilenameFormat(t *testing.T) {
	renamer := NewFileRenamer("", "")
	got, err := renamer.GenerateFilename(testInfo(), "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Patient, Test 121825 PT Note.pdf" {
		t.Errorf("got %q, want 'Patient, Test 121825 PT Note.pdf'", got)
	}
}

func TestGenerateFilenameWithDifferentFormats(t *testing.T) {
	renamer := NewFileRenamer("", "")
	info := testInfo()

	cases := []struct {
		format FileFormat
		want   string
	}{
		{ApptBilling, "Patient, Test 121825 PT Note.pdf"},
		{ApptBillingEval, "Patient, Test 121825 PT Eval Note.pdf"},
		{ApptBillingProgress, "Patient, Test 121825 PT Progress Note.pdf"},
		{ApptBillingDischarge, "Patient, Test 121825 PT Discharge Note.pdf"},
	}
	for _, c := range cases {
		got, err := renamer.GenerateFilename(info, c.format)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("format %s: got %q, want %q", c.format, got, c.want)
		}
	}

	// Current discharge uses today's date
	todayStr := time.Now().Format("010206")
	got, err := renamer.GenerateFilename(info, CurrentDischarge)
	if err != nil {
		t.Fatal(err)
	}
	want := "Patient, Test " + todayStr + " PT Chart Note.pdf"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGenerateFilenameWithDOICode(t *testing.T) {
	renamer := NewFileRenamer("", ApptBilling)
	d := dateOf(2025, time.December, 18)
	info := PatientInfo{
		FirstName: "Test", LastName: "Patient 1",
		AppointmentDate: &d, Confidence: 1.0, DateCode: "DOI 010125",
	}
	got, err := renamer.GenerateFilename(info, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Patient 1, Test DOI 010125 121825 PT Note.pdf" {
		t.Errorf("got %q, want 'Patient 1, Test DOI 010125 121825 PT Note.pdf'", got)
	}
}

func TestGenerateFilenameWithDOBCode(t *testing.T) {
	renamer := NewFileRenamer("", ApptBilling)
	d := dateOf(2025, time.December, 18)
	info := PatientInfo{
		FirstName: "Jane", LastName: "Doe 2",
		AppointmentDate: &d, Confidence: 1.0, DateCode: "DOB 031590",
	}
	got, err := renamer.GenerateFilename(info, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Doe 2, Jane DOB 031590 121825 PT Note.pdf" {
		t.Errorf("got %q, want 'Doe 2, Jane DOB 031590 121825 PT Note.pdf'", got)
	}
}

func TestGenerateFilenameWithFourDigitYearDOI(t *testing.T) {
	renamer := NewFileRenamer("", ApptBilling)
	d := dateOf(2026, time.April, 25)
	info := PatientInfo{
		FirstName: "Lihuan", LastName: "Zhang",
		AppointmentDate: &d, Confidence: 1.0, DateCode: "DOI 10272025",
	}
	got, err := renamer.GenerateFilename(info, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Zhang, Lihuan DOI 10272025 042526 PT Note.pdf" {
		t.Errorf("got %q, want 'Zhang, Lihuan DOI 10272025 042526 PT Note.pdf'", got)
	}
}

func TestGenerateFilenameWithSubtype(t *testing.T) {
	renamer := NewFileRenamer("", "")
	info := testInfo()
	info.NoteSubtype = "Ortho"
	got, err := renamer.GenerateFilename(info, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "Patient, Test 121825 PT Note Ortho.pdf" {
		t.Errorf("got %q, want 'Patient, Test 121825 PT Note Ortho.pdf'", got)
	}
}

func TestGenerateFilenameSanitizesInvalidChars(t *testing.T) {
	renamer := NewFileRenamer("", "")
	info := testInfo()
	info.LastName = `Pa/ti\ent:<>"|?*`
	got, err := renamer.GenerateFilename(info, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(got, `/\:<>"|?*`) {
		t.Errorf("filename %q contains invalid characters", got)
	}
}

func TestGenerateFilenameErrorsWithoutDate(t *testing.T) {
	renamer := NewFileRenamer("", "")
	info := PatientInfo{FirstName: "Test", LastName: "Patient", Confidence: 1.0}
	if _, err := renamer.GenerateFilename(info, ""); err == nil {
		t.Error("expected error when no date available")
	}
}

func TestRenameFileCreatesCorrectName(t *testing.T) {
	dir := t.TempDir()
	source := writeTempPDF(t, dir, "test_input.pdf", []byte("pdf content"))

	renamer := NewFileRenamer("", "")
	result, err := renamer.RenameFile(source, testInfo(), "")
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(dir, "Patient, Test 121825 PT Note.pdf")
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
	if !pathExists(want) {
		t.Error("target file does not exist")
	}
	if pathExists(source) {
		t.Error("source file still exists")
	}
}

func TestRenameFileToOutputFolder(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "Processed")
	source := writeTempPDF(t, dir, "test_input.pdf", []byte("pdf content"))

	renamer := NewFileRenamer(outputDir, "")
	result, err := renamer.RenameFile(source, testInfo(), "")
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(outputDir, "Patient, Test 121825 PT Note.pdf")
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
	if !pathExists(want) {
		t.Error("target file does not exist in output folder")
	}
}

func TestRenamePreventsOverwriteWithHash(t *testing.T) {
	dir := t.TempDir()
	writeTempPDF(t, dir, "Patient, Test 121825 PT Note.pdf", []byte("existing content"))
	source := writeTempPDF(t, dir, "test_input.pdf", []byte("different content"))

	renamer := NewFileRenamer("", "")
	result, err := renamer.RenameFile(source, testInfo(), "")
	if err != nil {
		t.Fatal(err)
	}

	base := filepath.Base(result)
	if !strings.HasPrefix(base, "Patient, Test 121825 PT Note_") || !strings.HasSuffix(base, ".pdf") {
		t.Errorf("expected hash-suffixed name, got %q", base)
	}
	if !pathExists(result) {
		t.Error("renamed file does not exist")
	}
	if !pathExists(filepath.Join(dir, "Patient, Test 121825 PT Note.pdf")) {
		t.Error("original target should still exist")
	}

	// Hash suffix should be short (6-8 chars)
	stem := strings.TrimSuffix(base, ".pdf")
	hashPart := stem[strings.LastIndex(stem, "_")+1:]
	if len(hashPart) < 6 || len(hashPart) > 8 {
		t.Errorf("hash suffix %q has length %d, want 6-8", hashPart, len(hashPart))
	}
}

func TestRenameIdenticalFileReplacesWithoutHash(t *testing.T) {
	dir := t.TempDir()
	content := []byte("identical pdf content")

	renamer := NewFileRenamer(dir, "")

	first := writeTempPDF(t, dir, "test_input.pdf", content)
	firstResult, err := renamer.RenameFile(first, testInfo(), "")
	if err != nil {
		t.Fatal(err)
	}

	second := writeTempPDF(t, dir, "source_copy.pdf", content)
	secondResult, err := renamer.RenameFile(second, testInfo(), "")
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(secondResult) != filepath.Base(firstResult) {
		t.Errorf("identical file got different name: %q vs %q",
			filepath.Base(secondResult), filepath.Base(firstResult))
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "Patient, Test *.pdf"))
	if len(matches) != 1 {
		t.Errorf("expected 1 patient file, found %d", len(matches))
	}
}

func TestParseFileFormat(t *testing.T) {
	if got := ParseFileFormat("appt_billing_eval"); got != ApptBillingEval {
		t.Errorf("got %q, want %q", got, ApptBillingEval)
	}
	if got := ParseFileFormat("bogus"); got != ApptBilling {
		t.Errorf("invalid format should default to appt_billing, got %q", got)
	}
}
