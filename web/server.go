// Web server for the browser-based GUI with drag-and-drop and format selection.
package web

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pbarrett520/jane-pdf-renamer-v2/core"
)

//go:embed assets
var assets embed.FS

var indexTemplate = template.Must(template.ParseFS(assets, "assets/index.html"))

// UploadDir is where uploaded files are staged and the default Processed
// folder lives.
var UploadDir = filepath.Join(os.TempDir(), "jane-pdf-renamer")

// ProcessResult mirrors the JSON contract the frontend expects.
type ProcessResult struct {
	Success      bool    `json:"success"`
	OriginalName string  `json:"original_name"`
	NewName      string  `json:"new_name,omitempty"`
	NewPath      string  `json:"new_path,omitempty"`
	Error        string  `json:"error,omitempty"`
	NeedsReview  bool    `json:"needs_review"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	DateStr      string  `json:"date_str"`
	Confidence   float64 `json:"confidence"`
}

// ResolveOutputPath resolves a user-provided output folder into a usable
// local filesystem path.
//
// Rules:
//   - Empty value -> default temp Processed folder
//   - Expand "~"
//   - Relative paths are rooted at the user's home directory to avoid
//     writing into the app's working directory, which may be read-only.
func ResolveOutputPath(outputFolder string) string {
	if outputFolder == "" {
		return filepath.Join(UploadDir, "Processed")
	}

	expanded := outputFolder
	if strings.HasPrefix(expanded, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			rest := strings.TrimPrefix(expanded, "~")
			rest = strings.TrimLeft(rest, `/\`)
			expanded = filepath.Join(home, rest)
		}
	}
	if !filepath.IsAbs(expanded) {
		if home, err := os.UserHomeDir(); err == nil {
			expanded = filepath.Join(home, expanded)
		}
	}
	return expanded
}

// ProcessPDF extracts, parses, and renames a single PDF.
// originalFilename (if non-empty) is used for the initials hint; the staged
// temp file has a UUID prefix that would break initials extraction.
func ProcessPDF(filePath string, format core.FileFormat, outputFolder, originalFilename string) ProcessResult {
	baseName := filepath.Base(filePath)
	filenameForParsing := originalFilename
	if filenameForParsing == "" {
		filenameForParsing = baseName
	}

	text, err := core.ExtractText(filePath)
	if err != nil {
		log.Printf("Error processing file: %v", err)
		return ProcessResult{Success: false, OriginalName: baseName, Error: err.Error()}
	}

	info := core.PatientInfoParser{}.Parse(text, filenameForParsing)

	if info.Confidence < 0.8 || info.FirstName == "" || info.LastName == "" {
		dateStr := ""
		if info.AppointmentDate != nil {
			dateStr = info.AppointmentDate.Format("010206")
		}
		return ProcessResult{
			Success:      false,
			OriginalName: baseName,
			NeedsReview:  true,
			FirstName:    info.FirstName,
			LastName:     info.LastName,
			DateStr:      dateStr,
			Confidence:   info.Confidence,
		}
	}

	renamer := core.NewFileRenamer(outputFolder, format)
	resultPath, err := renamer.RenameFile(filePath, info, "")
	if err != nil {
		log.Printf("Error processing file: %v", err)
		return ProcessResult{Success: false, OriginalName: baseName, Error: err.Error()}
	}

	return ProcessResult{
		Success:      true,
		OriginalName: baseName,
		NewName:      filepath.Base(resultPath),
		NewPath:      resultPath,
		Confidence:   info.Confidence,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	indexTemplate.Execute(w, map[string]string{
		"DefaultOutput": filepath.Join(UploadDir, "Processed"),
	})
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ProcessResult{Success: false, Error: "no file provided"})
		return
	}
	defer file.Close()

	originalFilename := filepath.Base(header.Filename)
	format := core.ParseFileFormat(r.FormValue("format_type"))
	outputPath := ResolveOutputPath(r.FormValue("output_folder"))

	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError,
			ProcessResult{Success: false, OriginalName: originalFilename, Error: err.Error()})
		return
	}

	// Stage upload with a unique prefix to avoid conflicts
	tempPath := filepath.Join(UploadDir, uniqueID()+"_"+originalFilename)
	out, err := os.Create(tempPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError,
			ProcessResult{Success: false, OriginalName: originalFilename, Error: err.Error()})
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(tempPath)
		writeJSON(w, http.StatusInternalServerError,
			ProcessResult{Success: false, OriginalName: originalFilename, Error: err.Error()})
		return
	}
	out.Close()

	result := ProcessPDF(tempPath, format, outputPath, originalFilename)

	// Keep the staged file when review is needed — /rename-manual consumes it.
	// Otherwise clean up (on success the rename already moved it).
	if !result.NeedsReview {
		if pathExists(tempPath) {
			os.Remove(tempPath)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func handleRenameManual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := filepath.Base(r.FormValue("filename"))
	tempPath := filepath.Join(UploadDir, filename)
	if !pathExists(tempPath) {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "error": "File not found"})
		return
	}

	format := core.ParseFileFormat(r.FormValue("format_type"))
	outputPath := ResolveOutputPath(r.FormValue("output_folder"))
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": err.Error()})
		return
	}

	// Parse the date - try MMDDYY, then MM/DD/YY, else today
	dateStr := r.FormValue("date_str")
	targetDate, err := time.Parse("010206", dateStr)
	if err != nil {
		targetDate, err = time.Parse("01/02/06", dateStr)
		if err != nil {
			targetDate = time.Now()
		}
	}

	info := core.PatientInfo{
		FirstName:       r.FormValue("first_name"),
		LastName:        r.FormValue("last_name"),
		AppointmentDate: &targetDate,
		Confidence:      1.0, // user confirmed
	}

	renamer := core.NewFileRenamer(outputPath, format)
	resultPath, err := renamer.RenameFile(tempPath, info, "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"original_name": filename,
		"new_name":      filepath.Base(resultPath),
		"new_path":      resultPath,
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(strings.TrimPrefix(r.URL.Path, "/download/"))
	if filename == "" || filename == "." || filename == "/" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "File not found"})
		return
	}

	filePath := filepath.Join(UploadDir, "Processed", filename)
	if !pathExists(filePath) {
		filePath = filepath.Join(UploadDir, filename)
	}
	if !pathExists(filePath) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "File not found"})
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	http.ServeFile(w, r, filePath)
}

func uniqueID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// NewHandler builds the HTTP handler for the app.
func NewHandler() http.Handler {
	staticFS, err := fs.Sub(assets, "assets/static")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/upload", handleUpload)
	mux.HandleFunc("/rename-manual", handleRenameManual)
	mux.HandleFunc("/download/", handleDownload)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	return mux
}

// RunServer starts the web server (blocking).
func RunServer(host string, port int) error {
	if err := os.MkdirAll(UploadDir, 0o755); err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	return http.ListenAndServe(addr, NewHandler())
}
