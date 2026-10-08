package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// MergePDFs combines multiple PDF files into a single document in the specified order.
func MergePDFs(ctx context.Context, inFiles []string, outFile string, dividerPage bool) (string, error) {
	if len(inFiles) < 2 {
		return "", fmt.Errorf("at least 2 PDF files are required to merge, got %d", len(inFiles))
	}

	conf := model.NewDefaultConfiguration()

	// Validate and resolve all input files
	absInputs := make([]string, len(inFiles))
	totalExpectedPages := 0
	for i, f := range inFiles {
		absPath, err := filepath.Abs(f)
		if err != nil {
			return "", fmt.Errorf("resolving input path %s: %w", f, err)
		}
		if _, err := os.Stat(absPath); err != nil {
			return "", fmt.Errorf("file not found: %s", absPath)
		}
		cnt, err := api.PageCountFile(ctx, absPath)
		if err != nil {
			return "", fmt.Errorf("invalid PDF file %s: %w", absPath, err)
		}
		totalExpectedPages += cnt
		absInputs[i] = absPath
	}

	if outFile == "" {
		firstBase := strings.TrimSuffix(filepath.Base(absInputs[0]), filepath.Ext(absInputs[0]))
		outFile = filepath.Join(filepath.Dir(absInputs[0]), firstBase+"-merged.pdf")
	}

	absOutput, err := filepath.Abs(outFile)
	if err != nil {
		return "", fmt.Errorf("resolving output path %s: %w", outFile, err)
	}

	// Ensure output directory exists
	outDir := filepath.Dir(absOutput)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	// Prevent overwriting any input file
	for _, in := range absInputs {
		if in == absOutput {
			return "", fmt.Errorf("output file cannot overwrite input file %s", in)
		}
	}

	_ = os.Remove(absOutput)

	// Call pdfcpu merge
	if err := api.MergeCreateFile(ctx, absInputs, absOutput, dividerPage, conf); err != nil {
		return "", fmt.Errorf("failed to merge PDFs: %w", err)
	}

	// Verify merged page count
	outCount, err := api.PageCountFile(ctx, absOutput)
	if err != nil {
		return "", fmt.Errorf("failed to verify merged output: %w", err)
	}

	expected := totalExpectedPages
	if dividerPage {
		expected += len(inFiles) - 1
	}
	if outCount != expected {
		return "", fmt.Errorf("merged page count mismatch: got %d, expected %d", outCount, expected)
	}

	return absOutput, nil
}
