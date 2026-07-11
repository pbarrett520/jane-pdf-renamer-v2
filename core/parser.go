// Patient information parser.
//
// Parses patient name and appointment date from PDF text.
// HIPAA compliant: no logging of patient information.
package core

import (
	"regexp"
	"strings"
	"time"
)

var monthMap = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June,
	"july": time.July, "august": time.August, "september": time.September,
	"october": time.October, "november": time.November, "december": time.December,
}

var (
	// Date pattern: MonthName DD, YYYY
	datePattern = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2}),\s*(\d{4})\b`)

	// Trailing number in patient name (e.g., "Test Patient 1")
	trailingNumberPattern = regexp.MustCompile(`\s+\d+$`)

	// Initials from Jane filename: HealthStre_Chart_1_XX_20251218_88209-2.pdf
	initialsPattern = regexp.MustCompile(`_([A-Z]{2})_\d{8}_`)

	// DOI (Date of Injury) / DOB (Date of Birth) in patient name.
	// Handles (DOI:MMDDYY), (DOB: MMDDYY), (DOB: 01/02/25), (DOI 10/27/2025)
	doiDobPattern = regexp.MustCompile(`(?i)\s*\((DOI|DOB):?\s*(\d{2})[/\-]?(\d{2})[/\-]?(\d{2,4})\)\s*$`)

	// Base note type heading with optional sub-type after it
	noteTypePattern = regexp.MustCompile(`(?i)^(Follow Up|Discharge Visit|Initial Assessment|Progress Note)(.*)`)
)

// PatientInfo holds parsed patient information from a PDF.
type PatientInfo struct {
	FirstName       string
	LastName        string
	AppointmentDate *time.Time
	Confidence      float64
	DateCode        string // e.g., "DOI 010125" or "DOB 010225"; empty if none
	NoteSubtype     string // e.g., "Ortho", "Vestibular", "Saucedo Rx"; empty if none
}

// IsComplete reports whether all required fields are present.
// Either AppointmentDate or DateCode satisfies the date requirement.
func (p PatientInfo) IsComplete() bool {
	hasDate := p.AppointmentDate != nil || p.DateCode != ""
	return p.FirstName != "" && p.LastName != "" && hasDate
}

// NeedsReview reports whether manual review is needed.
func (p PatientInfo) NeedsReview() bool {
	return p.Confidence < 0.9 || !p.IsComplete()
}

// ExtractInitialsFromFilename extracts patient initials from a Jane PDF filename.
// Example: "HealthStre_Chart_1_TP_20251218_88209-2.pdf" -> "TP"
func ExtractInitialsFromFilename(filename string) string {
	m := initialsPattern.FindStringSubmatch(filename)
	if m != nil {
		return m[1]
	}
	return ""
}

// PatientInfoParser parses patient name and date from extracted PDF text.
//
// Parsing rules:
//  1. Find line that equals "Chart"
//  2. Next non-empty line is patient display name
//  3. Strip trailing number from name (e.g., "1" from "Test Patient 1")
//  4. Use initials from filename to determine correct first/last split
//  5. Find first occurrence of "MonthName DD, YYYY" pattern
type PatientInfoParser struct{}

// Parse extracts patient information from PDF text. filename (optional, may
// be empty) is the original PDF filename used to extract an initials hint.
func (p PatientInfoParser) Parse(text, filename string) PatientInfo {
	initials := ""
	if filename != "" {
		initials = ExtractInitialsFromFilename(filename)
	}

	firstName, lastName, nameFound, dateCode := parsePatientName(text, initials)
	appointmentDate, dateFound := parseAppointmentDate(text)
	noteSubtype := parseNoteSubtype(text)

	hasDate := dateFound || dateCode != ""
	confidence := calculateConfidence(nameFound, hasDate)

	return PatientInfo{
		FirstName:       firstName,
		LastName:        lastName,
		AppointmentDate: appointmentDate,
		Confidence:      confidence,
		DateCode:        dateCode,
		NoteSubtype:     noteSubtype,
	}
}

func parsePatientName(text, initials string) (first, last string, found bool, dateCode string) {
	lines := strings.Split(text, "\n")

	chartIndex := -1
	for i, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "chart") {
			chartIndex = i
			break
		}
	}
	if chartIndex == -1 {
		return "", "", false, ""
	}

	nameLine := ""
	for _, line := range lines[chartIndex+1:] {
		if stripped := strings.TrimSpace(line); stripped != "" {
			nameLine = stripped
			break
		}
	}
	if nameLine == "" {
		return "", "", false, ""
	}

	var name string
	if m := doiDobPattern.FindStringSubmatch(nameLine); m != nil {
		codeType := strings.ToUpper(m[1]) // "DOI" or "DOB"
		dateCode = codeType + " " + m[2] + m[3] + m[4]
		// When DOI/DOB is present, DON'T strip trailing numbers
		// (they're part of the compound name like "Patient 1")
		name = strings.TrimSpace(doiDobPattern.ReplaceAllString(nameLine, ""))
	} else {
		name = strings.TrimSpace(trailingNumberPattern.ReplaceAllString(nameLine, ""))
	}

	parts := strings.Fields(name)

	if len(parts) < 2 {
		if len(parts) == 1 {
			return "", parts[0], true, dateCode
		}
		return "", "", true, dateCode
	}

	// If we have initials, use them to find the correct split point
	if len(initials) == 2 {
		firstInitial := strings.ToUpper(initials[0:1])
		lastInitial := strings.ToUpper(initials[1:2])

		for splitIdx := 1; splitIdx < len(parts); splitIdx++ {
			potentialFirst := strings.Join(parts[:splitIdx], " ")
			potentialLast := strings.Join(parts[splitIdx:], " ")

			if strings.EqualFold(potentialFirst[0:1], firstInitial) &&
				strings.EqualFold(potentialLast[0:1], lastInitial) {
				return potentialFirst, potentialLast, true, dateCode
			}
		}
		// No match found - fall through to default behavior
	}

	// Default: last word = last name, everything before = first name.
	// BUT if DOI/DOB is present and last part is a number, keep it with the
	// previous word (e.g., "Test Patient 1" -> first="Test", last="Patient 1")
	if dateCode != "" && len(parts) >= 3 && isAllDigits(parts[len(parts)-1]) {
		last = strings.Join(parts[len(parts)-2:], " ")
		first = strings.Join(parts[:len(parts)-2], " ")
	} else {
		last = parts[len(parts)-1]
		first = strings.Join(parts[:len(parts)-1], " ")
	}

	return first, last, true, dateCode
}

// parseNoteSubtype extracts the note sub-type from the heading line that
// follows "Added by:".
//
//	"Follow Up - Ortho"    -> "Ortho"
//	"Follow Up Vestibular" -> "Vestibular"
//	"Follow Up"            -> ""
func parseNoteSubtype(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.Contains(strings.ToLower(line), "added by") {
			continue
		}
		for _, subsequent := range lines[i+1:] {
			stripped := strings.TrimSpace(subsequent)
			if stripped == "" {
				continue
			}
			if m := noteTypePattern.FindStringSubmatch(stripped); m != nil {
				remainder := strings.TrimSpace(m[2])
				remainder = strings.TrimSpace(strings.TrimLeft(remainder, "-–"))
				return remainder
			}
			return ""
		}
	}
	return ""
}

func parseAppointmentDate(text string) (*time.Time, bool) {
	m := datePattern.FindStringSubmatch(text)
	if m == nil {
		return nil, false
	}

	month, ok := monthMap[strings.ToLower(m[1])]
	if !ok {
		return nil, false
	}
	day := atoiSafe(m[2])
	year := atoiSafe(m[3])

	d := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	// time.Date normalizes invalid dates (e.g., Feb 30 -> Mar 2); reject those
	if d.Year() != year || d.Month() != month || d.Day() != day {
		return nil, false
	}
	return &d, true
}

func calculateConfidence(nameFound, dateFound bool) float64 {
	switch {
	case nameFound && dateFound:
		return 1.0
	case nameFound || dateFound:
		return 0.5
	default:
		return 0.0
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}
