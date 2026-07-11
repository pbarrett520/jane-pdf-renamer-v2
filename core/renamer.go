// File renaming with naming convention and collision handling.
//
// HIPAA compliant: logs only file paths and hashes, never patient info.
package core

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileFormat selects the output filename format.
type FileFormat string

const (
	CurrentDischarge     FileFormat = "current_discharge"
	ApptBilling          FileFormat = "appt_billing"
	ApptBillingEval      FileFormat = "appt_billing_eval"
	ApptBillingProgress  FileFormat = "appt_billing_progress"
	ApptBillingDischarge FileFormat = "appt_billing_discharge"
)

type formatSpec struct {
	UseCurrentDate bool
	Suffix         string
}

var formatConfig = map[FileFormat]formatSpec{
	CurrentDischarge:     {true, "PT Chart Note"},
	ApptBilling:          {false, "PT Note"},
	ApptBillingEval:      {false, "PT Eval Note"},
	ApptBillingProgress:  {false, "PT Progress Note"},
	ApptBillingDischarge: {false, "PT Discharge Note"},
}

// ParseFileFormat returns the FileFormat for s, defaulting to ApptBilling.
func ParseFileFormat(s string) FileFormat {
	if _, ok := formatConfig[FileFormat(s)]; ok {
		return FileFormat(s)
	}
	return ApptBilling
}

// FileRenamer renames PDF files according to the naming convention:
//
//	Last, First MMDDYY <Suffix>.pdf
//
// If the target filename exists with different content, a short content hash
// is appended before .pdf. Identical files replace the existing copy.
type FileRenamer struct {
	OutputFolder string // empty = rename in place
	Format       FileFormat
}

func NewFileRenamer(outputFolder string, format FileFormat) FileRenamer {
	if format == "" {
		format = ApptBilling
	}
	return FileRenamer{OutputFolder: outputFolder, Format: format}
}

// GenerateFilename builds the target filename from patient info.
// formatOverride, if non-empty, takes precedence over the instance format.
func (r FileRenamer) GenerateFilename(info PatientInfo, formatOverride FileFormat) (string, error) {
	format := r.Format
	if formatOverride != "" {
		format = formatOverride
	}
	spec, ok := formatConfig[format]
	if !ok {
		spec = formatConfig[ApptBilling]
	}

	// When DOI/DOB is present, include BOTH the code AND the appropriate date
	var dateStr string
	if info.DateCode != "" {
		datePart := ""
		if spec.UseCurrentDate {
			datePart = time.Now().Format("010206")
		} else if info.AppointmentDate != nil {
			datePart = info.AppointmentDate.Format("010206")
		}
		if datePart != "" {
			dateStr = info.DateCode + " " + datePart
		} else {
			dateStr = info.DateCode
		}
	} else if spec.UseCurrentDate {
		dateStr = time.Now().Format("010206")
	} else if info.AppointmentDate != nil {
		dateStr = info.AppointmentDate.Format("010206")
	} else {
		return "", fmt.Errorf("cannot generate filename without appointment date or date code")
	}

	subtype := ""
	if info.NoteSubtype != "" {
		subtype = " " + info.NoteSubtype
	}
	filename := fmt.Sprintf("%s, %s %s %s%s.pdf", info.LastName, info.FirstName, dateStr, spec.Suffix, subtype)

	return sanitizeFilename(filename), nil
}

// sanitizeFilename removes characters that are invalid in filenames across
// platforms (path separators, plus < > : " | ? * which are invalid on Windows).
func sanitizeFilename(filename string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '<', '>', '"', '|', '?', '*':
			return -1
		}
		return r
	}, filename)
}

// RenameFile renames a PDF according to the naming convention and returns the
// final path. Files move to OutputFolder if set, otherwise rename in place.
func (r FileRenamer) RenameFile(sourcePath string, info PatientInfo, formatOverride FileFormat) (string, error) {
	if _, err := os.Stat(sourcePath); err != nil {
		return "", fmt.Errorf("source file not found: %s", sourcePath)
	}

	filename, err := r.GenerateFilename(info, formatOverride)
	if err != nil {
		return "", err
	}

	targetDir := r.OutputFolder
	if targetDir == "" {
		targetDir = filepath.Dir(sourcePath)
	} else if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create output folder: %w", err)
	}

	targetPath := filepath.Join(targetDir, filename)

	if pathExists(targetPath) && !samePath(targetPath, sourcePath) {
		identical, err := filesAreIdentical(sourcePath, targetPath)
		if err == nil && identical {
			log.Printf("Target file identical to source, replacing: %s", filepath.Base(targetPath))
			if err := os.Remove(targetPath); err != nil {
				return "", fmt.Errorf("cannot replace existing file: %w", err)
			}
		} else {
			log.Printf("Collision detected - different file exists: %s", filepath.Base(targetPath))
			targetPath, err = handleCollision(sourcePath, targetPath)
			if err != nil {
				return "", err
			}
		}
	}

	// Log operation (file path and hash only, no PHI)
	fileHash, err := computeShortHash(sourcePath)
	if err != nil {
		return "", err
	}
	log.Printf("Renaming file: hash=%s, target=%s", fileHash, filepath.Base(targetPath))

	if err := moveFile(sourcePath, targetPath); err != nil {
		return "", err
	}

	log.Printf("Rename complete: hash=%s, success=true", fileHash)
	return targetPath, nil
}

// handleCollision appends a short content hash before the .pdf extension.
func handleCollision(sourcePath, targetPath string) (string, error) {
	shortHash, err := computeShortHash(sourcePath)
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(targetPath)
	stem := strings.TrimSuffix(filepath.Base(targetPath), filepath.Ext(targetPath))

	newTarget := filepath.Join(dir, fmt.Sprintf("%s_%s.pdf", stem, shortHash))
	for counter := 1; pathExists(newTarget); counter++ {
		newTarget = filepath.Join(dir, fmt.Sprintf("%s_%s_%d.pdf", stem, shortHash, counter))
	}
	return newTarget, nil
}

func filesAreIdentical(path1, path2 string) (bool, error) {
	hash1, err := computeFullHash(path1)
	if err != nil {
		return false, err
	}
	hash2, err := computeFullHash(path2)
	if err != nil {
		return false, err
	}
	return hash1 == hash2, nil
}

func computeFullHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := md5.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func computeShortHash(path string) (string, error) {
	full, err := computeFullHash(path)
	if err != nil {
		return "", err
	}
	return full[:6], nil
}

// moveFile renames, falling back to copy+delete for cross-device moves
// (e.g., temp dir on a different filesystem than the output folder).
func moveFile(source, target string) error {
	if err := os.Rename(source, target); err == nil {
		return nil
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(target)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	in.Close()
	return os.Remove(source)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return absA == absB
}
