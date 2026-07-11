# Jane PDF Renamer

A cross-platform (macOS + Windows) local-only tool that renames patient chart
PDFs exported from the Jane app using standardized naming conventions.

**Example:**
- **Input:** `HealthStre_Chart_1_TP_20251218_88209-2.pdf`
- **Output:** `Patient, Test 121825 PT Note.pdf`

Written in Go — each platform gets a single static binary with no runtime to
install. PDF text extraction uses
[go-pdfium](https://github.com/klippa-app/go-pdfium) (Chrome's PDF engine) in
WebAssembly mode, so builds cross-compile from any machine with no CGO.

> This is a Go rewrite of the original Python implementation, which lives in
> its own repository. This app is the source of truth going forward.

## 🔒 HIPAA Compliance

- ✅ **100% Local Processing** — no cloud, no external APIs
- ✅ **No Data Persistence** — extracted text is never written to disk
- ✅ **Memory-Only Processing** — patient information exists only during processing
- ✅ **No PHI in Logs** — logs contain file hashes, status codes, and target filenames only

## ⚡ Features

- **Web-based GUI** — browser interface with drag-and-drop and batch processing
- **5 Naming Formats** — choose date source (today vs appointment) and suffix
- **Smart Name Parsing** — splits compound names using initials from the Jane filename
- **DOI/DOB Codes** — `(DOI:MMDDYY)` / `(DOB: MM/DD/YY)` in patient names carry into the filename
- **Note Subtypes** — "Follow Up - Ortho" → `... PT Note Ortho.pdf`
- **CLI Mode** — for scripting and automation
- **Watch Mode** — auto-process new PDFs appearing in a folder
- **Smart Collision Handling** — identical files replace silently; different files get a short hash suffix

## 📋 Naming Formats

| Format | Date Source | Output Filename |
|--------|-------------|-----------------|
| `current_discharge` | Today's date | `Last, First MMDDYY PT Chart Note.pdf` |
| `appt_billing` (default) | Appointment date | `Last, First MMDDYY PT Note.pdf` |
| `appt_billing_eval` | Appointment date | `Last, First MMDDYY PT Eval Note.pdf` |
| `appt_billing_progress` | Appointment date | `Last, First MMDDYY PT Progress Note.pdf` |
| `appt_billing_discharge` | Appointment date | `Last, First MMDDYY PT Discharge Note.pdf` |

## 🖥️ Usage

### Web GUI (recommended)

Double-click the binary, or run:

```bash
./JanePDFRenamer                # opens http://127.0.0.1:8080
./JanePDFRenamer -port 3000     # different port
./JanePDFRenamer -no-browser    # don't auto-open the browser
```

The app prefers Chrome/Edge/Brave when opening the browser — the native
folder picker needs the File System Access API. Other browsers fall back to
typing an output path.

### CLI mode

```bash
./JanePDFRenamer -cli path/to/chart.pdf
./JanePDFRenamer -cli chart.pdf -format appt_billing_eval -output ./Processed
```

### Watch mode

```bash
./JanePDFRenamer -watch ./Downloads -output ./Processed
```

## 📖 Parsing Rules

1. **Patient Name** — the line after the `Chart` heading. Trailing numbers are
   stripped (`Test Patient 1` → `Test Patient`) unless a DOI/DOB code marks
   them as part of the name. Initials from the Jane filename (`..._AN_...`)
   pick the right first/last split for compound names
   (`Anna Nogales Ramirez` → last name `Nogales Ramirez`).
2. **Appointment Date** — first `MonthName DD, YYYY` occurrence, formatted MMDDYY.
3. **Note Subtype** — the heading after `Added by:`
   (`Follow Up - Ortho` → suffix `Ortho`).

Files that parse with low confidence pop a manual-review form in the GUI
(or are skipped in watch mode / flagged in CLI mode).

## 🔧 Development

Requires Go 1.22+.

```bash
go build .        # build for this machine
go test ./...     # run the test suite
go run . -no-browser
```

Extraction tests use real Jane exports from `pdf_examples/` (gitignored —
PHI) and skip automatically when absent.

### Project structure

```
├── main.go            # entry point, CLI flags, browser launching
├── watcher.go         # folder watch mode (fsnotify)
├── core/              # business logic + tests
│   ├── extractor.go   # PDF text extraction (go-pdfium WASM)
│   ├── parser.go      # patient info parsing
│   └── renamer.go     # filename generation, collision handling
├── web/
│   ├── server.go      # HTTP server (upload, manual review, download)
│   └── assets/        # embedded frontend (HTML/CSS/JS)
└── build.sh           # cross-compilation for all targets
```

## 📦 Distribution Builds

```bash
./build.sh
```

Produces in `dist/`: `JanePDFRenamer.exe` (Windows x64),
`JanePDFRenamer-mac-applesilicon`, `JanePDFRenamer-mac-intel`, and
`JanePDFRenamer-linux`. Each is a self-contained ~20 MB binary — send the
file, double-click, done.

### macOS Gatekeeper

The mac binaries are unsigned, so a *downloaded* copy is quarantined. Either
right-click → Open (twice), or:

```bash
xattr -d com.apple.quarantine ./JanePDFRenamer-mac-applesilicon
chmod +x ./JanePDFRenamer-mac-applesilicon
```

Files arriving via AirDrop or USB drive typically skip quarantine entirely.

## 🐛 Troubleshooting

- **"Needs Review" dialog** — the parser couldn't confidently extract all
  fields; verify/correct the name and date, then click Rename.
- **File not processing** — confirm it's a valid Jane chart export PDF; try
  CLI mode for detailed errors.
- **Folder picker not working** — requires a Chromium browser (Chrome, Edge,
  Brave); others can type the output path manually.

## 📄 License

Apache 2.0 — see [LICENSE](LICENSE). All patient data remains local and is
never transmitted.
