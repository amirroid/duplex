package pdf

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// MergePDFs combines multiple PDF files into a single document in the specified order.
// Uses a resilient multi-tier engine: pdfcpu (relaxed validation) -> pdfunite (Poppler) -> pypdf.
func MergePDFs(ctx context.Context, inFiles []string, outFile string, dividerPage bool) (string, error) {
	if len(inFiles) < 2 {
		return "", fmt.Errorf("at least 2 PDF files are required to merge, got %d", len(inFiles))
	}

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
		cnt, err := getPageCount(ctx, absPath)
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

	// Tier 1: Try pdfcpu with relaxed validation
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	conf.CheckFileNameExt = false

	mergeErr := api.MergeCreateFile(ctx, absInputs, absOutput, dividerPage, conf)
	if mergeErr == nil {
		if outCount, err := getPageCount(ctx, absOutput); err == nil && outCount > 0 {
			return absOutput, nil
		}
	}

	// Tier 2: Fallback to pdfunite (Poppler)
	// Handles non-standard PDFs with embedded equations/MathML, malformed streams, etc.
	if err := mergeWithPDFUnite(ctx, absInputs, absOutput, dividerPage); err == nil {
		if outCount, err := getPageCount(ctx, absOutput); err == nil && outCount > 0 {
			return absOutput, nil
		}
	}

	// Tier 3: Fallback to Python pypdf
	if err := mergeWithPyPDF(ctx, absInputs, absOutput, dividerPage); err == nil {
		if outCount, err := getPageCount(ctx, absOutput); err == nil && outCount > 0 {
			return absOutput, nil
		}
	}

	return "", fmt.Errorf("failed to merge PDFs: %w", mergeErr)
}

// getPageCount extracts the page count of a PDF using relaxed validation and fallbacks.
func getPageCount(ctx context.Context, pdfPath string) (int, error) {
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	f, err := os.Open(pdfPath)
	if err == nil {
		defer f.Close()
		cnt, err := api.PageCount(ctx, f, conf)
		if err == nil && cnt > 0 {
			return cnt, nil
		}

		// Try ReadContext without full validation
		_, _ = f.Seek(0, io.SeekStart)
		pdfCtx, err := api.ReadContext(ctx, f, conf)
		if err == nil && pdfCtx != nil && pdfCtx.PageCount > 0 {
			return pdfCtx.PageCount, nil
		}
	}

	// Fallback 1: pdfinfo
	if p, err := getPageCountFromPDFInfo(ctx, pdfPath); err == nil && p > 0 {
		return p, nil
	}

	// Fallback 2: python pypdf
	if p, err := getPageCountFromPyPDF(ctx, pdfPath); err == nil && p > 0 {
		return p, nil
	}

	// Fallback 3: regex scan on raw PDF bytes
	if p, err := getPageCountFromRawBytes(pdfPath); err == nil && p > 0 {
		return p, nil
	}

	return 0, fmt.Errorf("could not determine page count for %s", pdfPath)
}

func getPageCountFromPDFInfo(ctx context.Context, pdfPath string) (int, error) {
	cmd := exec.CommandContext(ctx, "pdfinfo", pdfPath)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Pages:") {
			pStr := strings.TrimSpace(strings.TrimPrefix(line, "Pages:"))
			if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
				return p, nil
			}
		}
	}
	return 0, fmt.Errorf("pages field not found in pdfinfo")
}

func getPageCountFromPyPDF(ctx context.Context, pdfPath string) (int, error) {
	cmd := exec.CommandContext(ctx, "python3", "-c", "import sys, pypdf; print(len(pypdf.PdfReader(sys.argv[1]).pages))", pdfPath)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	p, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || p <= 0 {
		return 0, fmt.Errorf("invalid pypdf output")
	}
	return p, nil
}

func getPageCountFromRawBytes(pdfPath string) (int, error) {
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return 0, err
	}
	// Look for /Count N in Catalog /Pages
	re := regexp.MustCompile(`/Count\s+(\d+)`)
	matches := re.FindAllSubmatch(data, -1)
	maxCount := 0
	for _, m := range matches {
		if len(m) > 1 {
			if cnt, err := strconv.Atoi(string(m[1])); err == nil && cnt > maxCount {
				maxCount = cnt
			}
		}
	}
	if maxCount > 0 {
		return maxCount, nil
	}
	return 0, fmt.Errorf("count not found in bytes")
}

func mergeWithPDFUnite(ctx context.Context, inFiles []string, outFile string, dividerPage bool) error {
	if _, err := exec.LookPath("pdfunite"); err != nil {
		return err
	}

	filesToMerge := inFiles
	var tmpDir string

	if dividerPage {
		var err error
		tmpDir, err = os.MkdirTemp("", "duplex-divider-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		blankPath := filepath.Join(tmpDir, "blank.pdf")
		if err := createBlankPDF(blankPath, 595.28, 841.89, 1); err != nil {
			return err
		}

		filesToMerge = make([]string, 0, len(inFiles)*2-1)
		for i, f := range inFiles {
			filesToMerge = append(filesToMerge, f)
			if i < len(inFiles)-1 {
				filesToMerge = append(filesToMerge, blankPath)
			}
		}
	}

	args := append(filesToMerge, outFile)
	cmd := exec.CommandContext(ctx, "pdfunite", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pdfunite: %s: %w", string(out), err)
	}

	return nil
}

func mergeWithPyPDF(ctx context.Context, inFiles []string, outFile string, dividerPage bool) error {
	divStr := "0"
	if dividerPage {
		divStr = "1"
	}

	pyScript := `
import sys
from pypdf import PdfWriter

divider = sys.argv[1] == "1"
out_file = sys.argv[2]
in_files = sys.argv[3:]

writer = PdfWriter()
for i, f in enumerate(in_files):
    writer.append(f)
    if divider and i < len(in_files) - 1:
        writer.add_blank_page()
with open(out_file, "wb") as fp:
    writer.write(fp)
`
	args := append([]string{"-c", pyScript, divStr, outFile}, inFiles...)
	cmd := exec.CommandContext(ctx, "python3", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pypdf merge: %s: %w", string(out), err)
	}

	return nil
}
