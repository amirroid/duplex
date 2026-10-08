package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"duplex/pkg/duplex"
	"duplex/pkg/pdf"
	"duplex/pkg/ui"
)

// StartServer launches the Duplex local web GUI.
func StartServer(port int, autoOpen bool) error {
	if port <= 0 {
		port = 8080
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		// If port is occupied, try next available port
		listener, err = net.Listen("tcp", ":0")
		if err != nil {
			return fmt.Errorf("failed to bind network port: %w", err)
		}
		port = listener.Addr().(*net.TCPAddr).Port
	}

	handler := NewHandler()

	localIP := getLocalIP()
	desktopURL := fmt.Sprintf("http://localhost:%d", port)
	mobileURL := fmt.Sprintf("http://%s:%d", localIP, port)

	fmt.Println()
	fmt.Printf("%s%s╔══════════════════════════════════════════════════════════════════════════╗%s\r\n", ui.Bold, ui.FgCyan, ui.Reset)
	fmt.Printf("%s%s║                       DUPLEX WEB GUI RUNNING                             ║%s\r\n", ui.Bold, ui.FgCyan, ui.Reset)
	fmt.Printf("%s%s║                Minimal Responsive GUI for Desktop & Mobile               ║%s\r\n", ui.Bold, ui.FgCyan, ui.Reset)
	fmt.Printf("%s%s╚══════════════════════════════════════════════════════════════════════════╝%s\r\n", ui.Bold, ui.FgCyan, ui.Reset)
	fmt.Printf("  🖥️  %sDesktop URL:%s  %s%s%s\r\n", ui.Bold, ui.Reset, ui.FgGreen, desktopURL, ui.Reset)
	fmt.Printf("  📱  %sMobile URL:%s   %s%s%s (Open on phone on same Wi-Fi)\r\n", ui.Bold, ui.Reset, ui.FgCyan, mobileURL, ui.Reset)
	fmt.Println()
	fmt.Println("  Press [Ctrl+C] to stop the server.")
	fmt.Println()

	if autoOpen {
		go func() {
			_ = openURL(desktopURL)
		}()
	}

	server := &http.Server{Handler: handler}
	return server.Serve(listener)
}

// NewHandler creates and configures the HTTP router.
func NewHandler() http.Handler {
	mux := http.NewServeMux()

	// Static assets
	mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		data, err := staticFS.ReadFile(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(path, ".css") {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		} else if strings.HasSuffix(path, ".js") {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		} else if strings.HasSuffix(path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		} else if strings.HasSuffix(path, ".json") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		} else if strings.HasSuffix(path, ".png") {
			w.Header().Set("Content-Type", "image/png")
		} else if strings.HasSuffix(path, ".svg") {
			w.Header().Set("Content-Type", "image/svg+xml")
		}
		_, _ = w.Write(data)
	})

	// PWA Manifest
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile("static/manifest.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		_, _ = w.Write(data)
	})

	// PWA Service Worker (with root scope permission)
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile("static/sw.js")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Service-Worker-Allowed", "/")
		_, _ = w.Write(data)
	})

	// Favicon
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile("static/favicon.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	})

	// Index page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	// API Endpoints
	mux.HandleFunc("/api/status", handleStatus)
	mux.HandleFunc("/api/inspect", handleInspect)
	mux.HandleFunc("/api/duplex", handleDuplex)
	mux.HandleFunc("/api/merge", handleMerge)
	mux.HandleFunc("/api/invert", handleInvert)
	mux.HandleFunc("/api/split", handleSplit)

	return mux
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": "1.0.0",
	})
}

func handleInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpPath, originalName, cleanup, err := saveUploadedFile(r, "file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer cleanup()

	info, err := pdf.InspectPDF(r.Context(), tmpPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info.Filename = originalName

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

func handleDuplex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpPath, originalName, cleanup, err := saveUploadedFile(r, "file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer cleanup()

	info, err := pdf.InspectPDF(r.Context(), tmpPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	opts := duplex.DefaultOptions(info.Orientation)
	if r.FormValue("flip_mode") == "forward" {
		opts.FlipMode = duplex.FlipModeForward
	}
	if rot := r.FormValue("rotation"); rot == "180" {
		opts.Rotation = duplex.Rotate180
	} else if rot == "none" {
		opts.Rotation = duplex.RotateNone
	}

	plan, err := duplex.CalculatePlan(info.PageCount, opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	outDir, err := os.MkdirTemp("", "duplex-web-out-*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(outDir)

	res, err := pdf.GenerateDuplex(r.Context(), tmpPath, outDir, plan)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create ZIP with front.pdf, back.pdf, and instructions
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// front.pdf
	if fData, err := os.ReadFile(res.FrontPDF); err == nil {
		fWriter, _ := zw.Create("front.pdf")
		_, _ = fWriter.Write(fData)
	}

	// back.pdf
	if res.BackCount > 0 {
		if bData, err := os.ReadFile(res.BackPDF); err == nil {
			bWriter, _ := zw.Create("back.pdf")
			_, _ = bWriter.Write(bData)
		}
	}

	// RELOAD_GUIDE.txt
	guide := fmt.Sprintf(`DUPLEX PRINTING RELOAD GUIDE
Document: %s (%d pages, %d sheets)
Front pages: %s
Back pages:  %s

STEPS:
1. Print 'front.pdf' on your printer.
2. Collect the printed pages without shuffling their order.
3. Turn the entire stack over (flip along the long edge).
4. Reload the stack into the printer paper tray.
5. Print 'back.pdf'.
`, originalName, info.PageCount, plan.TotalSheets, plan.FrontPagesList(), plan.BackPagesList())

	gWriter, _ := zw.Create("RELOAD_GUIDE.txt")
	_, _ = gWriter.Write([]byte(guide))
	_ = zw.Close()

	base := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-duplex.zip"`, base))
	_, _ = w.Write(buf.Bytes())
}

func handleMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with up to 64MB memory limit
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		http.Error(w, "failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) < 2 {
		http.Error(w, "at least 2 PDF files are required for merging", http.StatusBadRequest)
		return
	}

	tmpDir, err := os.MkdirTemp("", "duplex-merge-upload-*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	var savedPaths []string
	for i, fh := range files {
		file, err := fh.Open()
		if err != nil {
			http.Error(w, "reading uploaded file: "+err.Error(), http.StatusBadRequest)
			return
		}

		dstPath := filepath.Join(tmpDir, fmt.Sprintf("%03d_%s", i, fh.Filename))
		out, err := os.Create(dstPath)
		if err != nil {
			file.Close()
			http.Error(w, "saving temp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = io.Copy(out, file)
		file.Close()
		out.Close()
		if err != nil {
			http.Error(w, "saving file content: "+err.Error(), http.StatusInternalServerError)
			return
		}
		savedPaths = append(savedPaths, dstPath)
	}

	mergedPath := filepath.Join(tmpDir, "merged.pdf")
	res, err := pdf.MergePDFs(r.Context(), savedPaths, mergedPath, false)
	if err != nil {
		http.Error(w, "merge failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	mergedBytes, err := os.ReadFile(res)
	if err != nil {
		http.Error(w, "reading merged result: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="merged.pdf"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(mergedBytes)))
	_, _ = w.Write(mergedBytes)
}

func handleInvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpPath, originalName, cleanup, err := saveUploadedFile(r, "file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer cleanup()

	dpi := 200
	if dpiStr := r.FormValue("dpi"); dpiStr != "" {
		if d, err := strconv.Atoi(dpiStr); err == nil && d > 50 && d <= 600 {
			dpi = d
		}
	}

	outDir, err := os.MkdirTemp("", "duplex-invert-web-*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(outDir)

	outPath := filepath.Join(outDir, "inverted.pdf")
	res, err := pdf.InvertPDF(r.Context(), tmpPath, outPath, pdf.InvertOptions{DPI: dpi})
	if err != nil {
		http.Error(w, "inversion error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := os.ReadFile(res)
	if err != nil {
		http.Error(w, "reading result: "+err.Error(), http.StatusInternalServerError)
		return
	}

	base := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-inverted.pdf"`, base))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func handleSplit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpPath, originalName, cleanup, err := saveUploadedFile(r, "file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer cleanup()

	outDir, err := os.MkdirTemp("", "duplex-split-web-*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(outDir)

	oddPath, evenPath, err := pdf.SplitOddEven(r.Context(), tmpPath, outDir)
	if err != nil {
		http.Error(w, "split error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	if oddData, err := os.ReadFile(oddPath); err == nil {
		w1, _ := zw.Create("odd.pdf")
		_, _ = w1.Write(oddData)
	}

	if _, err := os.Stat(evenPath); err == nil {
		if evenData, err := os.ReadFile(evenPath); err == nil {
			w2, _ := zw.Create("even.pdf")
			_, _ = w2.Write(evenData)
		}
	}
	_ = zw.Close()

	base := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-split.zip"`, base))
	_, _ = w.Write(buf.Bytes())
}

func saveUploadedFile(r *http.Request, formKey string) (tmpPath string, filename string, cleanup func(), err error) {
	file, header, err := r.FormFile(formKey)
	if err != nil {
		return "", "", nil, fmt.Errorf("missing uploaded file field '%s'", formKey)
	}
	defer file.Close()

	tmpDir, err := os.MkdirTemp("", "duplex-upload-*")
	if err != nil {
		return "", "", nil, err
	}

	tmpPath = filepath.Join(tmpDir, header.Filename)
	out, err := os.Create(tmpPath)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", "", nil, err
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", "", nil, err
	}

	cleanup = func() {
		_ = os.RemoveAll(tmpDir)
	}

	return tmpPath, header.Filename, cleanup, nil
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func openURL(targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}
