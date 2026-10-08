package pdf

import (
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// InvertOptions configures the color inversion process.
type InvertOptions struct {
	DPI int // Resolution in dots per inch (default: 200)
}

// DefaultInvertOptions returns recommended settings for printing.
func DefaultInvertOptions() InvertOptions {
	return InvertOptions{
		DPI: 200,
	}
}

// InvertPDF inverts colors in the PDF (e.g. dark mode black-to-white for ink saving).
func InvertPDF(ctx context.Context, inputPDF, outputPDF string, opts InvertOptions) (string, error) {
	absInput, err := filepath.Abs(inputPDF)
	if err != nil {
		return "", fmt.Errorf("resolving input path: %w", err)
	}

	if _, err := os.Stat(absInput); err != nil {
		return "", fmt.Errorf("input file not found: %s", absInput)
	}

	if outputPDF == "" {
		ext := filepath.Ext(absInput)
		base := strings.TrimSuffix(absInput, ext)
		outputPDF = base + "-inverted" + ext
	}

	absOutput, err := filepath.Abs(outputPDF)
	if err != nil {
		return "", fmt.Errorf("resolving output path: %w", err)
	}

	if absInput == absOutput {
		return "", fmt.Errorf("output file cannot overwrite input file directly")
	}

	dpi := opts.DPI
	if dpi <= 0 {
		dpi = 200
	}

	// Check if pdftoppm is available
	pdftoppmPath, err := exec.LookPath("pdftoppm")
	if err != nil {
		return "", fmt.Errorf("color inversion requires 'pdftoppm' (Poppler utilities).\nTo install on macOS: brew install poppler\nTo install on Linux: sudo apt-get install poppler-utils")
	}

	tmpDir, err := os.MkdirTemp("", "duplex-invert-*")
	if err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Step 1: Render PDF pages to PNG using pdftoppm
	prefix := filepath.Join(tmpDir, "page")
	cmd := exec.CommandContext(ctx, pdftoppmPath, "-png", "-r", strconv.Itoa(dpi), absInput, prefix)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pdftoppm failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// Find all rendered PNGs
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", err
	}

	var pageFiles []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "page-") && strings.HasSuffix(entry.Name(), ".png") {
			pageFiles = append(pageFiles, filepath.Join(tmpDir, entry.Name()))
		}
	}

	if len(pageFiles) == 0 {
		return "", fmt.Errorf("no pages rendered from PDF")
	}

	sort.Strings(pageFiles)

	// Step 2: Invert colors
	// If ImageMagick (magick) is available, use it for batch inversion & assembly
	if magickPath, err := exec.LookPath("magick"); err == nil {
		args := append(pageFiles, "-negate", "-quality", "95", absOutput)
		cmdMagick := exec.CommandContext(ctx, magickPath, args...)
		outMagick, err := cmdMagick.CombinedOutput()
		if err == nil {
			return absOutput, nil
		}
		// If magick failed, fallback to pure Go inversion
		_ = outMagick
	}

	// Pure Go fallback for pixel inversion + pdfcpu assembly
	var invertedFiles []string
	for idx, pageFile := range pageFiles {
		invPath := filepath.Join(tmpDir, fmt.Sprintf("inv-%04d.png", idx+1))
		if err := invertImageFile(pageFile, invPath); err != nil {
			return "", fmt.Errorf("inverting page %d: %w", idx+1, err)
		}
		invertedFiles = append(invertedFiles, invPath)
	}

	// Assemble back into PDF using pdfcpu
	_ = os.Remove(absOutput)
	conf := model.NewDefaultConfiguration()
	if err := api.ImportImagesFile(ctx, invertedFiles, absOutput, nil, conf); err != nil {
		return "", fmt.Errorf("assembling inverted PDF: %w", err)
	}

	return absOutput, nil
}

// invertImageFile reads a PNG image, negates RGB channels, and writes to target path.
func invertImageFile(srcPath, dstPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	srcImg, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	bounds := srcImg.Bounds()
	rgba := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := srcImg.At(x, y)
			r, g, b, a := c.RGBA() // 0 - 65535
			if a == 0 {
				// Transparent background becomes pure white
				rgba.Set(x, y, color.White)
			} else {
				// Invert RGB: 255 - value
				invR := uint8(255 - (r >> 8))
				invG := uint8(255 - (g >> 8))
				invB := uint8(255 - (b >> 8))
				invA := uint8(a >> 8)
				rgba.Set(x, y, color.RGBA{R: invR, G: invG, B: invB, A: invA})
			}
		}
	}

	outF, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer outF.Close()

	return png.Encode(outF, rgba)
}
