package pdf

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestInvertImageFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcImg := filepath.Join(tmpDir, "src.png")
	dstImg := filepath.Join(tmpDir, "dst.png")

	// Create test image with black pixel at (0,0) and white pixel at (1,1)
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0, G: 0, B: 0, A: 255})       // Black
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255}) // White

	f, err := os.Create(srcImg)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := invertImageFile(srcImg, dstImg); err != nil {
		t.Fatalf("invertImageFile failed: %v", err)
	}

	// Verify inverted colors
	df, err := os.Open(dstImg)
	if err != nil {
		t.Fatal(err)
	}
	defer df.Close()

	resImg, err := png.Decode(df)
	if err != nil {
		t.Fatal(err)
	}

	// (0,0) was black, now should be white (255, 255, 255)
	c0 := resImg.At(0, 0)
	r0, g0, b0, _ := c0.RGBA()
	if r0>>8 != 255 || g0>>8 != 255 || b0>>8 != 255 {
		t.Errorf("expected white at (0,0), got r=%d g=%d b=%d", r0>>8, g0>>8, b0>>8)
	}

	// (1,1) was white, now should be black (0, 0, 0)
	c1 := resImg.At(1, 1)
	r1, g1, b1, _ := c1.RGBA()
	if r1>>8 != 0 || g1>>8 != 0 || b1>>8 != 0 {
		t.Errorf("expected black at (1,1), got r=%d g=%d b=%d", r1>>8, g1>>8, b1>>8)
	}
}

func TestInvertPDF(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	testPDF := filepath.Join(tmpDir, "test.pdf")
	if err := createTestPDF(testPDF, 200, 200, 2); err != nil {
		t.Fatal(err)
	}

	outPDF := filepath.Join(tmpDir, "inverted.pdf")
	resPath, err := InvertPDF(ctx, testPDF, outPDF, InvertOptions{DPI: 100})
	if err != nil {
		t.Fatalf("InvertPDF error: %v", err)
	}

	if resPath != outPDF {
		t.Errorf("resPath = %s, want %s", resPath, outPDF)
	}

	// Verify page count of inverted PDF
	count, err := api.PageCountFile(ctx, outPDF)
	if err != nil {
		t.Fatalf("cannot read inverted PDF page count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 pages, got %d", count)
	}
}
