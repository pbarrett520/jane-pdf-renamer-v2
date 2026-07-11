package main

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/pbarrett520/jane-pdf-renamer-v2/core"
)

// watchFolder watches a folder for new PDFs and processes them automatically.
// Low-confidence files are skipped with a warning (they need manual review).
func watchFolder(folder, outputFolder string, format core.FileFormat) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(folder); err != nil {
		return err
	}

	renamer := core.NewFileRenamer(outputFolder, format)
	processed := make(map[string]bool)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !event.Has(fsnotify.Create) {
				continue
			}
			path := event.Name
			if !strings.EqualFold(filepath.Ext(path), ".pdf") {
				continue
			}
			if processed[path] {
				continue
			}

			// Wait a moment for the file to be fully written
			time.Sleep(500 * time.Millisecond)

			newPath, ok := processWatchedPDF(renamer, path)
			if ok {
				processed[newPath] = true
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func processWatchedPDF(renamer core.FileRenamer, pdfPath string) (string, bool) {
	base := filepath.Base(pdfPath)
	log.Printf("Processing: %s", base)

	text, err := core.ExtractText(pdfPath)
	if err != nil {
		log.Printf("Failed to process %s: %v", base, err)
		return "", false
	}

	info := core.PatientInfoParser{}.Parse(text, base)

	if info.NeedsReview() {
		log.Printf("Low confidence for %s - skipping (needs manual review)", base)
		return "", false
	}

	newPath, err := renamer.RenameFile(pdfPath, info, "")
	if err != nil {
		log.Printf("Failed to process %s: %v", base, err)
		return "", false
	}

	log.Printf("Renamed to: %s", filepath.Base(newPath))
	return newPath, true
}
