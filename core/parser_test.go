package core

import (
	"testing"
	"time"
)

func dateOf(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestParseStripsTrailingNumberFromName(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\nTest Patient 1\nDecember 18, 2025", "")
	if info.FirstName != "Test" || info.LastName != "Patient" {
		t.Errorf("got %q %q, want Test Patient", info.FirstName, info.LastName)
	}
}

func TestParseHandlesMultiWordFirstName(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\nMary Jane Watson 2\nJanuary 1, 2024", "")
	if info.FirstName != "Mary Jane" || info.LastName != "Watson" {
		t.Errorf("got %q %q, want 'Mary Jane' 'Watson'", info.FirstName, info.LastName)
	}
}

func TestParseHandlesEmptyLinesAfterChart(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\n\n\nTest Patient 1\nDecember 18, 2025", "")
	if info.FirstName != "Test" || info.LastName != "Patient" {
		t.Errorf("got %q %q, want Test Patient", info.FirstName, info.LastName)
	}
}

func TestParseExtractsDate(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\nTest Patient 1\nDecember 18, 2025", "")
	want := dateOf(2025, time.December, 18)
	if info.AppointmentDate == nil || !info.AppointmentDate.Equal(want) {
		t.Errorf("got %v, want %v", info.AppointmentDate, want)
	}
}

func TestParseConfidenceHighWhenAllFieldsFound(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\nTest Patient 1\nDecember 18, 2025", "")
	if info.Confidence < 0.9 {
		t.Errorf("confidence = %v, want >= 0.9", info.Confidence)
	}
}

func TestParseConfidenceLowWhenNameMissing(t *testing.T) {
	info := PatientInfoParser{}.Parse("Some random text without patient info\nDecember 18, 2025", "")
	if info.Confidence > 0.5 {
		t.Errorf("confidence = %v, want <= 0.5", info.Confidence)
	}
}

func TestParseConfidenceLowWhenDateMissing(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\nTest Patient 1\nNo date here", "")
	if info.Confidence > 0.5 {
		t.Errorf("confidence = %v, want <= 0.5", info.Confidence)
	}
}

func TestParseVariousDateFormats(t *testing.T) {
	cases := []struct {
		text string
		want time.Time
	}{
		{"Chart\nJohn Doe 1\nJanuary 5, 2024", dateOf(2024, time.January, 5)},
		{"Chart\nJohn Doe 1\nFebruary 28, 2024", dateOf(2024, time.February, 28)},
		{"Chart\nJohn Doe 1\nMarch 15, 2024", dateOf(2024, time.March, 15)},
		{"Chart\nJohn Doe 1\nApril 1, 2024", dateOf(2024, time.April, 1)},
		{"Chart\nJohn Doe 1\nMay 20, 2024", dateOf(2024, time.May, 20)},
		{"Chart\nJohn Doe 1\nJune 30, 2024", dateOf(2024, time.June, 30)},
		{"Chart\nJohn Doe 1\nJuly 4, 2024", dateOf(2024, time.July, 4)},
		{"Chart\nJohn Doe 1\nAugust 15, 2024", dateOf(2024, time.August, 15)},
		{"Chart\nJohn Doe 1\nSeptember 21, 2024", dateOf(2024, time.September, 21)},
		{"Chart\nJohn Doe 1\nOctober 31, 2024", dateOf(2024, time.October, 31)},
		{"Chart\nJohn Doe 1\nNovember 11, 2024", dateOf(2024, time.November, 11)},
		{"Chart\nJohn Doe 1\nDecember 25, 2024", dateOf(2024, time.December, 25)},
	}
	for _, c := range cases {
		info := PatientInfoParser{}.Parse(c.text, "")
		if info.AppointmentDate == nil || !info.AppointmentDate.Equal(c.want) {
			t.Errorf("for %q: got %v, want %v", c.text, info.AppointmentDate, c.want)
		}
	}
}

func TestParseDOIInPatientName(t *testing.T) {
	info := PatientInfoParser{}.Parse(
		"Chart\nTest Patient 1 (DOI:010125)\nDecember 18, 2025",
		"HealthStre_Chart_1_TP_20251218_88209-2.pdf",
	)
	if info.DateCode != "DOI 010125" {
		t.Errorf("date code = %q, want 'DOI 010125'", info.DateCode)
	}
	if info.FirstName != "Test" || info.LastName != "Patient 1" {
		t.Errorf("got %q %q, want 'Test' 'Patient 1'", info.FirstName, info.LastName)
	}
	if !info.IsComplete() {
		t.Error("expected IsComplete() = true")
	}
}

func TestParseDOBInPatientName(t *testing.T) {
	info := PatientInfoParser{}.Parse(
		"Chart\nTest Patient 1 (DOB:031590)\nDecember 18, 2025",
		"HealthStre_Chart_1_TP_20251218_88209-2.pdf",
	)
	if info.DateCode != "DOB 031590" {
		t.Errorf("date code = %q, want 'DOB 031590'", info.DateCode)
	}
	if info.FirstName != "Test" || info.LastName != "Patient 1" {
		t.Errorf("got %q %q, want 'Test' 'Patient 1'", info.FirstName, info.LastName)
	}
}

func TestParseDOIWithSpace(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\nTest Patient 1 (DOI: 010125)\nDecember 18, 2025", "")
	if info.DateCode != "DOI 010125" {
		t.Errorf("date code = %q, want 'DOI 010125'", info.DateCode)
	}
}

func TestParseDOICaseInsensitive(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\nTest Patient 1 (doi:010125)\nDecember 18, 2025", "")
	if info.DateCode != "DOI 010125" {
		t.Errorf("date code = %q, want 'DOI 010125' (normalized to uppercase)", info.DateCode)
	}
}

func TestParseDOIFourDigitYear(t *testing.T) {
	info := PatientInfoParser{}.Parse(
		"Chart\nLihuan Zhang (DOI 10/27/2025)\nApril 25, 2026",
		"HealthStre_Chart_4168_LZ_20260425_12345.pdf",
	)
	if info.DateCode != "DOI 10272025" {
		t.Errorf("date code = %q, want 'DOI 10272025'", info.DateCode)
	}
	if info.FirstName != "Lihuan" || info.LastName != "Zhang" {
		t.Errorf("got %q %q, want 'Lihuan' 'Zhang'", info.FirstName, info.LastName)
	}
	want := dateOf(2026, time.April, 25)
	if info.AppointmentDate == nil || !info.AppointmentDate.Equal(want) {
		t.Errorf("date = %v, want %v", info.AppointmentDate, want)
	}
}

func TestParseNoDOIStripsTrailingNumber(t *testing.T) {
	info := PatientInfoParser{}.Parse("Chart\nTest Patient 1\nDecember 18, 2025", "")
	if info.DateCode != "" {
		t.Errorf("date code = %q, want empty", info.DateCode)
	}
	if info.FirstName != "Test" || info.LastName != "Patient" {
		t.Errorf("got %q %q, want Test Patient", info.FirstName, info.LastName)
	}
}

func TestParseNoteSubtype(t *testing.T) {
	cases := []struct {
		heading string
		want    string
	}{
		{"Follow Up - Ortho", "Ortho"},
		{"Follow Up Vestibular", "Vestibular"},
		{"Follow Up", ""},
		{"Initial Assessment - Saucedo Rx", "Saucedo Rx"},
	}
	for _, c := range cases {
		text := "Chart\nTest Patient 1\nAdded by: Someone\n" + c.heading + "\nDecember 18, 2025"
		info := PatientInfoParser{}.Parse(text, "")
		if info.NoteSubtype != c.want {
			t.Errorf("for heading %q: subtype = %q, want %q", c.heading, info.NoteSubtype, c.want)
		}
	}
}

func TestExtractInitialsFromFilename(t *testing.T) {
	if got := ExtractInitialsFromFilename("HealthStre_Chart_1_TP_20251218_88209-2.pdf"); got != "TP" {
		t.Errorf("got %q, want TP", got)
	}
	if got := ExtractInitialsFromFilename("random.pdf"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestParseInitialsSplitCompoundLastName(t *testing.T) {
	// Filename ..._AN_... + "Anna Nogales Ramirez" -> First: Anna, Last: Nogales Ramirez
	info := PatientInfoParser{}.Parse(
		"Chart\nAnna Nogales Ramirez\nDecember 18, 2025",
		"HealthStre_Chart_1_AN_20251218_88209-2.pdf",
	)
	if info.FirstName != "Anna" || info.LastName != "Nogales Ramirez" {
		t.Errorf("got %q %q, want 'Anna' 'Nogales Ramirez'", info.FirstName, info.LastName)
	}

	// Filename ..._TN_... + "Tony Chan Nguyen" -> First: Tony Chan, Last: Nguyen
	info = PatientInfoParser{}.Parse(
		"Chart\nTony Chan Nguyen\nDecember 18, 2025",
		"HealthStre_Chart_1_TN_20251218_88209-2.pdf",
	)
	if info.FirstName != "Tony Chan" || info.LastName != "Nguyen" {
		t.Errorf("got %q %q, want 'Tony Chan' 'Nguyen'", info.FirstName, info.LastName)
	}
}
