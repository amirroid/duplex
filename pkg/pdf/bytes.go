package pdf

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"duplex/pkg/duplex"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// InspectPDFBytes inspects an in-memory PDF buffer and extracts metadata.
func InspectPDFBytes(ctx context.Context, data []byte, filename string) (*PDFInfo, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty PDF data")
	}
	if filename == "" {
		filename = "document.pdf"
	}

	conf := model.NewDefaultConfiguration()
	rs := bytes.NewReader(data)
	if err := api.Validate(ctx, rs, conf, nil); err != nil {
		return nil, fmt.Errorf("invalid or corrupted PDF: %w", err)
	}

	_, _ = rs.Seek(0, io.SeekStart)
	pageCount, err := api.PageCount(ctx, rs, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to read page count: %w", err)
	}
	if pageCount < 1 {
		return nil, fmt.Errorf("PDF contains no pages")
	}

	_, _ = rs.Seek(0, io.SeekStart)
	dims, err := api.PageDims(ctx, rs, conf)
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
		Path:        filename,
		Filename:    filename,
		PageCount:   pageCount,
		WidthPt:     w,
		HeightPt:    h,
		Orientation: orient,
		FileSize:    int64(len(data)),
	}, nil
}

// GenerateDuplexBytes generates in-memory front.pdf, back.pdf, and a duplex.zip archive.
func GenerateDuplexBytes(ctx context.Context, data []byte, plan *duplex.DuplexPlan) (frontBytes, backBytes, zipBytes []byte, err error) {
	if plan == nil {
		return nil, nil, nil, fmt.Errorf("duplex plan is nil")
	}
	if len(data) == 0 {
		return nil, nil, nil, fmt.Errorf("empty PDF data")
	}

	conf := model.NewDefaultConfiguration()

	// 1. Generate Front PDF
	frontStrs := make([]string, len(plan.FrontPages))
	for i, pg := range plan.FrontPages {
		frontStrs[i] = strconv.Itoa(pg)
	}

	frontBuf := &bytes.Buffer{}
	if err := api.Collect(ctx, bytes.NewReader(data), frontBuf, frontStrs, conf); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate front PDF: %w", err)
	}
	frontBytes = frontBuf.Bytes()

	// 2. Generate Back PDF
	if len(plan.BackPages) > 0 {
		backBuf, err := generateBackBytes(ctx, data, plan.BackPages, conf)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to generate back PDF: %w", err)
		}

		// Apply rotation if required
		if plan.BackRotation == 180 {
			rotBuf := &bytes.Buffer{}
			if err := api.Rotate(ctx, bytes.NewReader(backBuf), rotBuf, 180, nil, conf); err != nil {
				return nil, nil, nil, fmt.Errorf("failed to rotate back PDF: %w", err)
			}
			backBytes = rotBuf.Bytes()
		} else {
			backBytes = backBuf
		}
	}

	// 3. Create ZIP archive containing front.pdf, back.pdf, and instructions
	zipBuf := &bytes.Buffer{}
	zw := zip.NewWriter(zipBuf)

	fWriter, err := zw.Create("front.pdf")
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := fWriter.Write(frontBytes); err != nil {
		return nil, nil, nil, err
	}

	if len(backBytes) > 0 {
		bWriter, err := zw.Create("back.pdf")
		if err != nil {
			return nil, nil, nil, err
		}
		if _, err := bWriter.Write(backBytes); err != nil {
			return nil, nil, nil, err
		}
	}

	readmeWriter, err := zw.Create("INSTRUCTIONS.txt")
	if err == nil {
		instructions := fmt.Sprintf("DUPLEX MANUAL PRINTING GUIDE\n============================\n\n"+
			"1. Print 'front.pdf' first on your single-sided printer.\n"+
			"2. Take the printed paper stack from the output tray.\n"+
			"3. Reload mode: %s\n"+
			"   - Standard Reverse: Turn the stack over so the blank side will be printed on,\n"+
			"     and place it back into the paper feed tray.\n"+
			"4. Print 'back.pdf'.\n"+
			"5. Your document will now be perfectly double-sided!\n", plan.Options.FlipMode)
		_, _ = readmeWriter.Write([]byte(instructions))
	}

	if err := zw.Close(); err != nil {
		return nil, nil, nil, err
	}
	zipBytes = zipBuf.Bytes()

	return frontBytes, backBytes, zipBytes, nil
}

func generateBackBytes(ctx context.Context, inData []byte, backPages []int, conf *model.Configuration) ([]byte, error) {
	var nonZeroPages []string
	var zeroIndices []int

	for i, pg := range backPages {
		if pg == 0 {
			zeroIndices = append(zeroIndices, i)
		} else {
			nonZeroPages = append(nonZeroPages, strconv.Itoa(pg))
		}
	}

	// Case 1: No blank pages needed
	if len(zeroIndices) == 0 {
		outBuf := &bytes.Buffer{}
		if err := api.Collect(ctx, bytes.NewReader(inData), outBuf, nonZeroPages, conf); err != nil {
			return nil, err
		}
		return outBuf.Bytes(), nil
	}

	// Case 2: Only blank pages
	if len(nonZeroPages) == 0 {
		dims, err := api.PageDims(ctx, bytes.NewReader(inData), conf)
		if err != nil || len(dims) == 0 {
			return nil, fmt.Errorf("reading dimensions: %w", err)
		}
		return CreateBlankPDFBytes(dims[0].Width, dims[0].Height, len(backPages))
	}

	// Case 3: Mixed non-zero pages and blanks
	collectedBuf := &bytes.Buffer{}
	if err := api.Collect(ctx, bytes.NewReader(inData), collectedBuf, nonZeroPages, conf); err != nil {
		return nil, fmt.Errorf("collecting back pages: %w", err)
	}

	outBuf := &bytes.Buffer{}
	if zeroIndices[0] == 0 {
		// Insert blank before page 1
		if err := api.InsertPages(ctx, bytes.NewReader(collectedBuf.Bytes()), outBuf, []string{"1"}, true, nil, conf); err != nil {
			return nil, err
		}
	} else {
		// Insert blank after last page
		lastPageStr := strconv.Itoa(len(nonZeroPages))
		if err := api.InsertPages(ctx, bytes.NewReader(collectedBuf.Bytes()), outBuf, []string{lastPageStr}, false, nil, conf); err != nil {
			return nil, err
		}
	}

	return outBuf.Bytes(), nil
}

// CreateBlankPDFBytes writes a minimal valid blank PDF in memory.
func CreateBlankPDFBytes(width, height float64, pageCount int) ([]byte, error) {
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

	return []byte(sb.String()), nil
}

// MergePDFBytes merges multiple in-memory PDF byte slices into a single PDF.
func MergePDFBytes(ctx context.Context, files [][]byte, dividerPage bool) ([]byte, error) {
	if len(files) < 2 {
		return nil, fmt.Errorf("need at least 2 PDF files to merge")
	}

	conf := model.NewDefaultConfiguration()
	readers := make([]io.ReadSeeker, len(files))
	for i, f := range files {
		readers[i] = bytes.NewReader(f)
	}

	outBuf := &bytes.Buffer{}
	if err := api.MergeRaw(ctx, readers, outBuf, dividerPage, conf); err != nil {
		return nil, fmt.Errorf("merge failed: %w", err)
	}

	return outBuf.Bytes(), nil
}

// SplitOddEvenBytes splits an in-memory PDF into odd and even PDF byte buffers.
func SplitOddEvenBytes(ctx context.Context, data []byte) (oddBytes, evenBytes []byte, err error) {
	info, err := InspectPDFBytes(ctx, data, "input.pdf")
	if err != nil {
		return nil, nil, err
	}

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

	oddBuf := &bytes.Buffer{}
	if err := api.Collect(ctx, bytes.NewReader(data), oddBuf, oddPages, conf); err != nil {
		return nil, nil, fmt.Errorf("collecting odd pages: %w", err)
	}
	oddBytes = oddBuf.Bytes()

	if len(evenPages) > 0 {
		evenBuf := &bytes.Buffer{}
		if err := api.Collect(ctx, bytes.NewReader(data), evenBuf, evenPages, conf); err != nil {
			return nil, nil, fmt.Errorf("collecting even pages: %w", err)
		}
		evenBytes = evenBuf.Bytes()
	}

	return oddBytes, evenBytes, nil
}
