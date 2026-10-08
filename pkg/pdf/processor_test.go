package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"duplex/pkg/duplex"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// createTestPDF creates a minimal valid multi-page PDF for testing.
func createTestPDF(path string, width, height float64, pageCount int) error {
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
	for i := 1; i <= totalObjs; i++ {
		sb.WriteString(fmt.Sprintf("%010d 00000 n \n", i*10))
	}
	sb.WriteString(fmt.Sprintf("trailer <</Size %d /Root 1 0 R>>\nstartxref\n500\n%%%%EOF", totalObjs+1))

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func TestInspectPDF(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// 1. Portrait 3-page PDF (Letter: 612x792)
	pPath := filepath.Join(tmpDir, "portrait.pdf")
	if err := createTestPDF(pPath, 612, 792, 3); err != nil {
		t.Fatal(err)
	}

	info, err := InspectPDF(ctx, pPath)
	if err != nil {
		t.Fatalf("InspectPDF error: %v", err)
	}
	if info.PageCount != 3 {
		t.Errorf("PageCount = %d, want 3", info.PageCount)
	}
	if info.Orientation != duplex.OrientationPortrait {
		t.Errorf("Orientation = %s, want Portrait", info.Orientation)
	}

	// 2. Landscape 2-page PDF (Letter landscape: 792x612)
	lPath := filepath.Join(tmpDir, "landscape.pdf")
	if err := createTestPDF(lPath, 792, 612, 2); err != nil {
		t.Fatal(err)
	}

	infoL, err := InspectPDF(ctx, lPath)
	if err != nil {
		t.Fatalf("InspectPDF landscape error: %v", err)
	}
	if infoL.PageCount != 2 {
		t.Errorf("PageCount = %d, want 2", infoL.PageCount)
	}
	if infoL.Orientation != duplex.OrientationLandscape {
		t.Errorf("Orientation = %s, want Landscape", infoL.Orientation)
	}
}

func TestGenerateDuplex_EvenPages(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// 4-page test PDF
	inputPath := filepath.Join(tmpDir, "even4.pdf")
	if err := createTestPDF(inputPath, 612, 792, 4); err != nil {
		t.Fatal(err)
	}

	plan, err := duplex.CalculatePlan(4, duplex.DefaultOptions(duplex.OrientationPortrait))
	if err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tmpDir, "out-duplex")
	res, err := GenerateDuplex(ctx, inputPath, outDir, plan)
	if err != nil {
		t.Fatalf("GenerateDuplex error: %v", err)
	}

	if res.FrontCount != 2 {
		t.Errorf("FrontCount = %d, want 2", res.FrontCount)
	}
	if res.BackCount != 2 {
		t.Errorf("BackCount = %d, want 2", res.BackCount)
	}

	// Verify front.pdf has 2 pages
	fCount, _ := api.PageCountFile(ctx, res.FrontPDF)
	if fCount != 2 {
		t.Errorf("front.pdf page count = %d, want 2", fCount)
	}
	// Verify back.pdf has 2 pages
	bCount, _ := api.PageCountFile(ctx, res.BackPDF)
	if bCount != 2 {
		t.Errorf("back.pdf page count = %d, want 2", bCount)
	}
}

func TestGenerateDuplex_OddPages_WithBlank(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// 7-page test PDF (Requires 4 sheets)
	inputPath := filepath.Join(tmpDir, "odd7.pdf")
	if err := createTestPDF(inputPath, 612, 792, 7); err != nil {
		t.Fatal(err)
	}

	opts := duplex.DefaultOptions(duplex.OrientationPortrait)
	opts.OddPageMode = duplex.OddPageInsertBlank
	plan, err := duplex.CalculatePlan(7, opts)
	if err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tmpDir, "out-odd7")
	res, err := GenerateDuplex(ctx, inputPath, outDir, plan)
	if err != nil {
		t.Fatalf("GenerateDuplex error: %v", err)
	}

	// 7 pages: 4 front pages (1, 3, 5, 7), 4 back pages (Blank, 6, 4, 2)
	if res.FrontCount != 4 {
		t.Errorf("FrontCount = %d, want 4", res.FrontCount)
	}
	if res.BackCount != 4 {
		t.Errorf("BackCount = %d, want 4", res.BackCount)
	}
}

func TestGenerateDuplex_SinglePage(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// 1-page test PDF
	inputPath := filepath.Join(tmpDir, "one.pdf")
	if err := createTestPDF(inputPath, 612, 792, 1); err != nil {
		t.Fatal(err)
	}

	opts := duplex.DefaultOptions(duplex.OrientationPortrait)
	opts.OddPageMode = duplex.OddPageInsertBlank
	plan, err := duplex.CalculatePlan(1, opts)
	if err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tmpDir, "out-one")
	res, err := GenerateDuplex(ctx, inputPath, outDir, plan)
	if err != nil {
		t.Fatalf("GenerateDuplex error: %v", err)
	}

	if res.FrontCount != 1 {
		t.Errorf("FrontCount = %d, want 1", res.FrontCount)
	}
	if res.BackCount != 1 {
		t.Errorf("BackCount = %d, want 1", res.BackCount)
	}
}

func TestSplitOddEven(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	inputPath := filepath.Join(tmpDir, "doc6.pdf")
	if err := createTestPDF(inputPath, 612, 792, 6); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tmpDir, "out-split")
	oddPath, evenPath, err := SplitOddEven(ctx, inputPath, outDir)
	if err != nil {
		t.Fatalf("SplitOddEven error: %v", err)
	}

	oCount, _ := api.PageCountFile(ctx, oddPath)
	if oCount != 3 {
		t.Errorf("odd pages = %d, want 3", oCount)
	}
	eCount, _ := api.PageCountFile(ctx, evenPath)
	if eCount != 3 {
		t.Errorf("even pages = %d, want 3", eCount)
	}
}
