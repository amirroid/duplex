package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"duplex/pkg/duplex"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// PDFInfo holds metadata about an inspected PDF file.
type PDFInfo struct {
	Path        string             `json:"path"`
	Filename    string             `json:"filename"`
	PageCount   int                `json:"page_count"`
	WidthPt     float64            `json:"width_pt"`
	HeightPt    float64            `json:"height_pt"`
	Orientation duplex.Orientation `json:"orientation"`
	FileSize    int64              `json:"file_size"`
}

// FormatDims returns human-friendly dimension strings (inches and mm).
func (p *PDFInfo) FormatDims() string {
	wIn := p.WidthPt / 72.0
	hIn := p.HeightPt / 72.0
	wMm := wIn * 25.4
	hMm := hIn * 25.4
	return fmt.Sprintf("%.1f x %.1f in (%.0f x %.0f mm)", wIn, hIn, wMm, hMm)
}

// FormatFileSize returns a human-readable file size (e.g. 1.2 MB).
func (p *PDFInfo) FormatFileSize() string {
	const unit = 1024
	if p.FileSize < unit {
		return fmt.Sprintf("%d B", p.FileSize)
	}
	div, exp := int64(unit), 0
	for n := p.FileSize / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(p.FileSize)/float64(div), "KMGTPE"[exp])
}

// InspectPDF validates the PDF file and extracts basic geometry and metadata.
func InspectPDF(ctx context.Context, pdfPath string) (*PDFInfo, error) {
	fi, err := os.Stat(pdfPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", pdfPath)
		}
		return nil, fmt.Errorf("cannot access file: %w", err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a PDF file: %s", pdfPath)
	}

	conf := model.NewDefaultConfiguration()
	if err := api.ValidateFile(ctx, pdfPath, conf, nil); err != nil {
		return nil, fmt.Errorf("invalid or corrupted PDF file: %w", err)
	}

	pageCount, err := api.PageCountFile(ctx, pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read page count: %w", err)
	}
	if pageCount < 1 {
		return nil, fmt.Errorf("PDF contains no pages")
	}

	dims, err := api.PageDimsFile(ctx, pdfPath)
	if err != nil || len(dims) == 0 {
		return nil, fmt.Errorf("failed to read page dimensions: %w", err)
	}

	w := dims[0].Width
	h := dims[0].Height
	orient := duplex.OrientationPortrait
	if w > h {
		orient = duplex.OrientationLandscape
	}

	return &PDFInfo{
		Path:        pdfPath,
		Filename:    filepath.Base(pdfPath),
		PageCount:   pageCount,
		WidthPt:     w,
		HeightPt:    h,
		Orientation: orient,
		FileSize:    fi.Size(),
	}, nil
}

// DuplexResult contains the paths to the generated duplex PDF files.
type DuplexResult struct {
	OutputDir  string
	FrontPDF   string
	BackPDF    string
	FrontCount int
	BackCount  int
}

// GenerateDuplex creates the front and back PDF files according to the plan.
func GenerateDuplex(ctx context.Context, inputPDF, outputDir string, plan *duplex.DuplexPlan) (*DuplexResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("duplex plan is nil")
	}

	absInput, err := filepath.Abs(inputPDF)
	if err != nil {
		return nil, fmt.Errorf("resolving input path: %w", err)
	}

	if outputDir == "" {
		base := strings.TrimSuffix(filepath.Base(absInput), filepath.Ext(absInput))
		outputDir = filepath.Join(filepath.Dir(absInput), base+"-duplex")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create output directory %s: %w", outputDir, err)
	}

	frontPath := filepath.Join(outputDir, "front.pdf")
	backPath := filepath.Join(outputDir, "back.pdf")

	if frontPath == absInput || backPath == absInput {
		return nil, fmt.Errorf("output file path collides with original PDF file")
	}

	conf := model.NewDefaultConfiguration()

	// 1. Generate Front PDF
	frontStrs := make([]string, len(plan.FrontPages))
	for i, pg := range plan.FrontPages {
		frontStrs[i] = strconv.Itoa(pg)
	}

	// Remove old front.pdf if exists
	_ = os.Remove(frontPath)
	if err := api.CollectFile(ctx, absInput, frontPath, frontStrs, conf); err != nil {
		return nil, fmt.Errorf("failed to generate front PDF: %w", err)
	}

	frontCount, err := api.PageCountFile(ctx, frontPath)
	if err != nil {
		return nil, fmt.Errorf("failed to verify front PDF: %w", err)
	}

	// 2. Generate Back PDF
	_ = os.Remove(backPath)
	backCount := 0
	if len(plan.BackPages) > 0 {
		if err := generateBackFile(ctx, absInput, backPath, plan.BackPages, conf); err != nil {
			return nil, fmt.Errorf("failed to generate back PDF: %w", err)
		}

		// Apply rotation if required
		if plan.BackRotation == 180 {
			if err := api.RotateFile(ctx, backPath, backPath, 180, nil, conf); err != nil {
				return nil, fmt.Errorf("failed to rotate back PDF: %w", err)
			}
		}

		backCount, err = api.PageCountFile(ctx, backPath)
		if err != nil {
			return nil, fmt.Errorf("failed to verify back PDF: %w", err)
		}
	}

	return &DuplexResult{
		OutputDir:  outputDir,
		FrontPDF:   frontPath,
		BackPDF:    backPath,
		FrontCount: frontCount,
		BackCount:  backCount,
	}, nil
}

// SplitOddEven splits a PDF into pure odd and even page files.
func SplitOddEven(ctx context.Context, inputPDF, outputDir string) (oddPath, evenPath string, err error) {
	absInput, err := filepath.Abs(inputPDF)
	if err != nil {
		return "", "", fmt.Errorf("resolving input path: %w", err)
	}

	info, err := InspectPDF(ctx, absInput)
	if err != nil {
		return "", "", err
	}

	if outputDir == "" {
		base := strings.TrimSuffix(filepath.Base(absInput), filepath.Ext(absInput))
		outputDir = filepath.Join(filepath.Dir(absInput), base+"-split")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", "", fmt.Errorf("cannot create output directory %s: %w", outputDir, err)
	}

	oddPath = filepath.Join(outputDir, "odd.pdf")
	evenPath = filepath.Join(outputDir, "even.pdf")

	conf := model.NewDefaultConfiguration()

	var oddPages []string
	var evenPages []string
	for i := 1; i <= info.PageCount; i++ {
		if i%2 == 1 {
			oddPages = append(oddPages, strconv.Itoa(i))
		} else {
			evenPages = append(evenPages, strconv.Itoa(i))
		}
	}

	_ = os.Remove(oddPath)
	if err := api.CollectFile(ctx, absInput, oddPath, oddPages, conf); err != nil {
		return "", "", fmt.Errorf("failed to generate odd pages: %w", err)
	}

	if len(evenPages) > 0 {
		_ = os.Remove(evenPath)
		if err := api.CollectFile(ctx, absInput, evenPath, evenPages, conf); err != nil {
			return "", "", fmt.Errorf("failed to generate even pages: %w", err)
		}
	}

	return oddPath, evenPath, nil
}

// generateBackFile generates the back PDF file, inserting blank pages at positions marked with 0.
func generateBackFile(ctx context.Context, inPDF, outPDF string, backPages []int, conf *model.Configuration) error {
	// Separate non-zero pages and zero positions
	var nonZeroPages []string
	var zeroIndices []int // index in backPages where 0 occurs

	for i, pg := range backPages {
		if pg == 0 {
			zeroIndices = append(zeroIndices, i)
		} else {
			nonZeroPages = append(nonZeroPages, strconv.Itoa(pg))
		}
	}

	// Case 1: No blank pages needed
	if len(zeroIndices) == 0 {
		return api.CollectFile(ctx, inPDF, outPDF, nonZeroPages, conf)
	}

	// Case 2: Only blank pages (e.g. 1-page PDF where back is solely blank)
	if len(nonZeroPages) == 0 {
		dims, err := api.PageDimsFile(ctx, inPDF)
		if err != nil || len(dims) == 0 {
			return fmt.Errorf("reading dimensions: %w", err)
		}
		return createBlankPDF(outPDF, dims[0].Width, dims[0].Height, len(backPages))
	}

	// Case 3: Mixed non-zero pages and blanks
	// In manual duplex printing, blanks only appear on the final sheet:
	// - Reverse mode: at index 0 (Sheet S back is fed first) -> Insert BEFORE page 1
	// - Forward mode: at last index (Sheet S back is fed last) -> Insert AFTER last page
	tmpDir, err := os.MkdirTemp("", "duplex-tmp-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpCollected := filepath.Join(tmpDir, "collected.pdf")
	if err := api.CollectFile(ctx, inPDF, tmpCollected, nonZeroPages, conf); err != nil {
		return fmt.Errorf("collecting back pages: %w", err)
	}

	// If zero is at index 0 (Reverse mode):
	if zeroIndices[0] == 0 {
		// Insert blank before page 1
		return api.InsertPagesFile(ctx, tmpCollected, outPDF, []string{"1"}, true, nil, conf)
	}

	// If zero is at the last index (Forward mode):
	lastPageStr := strconv.Itoa(len(nonZeroPages))
	return api.InsertPagesFile(ctx, tmpCollected, outPDF, []string{lastPageStr}, false, nil, conf)
}

// createBlankPDF writes a minimal valid blank PDF with the given dimensions and page count.
func createBlankPDF(outPath string, width, height float64, pageCount int) error {
	var kids strings.Builder
	for i := 0; i < pageCount; i++ {
		pageObjNum := 3 + i
		kids.WriteString(fmt.Sprintf("%d 0 R ", pageObjNum))
	}

	var sb strings.Builder
	sb.WriteString("%PDF-1.4\n")
	sb.WriteString("1 0 obj <</Type /Catalog /Pages 2 0 R>> endobj\n")
	sb.WriteString(fmt.Sprintf("2 0 obj <</Type /Pages /Kids [%s] /Count %d>> endobj\n", strings.TrimSpace(kids.String()), pageCount))

	for i := 0; i < pageCount; i++ {
		pageObjNum := 3 + i
		sb.WriteString(fmt.Sprintf("%d 0 obj <</Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f]>> endobj\n", pageObjNum, width, height))
	}

	totalObjs := 2 + pageCount
	sb.WriteString("xref\n")
	sb.WriteString(fmt.Sprintf("0 %d\n", totalObjs+1))
	sb.WriteString("0000000000 65535 f \n")
	// Write dummy xrefs, pdfcpu reconstructs xref table on validation if offsets vary
	for i := 1; i <= totalObjs; i++ {
		sb.WriteString(fmt.Sprintf("%010d 00000 n \n", i*10))
	}
	sb.WriteString(fmt.Sprintf("trailer <</Size %d /Root 1 0 R>>\nstartxref\n500\n%%%%EOF", totalObjs+1))

	return os.WriteFile(outPath, []byte(sb.String()), 0644)
}
