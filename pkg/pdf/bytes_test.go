package pdf

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"duplex/pkg/duplex"
)

func TestBytesHelpers(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// 1. Create a 3-page test PDF
	testPath := filepath.Join(tmpDir, "test3.pdf")
	if err := createTestPDF(testPath, 612, 792, 3); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Test InspectPDFBytes
	info, err := InspectPDFBytes(ctx, data, "test3.pdf")
	if err != nil {
		t.Fatalf("InspectPDFBytes failed: %v", err)
	}
	if info.PageCount != 3 {
		t.Errorf("expected 3 pages, got %d", info.PageCount)
	}

	// 3. Test GenerateDuplexBytes (3 pages -> 2 sheets, blank on sheet 2 back)
	opts := duplex.DefaultOptions(duplex.OrientationPortrait)
	opts.FlipMode = duplex.FlipModeReverse
	opts.OddPageMode = duplex.OddPageInsertBlank
	plan, err := duplex.CalculatePlan(3, opts)
	if err != nil {
		t.Fatalf("CalculatePlan failed: %v", err)
	}

	frontBytes, backBytes, zipBytes, err := GenerateDuplexBytes(ctx, data, plan)
	if err != nil {
		t.Fatalf("GenerateDuplexBytes failed: %v", err)
	}
	if len(frontBytes) == 0 || len(backBytes) == 0 || len(zipBytes) == 0 {
		t.Errorf("expected non-empty byte slices")
	}

	// Verify zip contents
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("reading generated zip: %v", err)
	}
	foundFront := false
	foundBack := false
	for _, f := range zr.File {
		if f.Name == "front.pdf" {
			foundFront = true
		}
		if f.Name == "back.pdf" {
			foundBack = true
		}
	}
	if !foundFront || !foundBack {
		t.Errorf("zip missing front.pdf or back.pdf")
	}

	// 4. Test SplitOddEvenBytes
	oddBytes, evenBytes, err := SplitOddEvenBytes(ctx, data)
	if err != nil {
		t.Fatalf("SplitOddEvenBytes failed: %v", err)
	}
	oddInfo, err := InspectPDFBytes(ctx, oddBytes, "odd.pdf")
	if err != nil || oddInfo.PageCount != 2 {
		t.Errorf("expected 2 odd pages, got %v", oddInfo)
	}
	evenInfo, err := InspectPDFBytes(ctx, evenBytes, "even.pdf")
	if err != nil || evenInfo.PageCount != 1 {
		t.Errorf("expected 1 even page, got %v", evenInfo)
	}

	// 5. Test MergePDFBytes
	merged, err := MergePDFBytes(ctx, [][]byte{oddBytes, evenBytes}, false)
	if err != nil {
		t.Fatalf("MergePDFBytes failed: %v", err)
	}
	mergedInfo, err := InspectPDFBytes(ctx, merged, "merged.pdf")
	if err != nil || mergedInfo.PageCount != 3 {
		t.Errorf("expected 3 merged pages, got %v", mergedInfo)
	}
}
