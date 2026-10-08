package web

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func createTestPDF(path string, pageCount int) error {
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
		sb.WriteString(fmt.Sprintf("%d 0 obj <</Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]>> endobj\n", pageObjNum))
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

func TestWebHandler_StaticAndStatus(t *testing.T) {
	handler := NewHandler()

	// Test index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("Duplex")) {
		t.Errorf("expected body to contain 'Duplex'")
	}

	// Test app.css
	req = httptest.NewRequest(http.MethodGet, "/static/app.css", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /static/app.css, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/css; charset=utf-8" {
		t.Errorf("unexpected content-type: %s", ct)
	}

	// Test app.js
	req = httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /static/app.js, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/javascript; charset=utf-8" {
		t.Errorf("unexpected content-type: %s", ct)
	}

	// Test /api/status
	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/status, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"status":"ok"`)) {
		t.Errorf("expected JSON with status ok, got %s", rr.Body.String())
	}
}

func createMultipartFile(t *testing.T, fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatalf("failed to copy content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

func TestWebHandler_InspectAndDuplex(t *testing.T) {
	// Create a dummy PDF with 3 pages
	tmpDir := t.TempDir()
	testPdfPath := tmpDir + "/test_inspect.pdf"
	if err := createTestPDF(testPdfPath, 3); err != nil {
		t.Fatalf("failed to create dummy pdf: %v", err)
	}

	pdfData, err := os.ReadFile(testPdfPath)
	if err != nil {
		t.Fatalf("failed to read test pdf: %v", err)
	}

	handler := NewHandler()

	// 1. Test /api/inspect
	body, contentType := createMultipartFile(t, "file", "test.pdf", pdfData)
	req := httptest.NewRequest(http.MethodPost, "/api/inspect", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/inspect, got %d: %s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"page_count":3`)) {
		t.Errorf("expected 3 pages in inspect result, got: %s", rr.Body.String())
	}

	// 2. Test /api/duplex
	body, contentType = createMultipartFile(t, "file", "test.pdf", pdfData)
	req = httptest.NewRequest(http.MethodPost, "/api/duplex?mode=reverse", body)
	req.Header.Set("Content-Type", contentType)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/duplex, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected application/zip, got %s", ct)
	}
}

func TestWebHandler_Merge(t *testing.T) {
	tmpDir := t.TempDir()
	p1 := tmpDir + "/p1.pdf"
	p2 := tmpDir + "/p2.pdf"
	if err := createTestPDF(p1, 2); err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}
	if err := createTestPDF(p2, 3); err != nil {
		t.Fatalf("failed to create p2: %v", err)
	}

	p1Bytes, _ := os.ReadFile(p1)
	p2Bytes, _ := os.ReadFile(p2)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part1, _ := writer.CreateFormFile("files", "doc1.pdf")
	_, _ = part1.Write(p1Bytes)

	part2, _ := writer.CreateFormFile("files", "doc2.pdf")
	_, _ = part2.Write(p2Bytes)

	_ = writer.Close()

	handler := NewHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/merge", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/merge, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected application/pdf, got %s", ct)
	}
}
