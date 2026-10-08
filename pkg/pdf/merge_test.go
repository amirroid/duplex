package pdf

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMergePDFs(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	pdf1 := filepath.Join(tmpDir, "doc1.pdf")
	pdf2 := filepath.Join(tmpDir, "doc2.pdf")
	pdf3 := filepath.Join(tmpDir, "doc3.pdf")

	if err := createTestPDF(pdf1, 200, 200, 2); err != nil {
		t.Fatal(err)
	}
	if err := createTestPDF(pdf2, 200, 200, 3); err != nil {
		t.Fatal(err)
	}
	if err := createTestPDF(pdf3, 200, 200, 1); err != nil {
		t.Fatal(err)
	}

	// Test 1: Merge 2 PDFs (2 + 3 = 5 pages)
	out2 := filepath.Join(tmpDir, "merged2.pdf")
	res, err := MergePDFs(ctx, []string{pdf1, pdf2}, out2, false)
	if err != nil {
		t.Fatalf("MergePDFs failed: %v", err)
	}
	if res != out2 {
		t.Errorf("expected %s, got %s", out2, res)
	}

	info2, err := InspectPDF(ctx, out2)
	if err != nil {
		t.Fatal(err)
	}
	if info2.PageCount != 5 {
		t.Errorf("expected 5 pages, got %d", info2.PageCount)
	}

	// Test 2: Merge 3 PDFs (2 + 3 + 1 = 6 pages)
	out3 := filepath.Join(tmpDir, "merged3.pdf")
	_, err = MergePDFs(ctx, []string{pdf1, pdf2, pdf3}, out3, false)
	if err != nil {
		t.Fatalf("MergePDFs (3 files) failed: %v", err)
	}
	info3, err := InspectPDF(ctx, out3)
	if err != nil {
		t.Fatal(err)
	}
	if info3.PageCount != 6 {
		t.Errorf("expected 6 pages, got %d", info3.PageCount)
	}

	// Test 3: Less than 2 files error
	_, err = MergePDFs(ctx, []string{pdf1}, "", false)
	if err == nil {
		t.Errorf("expected error when merging only 1 file, got nil")
	}

	// Test 4: Missing file error
	_, err = MergePDFs(ctx, []string{pdf1, "nonexistent.pdf"}, "", false)
	if err == nil {
		t.Errorf("expected error for nonexistent file, got nil")
	}
}
