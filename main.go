// Jane PDF Renamer - medical office PDF renaming tool for Jane app exports.
//
// Usage:
//
//	jane-pdf-renamer                  # opens the web GUI (browser-based)
//	jane-pdf-renamer -cli chart.pdf   # headless rename
//	jane-pdf-renamer -watch ./folder  # watch folder mode
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pbarrett520/jane-pdf-renamer-v2/core"
	"github.com/pbarrett520/jane-pdf-renamer-v2/web"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	cliPath := flag.String("cli", "", "Process a single PDF file in headless mode")
	watchPath := flag.String("watch", "", "Watch a folder for new PDFs and process them automatically")
	output := flag.String("output", "", "Output folder for processed files (optional)")
	format := flag.String("format", "appt_billing",
		"Output filename format: current_discharge, appt_billing, appt_billing_eval, appt_billing_progress, appt_billing_discharge")
	port := flag.Int("port", 8080, "Port for web server")
	noBrowser := flag.Bool("no-browser", false, "Don't automatically open browser")
	flag.Parse()

	fileFormat := core.ParseFileFormat(*format)

	switch {
	case *cliPath != "":
		runCLI(*cliPath, *output, fileFormat)
	case *watchPath != "":
		runWatch(*watchPath, *output, fileFormat)
	default:
		runGUI(*port, !*noBrowser)
	}
}

func runCLI(pdfPath, outputFolder string, format core.FileFormat) {
	if _, err := os.Stat(pdfPath); err != nil {
		log.Fatalf("File not found: %s", pdfPath)
	}
	if !strings.EqualFold(filepath.Ext(pdfPath), ".pdf") {
		log.Fatalf("Not a PDF file: %s", pdfPath)
	}

	text, err := core.ExtractText(pdfPath)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	info := core.PatientInfoParser{}.Parse(text, filepath.Base(pdfPath))

	if info.Confidence < 0.8 || info.FirstName == "" || info.LastName == "" {
		fmt.Printf("⚠️  Needs review: %s\n", filepath.Base(pdfPath))
		fmt.Printf("   First: %s, Last: %s\n", orUnknown(info.FirstName), orUnknown(info.LastName))
		fmt.Printf("   Confidence: %.0f%%\n", info.Confidence*100)
		os.Exit(1)
	}

	renamer := core.NewFileRenamer(outputFolder, format)
	resultPath, err := renamer.RenameFile(pdfPath, info, "")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Renamed: %s\n", filepath.Base(resultPath))
	fmt.Printf("   Path: %s\n", resultPath)
}

func runWatch(folder, outputFolder string, format core.FileFormat) {
	if _, err := os.Stat(folder); err != nil {
		log.Fatalf("Folder not found: %s", folder)
	}

	fmt.Printf("👁️  Watching folder: %s\n", folder)
	if outputFolder != "" {
		fmt.Printf("📂 Output folder: %s\n", outputFolder)
	}
	fmt.Println("Press Ctrl+C to stop...")

	if err := watchFolder(folder, outputFolder, format); err != nil {
		log.Fatalf("Watcher failed: %v", err)
	}
}

func runGUI(port int, openBrowser bool) {
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	fmt.Printf("🌐 Starting web server at %s\n", url)

	if browser := findChromiumBrowser(); browser != "" {
		fmt.Printf("✨ Opening in %s (supports folder picker)\n", browser)
	} else {
		fmt.Println("⚠️  No Chromium browser detected - folder picker may not work")
		fmt.Println("   Install Chrome, Edge, or Brave for full functionality")
	}
	fmt.Println("Press Ctrl+C to stop...")

	if openBrowser {
		go func() {
			time.Sleep(1 * time.Second)
			openInBrowser(url)
		}()
	}

	if err := web.RunServer("127.0.0.1", port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// chromiumBrowsers lists candidate Chromium-based browsers per platform.
// These are preferred because they support the File System Access API
// (native folder picker).
func chromiumCandidates() []struct{ name, path string } {
	switch runtime.GOOS {
	case "darwin":
		return []struct{ name, path string }{
			{"Google Chrome", "/Applications/Google Chrome.app"},
			{"Microsoft Edge", "/Applications/Microsoft Edge.app"},
			{"Brave Browser", "/Applications/Brave Browser.app"},
		}
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localAppData := os.Getenv("LocalAppData")
		return []struct{ name, path string }{
			{"Google Chrome", filepath.Join(programFiles, `Google\Chrome\Application\chrome.exe`)},
			{"Google Chrome", filepath.Join(programFilesX86, `Google\Chrome\Application\chrome.exe`)},
			{"Google Chrome", filepath.Join(localAppData, `Google\Chrome\Application\chrome.exe`)},
			{"Microsoft Edge", filepath.Join(programFilesX86, `Microsoft\Edge\Application\msedge.exe`)},
			{"Microsoft Edge", filepath.Join(programFiles, `Microsoft\Edge\Application\msedge.exe`)},
			{"Brave Browser", filepath.Join(programFiles, `BraveSoftware\Brave-Browser\Application\brave.exe`)},
			{"Brave Browser", filepath.Join(localAppData, `BraveSoftware\Brave-Browser\Application\brave.exe`)},
		}
	default: // linux
		var out []struct{ name, path string }
		for _, bin := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if p, err := exec.LookPath(bin); err == nil {
				out = append(out, struct{ name, path string }{bin, p})
			}
		}
		return out
	}
}

func findChromiumBrowser() string {
	for _, c := range chromiumCandidates() {
		if _, err := os.Stat(c.path); err == nil {
			return c.name
		}
	}
	return ""
}

func openInBrowser(url string) {
	for _, c := range chromiumCandidates() {
		if _, err := os.Stat(c.path); err != nil {
			continue
		}
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", "-a", c.name, url)
		default:
			cmd = exec.Command(c.path, url)
		}
		if cmd.Start() == nil {
			return
		}
	}

	// Fallback to default browser
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

func orUnknown(s string) string {
	if s == "" {
		return "(unknown)"
	}
	return s
}
