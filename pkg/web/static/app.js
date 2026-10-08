// Duplex Web & Mobile PWA Client
// Supports 100% Offline On-Device PDF Processing via Go WebAssembly

let wasmEngineReady = false;
let deferredInstallPrompt = null;

document.addEventListener("DOMContentLoaded", () => {
  initTheme();
  initPWA();
  initWasmEngine();
  initTabs();
  initMerge();
  initDuplex();
  initInvert();
  initSplit();
});

// ================= PWA & SERVICE WORKER =================
function initPWA() {
  // 1. Register Service Worker for offline capability
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker
      .register("/sw.js", { scope: "/" })
      .then((reg) => {
        console.log("✓ Service Worker registered successfully, scope:", reg.scope);
      })
      .catch((err) => {
        console.warn("Service Worker registration failed:", err);
      });
  }

  // 2. Handle PWA Install Prompt (Android & Desktop Chrome)
  const installBtn = document.getElementById("installBtn");
  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault();
    deferredInstallPrompt = e;
    if (installBtn) {
      installBtn.style.display = "flex";
    }
  });

  if (installBtn) {
    installBtn.addEventListener("click", async () => {
      if (deferredInstallPrompt) {
        deferredInstallPrompt.prompt();
        const { outcome } = await deferredInstallPrompt.userChoice;
        if (outcome === "accepted") {
          installBtn.style.display = "none";
        }
        deferredInstallPrompt = null;
      } else {
        // Check if iOS device
        const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) && !window.MSStream;
        if (isIOS) {
          showIOSInstallModal();
        } else {
          alert("برای نصب برنامه، از منوی مرورگر خود گزینه Add to Home screen یا Install App را انتخاب کنید.");
        }
      }
    });
  }

  // Detect if already installed (standalone mode)
  if (window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true) {
    if (installBtn) installBtn.style.display = "none";
  }

  // iOS modal close triggers
  document.getElementById("iosModalClose")?.addEventListener("click", hideIOSInstallModal);
  document.getElementById("iosModalDone")?.addEventListener("click", hideIOSInstallModal);
  document.getElementById("iosModalBackdrop")?.addEventListener("click", (e) => {
    if (e.target.id === "iosModalBackdrop") hideIOSInstallModal();
  });
}

function showIOSInstallModal() {
  const modal = document.getElementById("iosModalBackdrop");
  if (modal) modal.style.display = "flex";
}

function hideIOSInstallModal() {
  const modal = document.getElementById("iosModalBackdrop");
  if (modal) modal.style.display = "none";
}

// ================= WEBASSEMBLY ENGINE =================
async function initWasmEngine() {
  const engineDot = document.getElementById("engineDot");
  const engineStatus = document.getElementById("engineStatus");

  if (typeof Go === "undefined") {
    console.warn("wasm_exec.js not loaded; fallback to server API");
    return;
  }

  try {
    if (engineDot) engineDot.classList.add("loading");
    if (engineStatus) engineStatus.textContent = "در حال بارگذاری...";

    const go = new Go();
    const wasmRes = await fetch("/static/duplex.wasm");
    if (!wasmRes.ok) {
      throw new Error("Failed to load duplex.wasm: " + wasmRes.statusText);
    }

    const wasmBytes = await wasmRes.arrayBuffer();
    const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
    go.run(instance);

    wasmEngineReady = true;
    if (engineDot) {
      engineDot.classList.remove("loading");
      engineDot.style.background = "#10b981";
    }
    if (engineStatus) {
      engineStatus.textContent = "آفلاین روی گوشی";
    }
    console.log("⚡ Duplex WebAssembly engine running locally on device (100% offline)");
  } catch (err) {
    console.warn("WASM engine init notice (fallback to server):", err);
    if (engineDot) {
      engineDot.classList.remove("loading");
      engineDot.style.background = "#3b82f6";
    }
    if (engineStatus) {
      engineStatus.textContent = "آنلاین";
    }
  }
}

// ================= THEME & TABS =================
function initTheme() {
  const toggle = document.getElementById("themeToggle");
  const saved = localStorage.getItem("duplex_theme") || "dark";
  document.documentElement.setAttribute("data-theme", saved);

  toggle?.addEventListener("click", () => {
    const current = document.documentElement.getAttribute("data-theme");
    const next = current === "dark" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    localStorage.setItem("duplex_theme", next);
  });
}

function initTabs() {
  const tabs = document.querySelectorAll(".nav-tab");
  const panels = document.querySelectorAll(".panel");

  tabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      const target = tab.dataset.tab;
      tabs.forEach((t) => t.classList.remove("active"));
      panels.forEach((p) => p.classList.remove("active"));

      tab.classList.add("active");
      document.getElementById(`panel-${target}`)?.classList.add("active");
    });
  });
}

function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

function downloadBlob(blob, filename) {
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.URL.revokeObjectURL(url);
}

// ================= MERGE TAB =================
let mergeFiles = [];

function initMerge() {
  const dropzone = document.getElementById("mergeDropzone");
  const input = document.getElementById("mergeInput");
  const list = document.getElementById("mergeFileList");
  const mergeBtn = document.getElementById("mergeBtn");
  const alertBox = document.getElementById("mergeAlert");

  dropzone.addEventListener("click", () => input.click());

  dropzone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropzone.classList.add("drag-over");
  });

  dropzone.addEventListener("dragleave", () => dropzone.classList.remove("drag-over"));

  dropzone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropzone.classList.remove("drag-over");
    handleMergeFiles(Array.from(e.dataTransfer.files));
  });

  input.addEventListener("change", (e) => {
    handleMergeFiles(Array.from(e.target.files));
    input.value = "";
  });

  function handleMergeFiles(files) {
    const pdfs = files.filter((f) => f.name.toLowerCase().endsWith(".pdf"));
    if (pdfs.length === 0) return;
    mergeFiles.push(...pdfs);
    renderMergeList();
  }

  function renderMergeList() {
    list.innerHTML = "";
    mergeBtn.disabled = mergeFiles.length < 2;

    mergeFiles.forEach((file, index) => {
      const item = document.createElement("div");
      item.className = "file-item";
      item.innerHTML = `
        <div class="file-info">
          <span class="file-badge">#${index + 1}</span>
          <div>
            <div class="file-name" title="${file.name}">${file.name}</div>
            <div class="file-meta">${formatBytes(file.size)}</div>
          </div>
        </div>
        <div class="file-actions">
          <button class="btn-icon" title="Move Up" ${index === 0 ? "disabled" : ""} data-action="up" data-index="${index}">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="18 15 12 9 6 15"/></svg>
          </button>
          <button class="btn-icon" title="Move Down" ${index === mergeFiles.length - 1 ? "disabled" : ""} data-action="down" data-index="${index}">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>
          </button>
          <button class="btn-icon delete" title="Remove" data-action="delete" data-index="${index}">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
      `;
      list.appendChild(item);
    });

    list.querySelectorAll("button[data-action]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const action = btn.dataset.action;
        const idx = parseInt(btn.dataset.index, 10);
        if (action === "up" && idx > 0) {
          const temp = mergeFiles[idx];
          mergeFiles[idx] = mergeFiles[idx - 1];
          mergeFiles[idx - 1] = temp;
        } else if (action === "down" && idx < mergeFiles.length - 1) {
          const temp = mergeFiles[idx];
          mergeFiles[idx] = mergeFiles[idx + 1];
          mergeFiles[idx + 1] = temp;
        } else if (action === "delete") {
          mergeFiles.splice(idx, 1);
        }
        renderMergeList();
      });
    });
  }

  mergeBtn.addEventListener("click", async () => {
    if (mergeFiles.length < 2) return;
    mergeBtn.disabled = true;
    mergeBtn.innerHTML = "Merging PDFs... (در حال ادغام)";
    alertBox.style.display = "none";

    try {
      // 1. Try Client-side WebAssembly Engine (100% Offline)
      if (wasmEngineReady && window.duplexWasm) {
        const buffers = await Promise.all(mergeFiles.map((f) => f.arrayBuffer()));
        const uint8Arrays = buffers.map((b) => new Uint8Array(b));
        const res = window.duplexWasm.merge(uint8Arrays, false);

        if (!res.success) {
          throw new Error(res.error || "Client merge failed");
        }

        const blob = new Blob([res.merged_pdf], { type: "application/pdf" });
        downloadBlob(blob, "merged.pdf");

        alertBox.className = "alert alert-success";
        alertBox.textContent = `✓ ادغام ${mergeFiles.length} فایل به صورت مستقیم در گوشی با موفقیت انجام شد!`;
        alertBox.style.display = "flex";
        return;
      }

      // 2. Fallback to Server API
      const formData = new FormData();
      mergeFiles.forEach((file) => formData.append("files", file));

      const res = await fetch("/api/merge", { method: "POST", body: formData });
      if (!res.ok) throw new Error(await res.text());

      const blob = await res.blob();
      downloadBlob(blob, "merged.pdf");

      alertBox.className = "alert alert-success";
      alertBox.textContent = `✓ Successfully merged ${mergeFiles.length} files into merged.pdf!`;
      alertBox.style.display = "flex";
    } catch (err) {
      alertBox.className = "alert alert-error";
      alertBox.textContent = "Error: " + err.message;
      alertBox.style.display = "flex";
    } finally {
      mergeBtn.disabled = false;
      mergeBtn.innerHTML = `
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"/></svg>
        Merge ${mergeFiles.length} PDFs (ادغام فایل‌ها)
      `;
    }
  });
}

// ================= DUPLEX TAB =================
let duplexFile = null;

function initDuplex() {
  const dropzone = document.getElementById("duplexDropzone");
  const input = document.getElementById("duplexInput");
  const details = document.getElementById("duplexDetails");
  const generateBtn = document.getElementById("duplexBtn");
  const alertBox = document.getElementById("duplexAlert");

  dropzone.addEventListener("click", () => input.click());

  dropzone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropzone.classList.add("drag-over");
  });

  dropzone.addEventListener("dragleave", () => dropzone.classList.remove("drag-over"));

  dropzone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropzone.classList.remove("drag-over");
    const file = e.dataTransfer.files[0];
    if (file && file.name.toLowerCase().endsWith(".pdf")) {
      handleDuplexFile(file);
    }
  });

  input.addEventListener("change", (e) => {
    if (e.target.files[0]) {
      handleDuplexFile(e.target.files[0]);
    }
  });

  async function handleDuplexFile(file) {
    duplexFile = file;
    document.getElementById("duplexFileName").textContent = file.name;
    document.getElementById("duplexFileSize").textContent = formatBytes(file.size);
    details.style.display = "block";
    generateBtn.disabled = false;

    // Inspect: try WASM first
    if (wasmEngineReady && window.duplexWasm) {
      try {
        const buf = await file.arrayBuffer();
        const uint8 = new Uint8Array(buf);
        const res = window.duplexWasm.inspect(uint8, file.name);
        if (res.success && res.info) {
          const info = res.info;
          const sheets = Math.ceil(info.page_count / 2);
          document.getElementById("duplexPageCount").textContent = `${info.page_count} pages`;
          document.getElementById("duplexSheetCount").textContent = `${sheets} sheets`;
          document.getElementById("duplexFormat").textContent = `${info.orientation} (${Math.round(info.width_pt / 72 * 25.4)}x${Math.round(info.height_pt / 72 * 25.4)} mm)`;
          return;
        }
      } catch (e) {
        console.warn("WASM inspect error:", e);
      }
    }

    // Server inspect fallback
    try {
      const fd = new FormData();
      fd.append("file", file);
      const res = await fetch("/api/inspect", { method: "POST", body: fd });
      if (res.ok) {
        const info = await res.json();
        const sheets = Math.ceil(info.page_count / 2);
        document.getElementById("duplexPageCount").textContent = `${info.page_count} pages`;
        document.getElementById("duplexSheetCount").textContent = `${sheets} sheets`;
        document.getElementById("duplexFormat").textContent = `${info.orientation} (${Math.round(info.width_pt / 72 * 25.4)}x${Math.round(info.height_pt / 72 * 25.4)} mm)`;
      }
    } catch (_) {}
  }

  // Option selection
  const optionCards = document.querySelectorAll(".option-card[data-mode]");
  let selectedMode = "reverse";
  let selectedRotate = "auto";

  optionCards.forEach((c) => {
    c.addEventListener("click", () => {
      optionCards.forEach((o) => o.classList.remove("active"));
      c.classList.add("active");
      selectedMode = c.dataset.mode;
      selectedRotate = c.dataset.rotate || "auto";
    });
  });

  generateBtn.addEventListener("click", async () => {
    if (!duplexFile) return;
    generateBtn.disabled = true;
    generateBtn.innerHTML = "Preparing Duplex Print... (در حال آماده‌سازی)";
    alertBox.style.display = "none";

    try {
      const base = duplexFile.name.replace(/\.pdf$/i, "");

      // 1. Client-Side WebAssembly Engine
      if (wasmEngineReady && window.duplexWasm) {
        const buf = await duplexFile.arrayBuffer();
        const uint8 = new Uint8Array(buf);
        const res = window.duplexWasm.generateDuplex(uint8, {
          mode: selectedMode,
          rotate: selectedRotate,
          odd_page: "blank"
        });

        if (!res.success) {
          throw new Error(res.error || "WASM duplex generation failed");
        }

        const zipBlob = new Blob([res.zip], { type: "application/zip" });
        downloadBlob(zipBlob, `${base}-duplex.zip`);

        alertBox.className = "alert alert-success";
        alertBox.textContent = `✓ فایل‌های front.pdf و back.pdf به صورت زیپ آماده و دانلود شدند! (${res.plan.total_sheets} برگ کاغذ)`;
        alertBox.style.display = "flex";
        return;
      }

      // 2. Server API Fallback
      const fd = new FormData();
      fd.append("file", duplexFile);
      fd.append("flip_mode", selectedMode);
      fd.append("rotation", selectedRotate);

      const res = await fetch("/api/duplex", { method: "POST", body: fd });
      if (!res.ok) throw new Error(await res.text());

      const blob = await res.blob();
      downloadBlob(blob, `${base}-duplex.zip`);

      alertBox.className = "alert alert-success";
      alertBox.textContent = "✓ Front & Back PDFs generated! Downloaded ZIP containing front.pdf and back.pdf.";
      alertBox.style.display = "flex";
    } catch (err) {
      alertBox.className = "alert alert-error";
      alertBox.textContent = "Error: " + err.message;
      alertBox.style.display = "flex";
    } finally {
      generateBtn.disabled = false;
      generateBtn.innerHTML = "Generate Duplex PDFs (دانلود فایل‌های پرینت دو رو)";
    }
  });
}

// ================= INVERT TAB =================
let invertFile = null;

function initInvert() {
  const dropzone = document.getElementById("invertDropzone");
  const input = document.getElementById("invertInput");
  const btn = document.getElementById("invertBtn");
  const alertBox = document.getElementById("invertAlert");

  dropzone.addEventListener("click", () => input.click());

  dropzone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropzone.classList.add("drag-over");
  });
  dropzone.addEventListener("dragleave", () => dropzone.classList.remove("drag-over"));
  dropzone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropzone.classList.remove("drag-over");
    const f = e.dataTransfer.files[0];
    if (f) setFile(f);
  });

  input.addEventListener("change", (e) => {
    if (e.target.files[0]) setFile(e.target.files[0]);
  });

  function setFile(f) {
    invertFile = f;
    document.getElementById("invertFileName").textContent = f.name;
    document.getElementById("invertFileDetails").style.display = "block";
    btn.disabled = false;
  }

  btn.addEventListener("click", async () => {
    if (!invertFile) return;
    btn.disabled = true;
    btn.innerHTML = "Inverting colors... (در حال تبدیل رنگ)";
    alertBox.style.display = "none";

    try {
      const fd = new FormData();
      fd.append("file", invertFile);
      const dpi = document.getElementById("invertDpi").value;
      fd.append("dpi", dpi);

      const res = await fetch("/api/invert", { method: "POST", body: fd });
      if (!res.ok) throw new Error(await res.text());

      const blob = await res.blob();
      const base = invertFile.name.replace(/\.pdf$/i, "");
      downloadBlob(blob, `${base}-inverted.pdf`);

      alertBox.className = "alert alert-success";
      alertBox.textContent = "✓ Inverted PDF generated! Black backgrounds converted to white.";
      alertBox.style.display = "flex";
    } catch (err) {
      alertBox.className = "alert alert-error";
      alertBox.textContent = "Error: " + err.message;
      alertBox.style.display = "flex";
    } finally {
      btn.disabled = false;
      btn.innerHTML = "Invert Colors & Download (تبدیل به سفید و دانلود)";
    }
  });
}

// ================= SPLIT TAB =================
let splitFile = null;

function initSplit() {
  const dropzone = document.getElementById("splitDropzone");
  const input = document.getElementById("splitInput");
  const btn = document.getElementById("splitBtn");
  const alertBox = document.getElementById("splitAlert");

  dropzone.addEventListener("click", () => input.click());

  dropzone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropzone.classList.add("drag-over");
  });
  dropzone.addEventListener("dragleave", () => dropzone.classList.remove("drag-over"));
  dropzone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropzone.classList.remove("drag-over");
    const f = e.dataTransfer.files[0];
    if (f) setFile(f);
  });

  input.addEventListener("change", (e) => {
    if (e.target.files[0]) setFile(e.target.files[0]);
  });

  function setFile(f) {
    splitFile = f;
    document.getElementById("splitFileName").textContent = f.name;
    document.getElementById("splitFileDetails").style.display = "block";
    btn.disabled = false;
  }

  btn.addEventListener("click", async () => {
    if (!splitFile) return;
    btn.disabled = true;
    btn.innerHTML = "Splitting pages... (در حال جداسازی)";
    alertBox.style.display = "none";

    try {
      const base = splitFile.name.replace(/\.pdf$/i, "");

      // 1. Try Client-side WASM engine
      if (wasmEngineReady && window.duplexWasm) {
        const buf = await splitFile.arrayBuffer();
        const uint8 = new Uint8Array(buf);
        const res = window.duplexWasm.split(uint8);

        if (!res.success) throw new Error(res.error || "WASM split failed");

        // Download odd pages
        const oddBlob = new Blob([res.odd_pdf], { type: "application/pdf" });
        downloadBlob(oddBlob, `${base}-odd.pdf`);

        // Download even pages if exists
        if (res.even_pdf) {
          setTimeout(() => {
            const evenBlob = new Blob([res.even_pdf], { type: "application/pdf" });
            downloadBlob(evenBlob, `${base}-even.pdf`);
          }, 350);
        }

        alertBox.className = "alert alert-success";
        alertBox.textContent = "✓ صفحات زوج و فرد به صورت مجزا دانلود شدند!";
        alertBox.style.display = "flex";
        return;
      }

      // 2. Fallback to Server API
      const fd = new FormData();
      fd.append("file", splitFile);

      const res = await fetch("/api/split", { method: "POST", body: fd });
      if (!res.ok) throw new Error(await res.text());

      const blob = await res.blob();
      downloadBlob(blob, `${base}-split.zip`);

      alertBox.className = "alert alert-success";
      alertBox.textContent = "✓ Split completed! Downloaded ZIP with odd.pdf and even.pdf.";
      alertBox.style.display = "flex";
    } catch (err) {
      alertBox.className = "alert alert-error";
      alertBox.textContent = "Error: " + err.message;
      alertBox.style.display = "flex";
    } finally {
      btn.disabled = false;
      btn.innerHTML = "Split Odd/Even Pages (جداسازی صفحات فرد و زوج)";
    }
  });
}
