# duplex

> **A cross-platform CLI utility for manual double-sided (duplex) printing on single-sided (simplex) printers.**

[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)]()
[![Go Version](https://img.shields.io/badge/go-1.26+-blue.svg)]()
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey.svg)]()
[![License](https://img.shields.io/badge/license-MIT-blue.svg)]()

`duplex` solves a classic hardware problem: printing a multi-page document double-sided when your printer only supports single-sided printing. Instead of guessing how to calculate odd/even sequences, which way to turn the paper stack, or how to handle odd page counts, `duplex` calculates the exact sequence, generates ready-to-print front and back PDFs, and guides you through reloading the paper.

---

## Features

- **Automatic Page Ordering**: Computes the exact front-side and back-side sequences so that after physically reloading the stack, the printed pages are in proper order.
- **Odd Page Count Handling**: Automatically inserts a blank page on the back of the final sheet so you can flip the entire paper stack without removing sheets. (Can also be configured to omit the blank page if you prefer removing the last sheet manually).
- **Configurable Reload & Flip Methods**:
  - **Standard Flip (Reverse Back)**: Stack turned over directly (last sheet fed first). Default for standard desktop printers.
  - **Flip + Rotate 180°**: For calendar / notepad short-edge binding or head-to-toe printing.
  - **Forward Feed**: For bottom-feeding trays or manually re-collated stacks.
- **Color Inversion (Dark Mode to Light Mode)**: Inverts PDF colors (e.g. converting solid black backgrounds to white and white text to black) to save massive amounts of printer ink and prevent paper soaking.
- **Zero-Loss PDF Processing**: Powered by pure-Go PDF processing (`pdfcpu`). Preserves 100% of vector graphics, embedded fonts, media boxes, annotations, and metadata. No rasterization.
- **Interactive Terminal UI**: Keyboard-driven interactive menu (arrow keys, Enter, Esc/q) with clear visual steps.
- **Cross-Platform Printing Integration**: Directly print front and back sides via system spoolers (`lpr` / `lp` on macOS/Linux, PowerShell print on Windows) or open files in your default PDF viewer (macOS Preview, Acrobat, etc.).
- **Standalone Static Binary**: Zero external runtime dependencies. No Python, Node.js, or virtual environments required.

---

## Installation

### macOS & Linux

Clone or open the repository directory, then run the installer:

```bash
./scripts/install.sh
```

Or using `make`:

```bash
make install
```

The installer:
1. Compiles a high-performance standalone binary (`duplex`).
2. Copies it to `/usr/local/bin/duplex` (or `~/.local/bin/duplex` for non-root users).
3. Verifies that the directory is on your `PATH`.
4. Tests the installation with `duplex --version`.

### Self-Installation via Binary

If you already have the compiled binary in your folder:

```bash
./duplex install
```

### Uninstallation

To completely remove `duplex`:

```bash
./scripts/uninstall.sh
```

Or:

```bash
duplex uninstall
```

---

## Usage

### 1. Interactive Mode (Recommended)

Run `duplex` with your target PDF file:

```bash
duplex document.pdf
```

Paths with spaces are fully supported:

```bash
duplex "/Users/me/Documents/My Document.pdf"
```

If launched without arguments:

```bash
duplex
```

`duplex` prompts you to enter a file path or drag and drop a PDF file directly from Finder/File Manager.

### Interactive Menu

When launched, `duplex` displays document statistics and an interactive menu:

```text
╔══════════════════════════════════════════════════════════════════════════╗
║                         DUPLEX PRINT UTILITY                             ║
║           Manual Double-Sided Printing on Single-Sided Printers          ║
╚══════════════════════════════════════════════════════════════════════════╝
  Document:    document.pdf (1.2 MB)
  Pages:       27 pages (14 physical sheets needed)
  Format:      8.5 x 11.0 in (216 x 279 mm) (Portrait)
  Reload Mode: Reverse (Feeding: Reverse, Back Rotation: None (0°))
  Odd Handling: Keep whole stack (blank sheet inserted)

Select an action:
❯ Prepare manual duplex (Step-by-step wizard) (Recommended: generates files & guides reload)
  Generate front and back PDFs (Creates front.pdf and back.pdf in document-duplex)
  Split odd/even pages (Creates separate odd.pdf and even.pdf files)
  Print front side (Sends front.pdf directly to system printer)
  Print back side (Sends back.pdf directly to system printer)
  Configure flip & reload settings (Change paper feeding order or 180° rotation)
  Show detailed PDF information (Page count, physical size, orientation, file size)
  Open output directory (document-duplex)
  Exit
```

* **Navigate**: `↑` / `↓` arrow keys (or `k` / `j`)
* **Select**: `Enter`
* **Exit**: `Esc` or `q`

---

## How Manual Duplex Works

### Example: 8-Page Document (4 Sheets)

```text
Pages: 1 2 3 4 5 6 7 8
Sheets: 4

Pass 1 (Front Side):
Sheet 1: Page 1
Sheet 2: Page 3
Sheet 3: Page 5
Sheet 4: Page 7
==> Front PDF: 1, 3, 5, 7

Pass 2 (Back Side - Standard Flip):
Sheet 4: Page 8
Sheet 3: Page 6
Sheet 2: Page 4
Sheet 1: Page 2
==> Back PDF: 8, 6, 4, 2
```

When Sheet 4 is printed first during the reload pass, Page 8 is printed on the back of Page 7. When the document finishes, pages `1, 2, 3, 4, 5, 6, 7, 8` are in perfect chronological order.

### Example: 7-Page Document (Odd Count, 4 Sheets)

```text
Pages: 1 2 3 4 5 6 7
Sheets: 4

Pass 1 (Front Side):
Front PDF: 1, 3, 5, 7

Pass 2 (Back Side):
Back PDF: [Blank], 6, 4, 2
```

A blank page is automatically inserted at page position 1 of `back.pdf`. You can simply pick up the entire stack from the output tray, flip it over, and place it directly into the input tray without removing any sheets.

### Paper Reload Visual Guide

```text
┌───────────────────────┐            ┌───────────────────────┐
│ [Pass 1 Output Tray]  │            │  [Pass 2 Input Tray]  │
│ Front pages printed   │  ───────►  │  Blank side facing up │
│ (Sheet S on top)      │    FLIP    │  (Sheet S fed first)  │
└───────────────────────┘            └───────────────────────┘
```

1. **Collect** the printed front stack from your printer's output tray.
2. **Do not shuffle** or alter the sheet order.
3. **Turn over** the stack along the long edge (standard book binding).
4. **Reload** the stack into the paper tray and print the back PDF.

---

## Non-Interactive & Scripting CLI Options

`duplex` can also run non-interactively in build scripts or automation pipelines:

| Flag / Option | Description |
| :--- | :--- |
| `duplex invert <file>` | Invert PDF colors (convert dark mode to white for ink saving) |
| `-i, --invert` | Invert PDF colors non-interactively and exit |
| `--dpi <n>` | Resolution DPI for color inversion (default: `200`) |
| `--generate` | Generate `front.pdf` and `back.pdf` non-interactively and exit |
| `--split` | Generate `odd.pdf` and `even.pdf` non-interactively and exit |
| `-o, --output <dir>` | Specify custom output directory or file path |
| `-m, --mode <mode>` | Flip mode: `reverse` (default) or `forward` |
| `-r, --rotate <deg>` | Back side rotation in degrees: `0`, `180`, or `-1` (auto) |
| `--omit-blank` | Omit blank page on odd page counts (requires manual removal of last sheet) |
| `-v, --version` | Display version information |
| `-h, --help` | Display command help manual |

### Script Examples

```bash
# Invert dark-mode PDF to white background (saves toner/ink):
duplex invert dark_document.pdf
# Or specify custom output name:
duplex invert dark_document.pdf -o light_document.pdf

# Generate duplex PDFs into a custom directory
duplex report.pdf --generate -o ./print-jobs

# Generate duplex with 180° rotated back pages for flip-pads / calendars
duplex calendar.pdf --generate --rotate 180

# Split a PDF into odd and even pages
duplex report.pdf --split
```

---

## Development & Testing

### Running Tests

Run the test suite with the Go race detector enabled:

```bash
make test
```

Or directly:

```bash
go test -v -race ./pkg/...
```

The test suite covers:
- Even page counts (2, 4, 6, 8, 10, ...)
- Odd page counts (1, 3, 5, 7, ...)
- Blank page insertion vs blank page omission
- Reverse and forward sheet feeding orders
- Auto-detection and 180° rotation for Portrait and Landscape documents
- Full PDF generation and page geometry verification

### Cross-Compilation

To compile standalone binaries for macOS (Intel & Apple Silicon), Linux (x86_64 & ARM64), and Windows:

```bash
make cross-compile
```

Compiled binaries will be saved in `dist/`:
- `dist/duplex-darwin-arm64`
- `dist/duplex-darwin-amd64`
- `dist/duplex-linux-amd64`
- `dist/duplex-linux-arm64`
- `dist/duplex-windows-amd64.exe`

---

## License

MIT License. See [LICENSE](LICENSE) for details.
