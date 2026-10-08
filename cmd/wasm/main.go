//go:build js && wasm
// +build js,wasm

package main

import (
	"context"
	"fmt"
	"syscall/js"

	"duplex/pkg/duplex"
	"duplex/pkg/pdf"
)

func jsBytesToGo(val js.Value) []byte {
	length := val.Get("length").Int()
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, val)
	return buf
}

func goBytesToJS(data []byte) js.Value {
	uint8Array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(uint8Array, data)
	return uint8Array
}

func jsInspect(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return map[string]any{"success": false, "error": "missing file data"}
	}
	data := jsBytesToGo(args[0])
	filename := "document.pdf"
	if len(args) > 1 && args[1].Type() == js.TypeString {
		filename = args[1].String()
	}

	ctx := context.Background()
	info, err := pdf.InspectPDFBytes(ctx, data, filename)
	if err != nil {
		return map[string]any{"success": false, "error": err.Error()}
	}

	return map[string]any{
		"success": true,
		"info": map[string]any{
			"filename":    info.Filename,
			"page_count":  info.PageCount,
			"width_pt":    info.WidthPt,
			"height_pt":   info.HeightPt,
			"format_dims": info.FormatDims(),
			"orientation": string(info.Orientation),
			"file_size":   info.FileSize,
			"format_size": info.FormatFileSize(),
		},
	}
}

func jsGenerateDuplex(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return map[string]any{"success": false, "error": "missing file data"}
	}
	data := jsBytesToGo(args[0])

	ctx := context.Background()
	info, err := pdf.InspectPDFBytes(ctx, data, "input.pdf")
	if err != nil {
		return map[string]any{"success": false, "error": "inspect failed: " + err.Error()}
	}

	opts := duplex.DefaultOptions(info.Orientation)
	if len(args) > 1 && args[1].Type() == js.TypeObject {
		optObj := args[1]
		if optObj.Get("mode").Truthy() && optObj.Get("mode").String() == "forward" {
			opts.FlipMode = duplex.FlipModeForward
		}
		if optObj.Get("rotate").Truthy() {
			rStr := optObj.Get("rotate").String()
			if rStr == "180" {
				opts.Rotation = duplex.Rotate180
			} else if rStr == "0" {
				opts.Rotation = duplex.RotateNone
			}
		}
		if optObj.Get("odd_page").Truthy() && optObj.Get("odd_page").String() == "omit" {
			opts.OddPageMode = duplex.OddPageOmit
		}
	}

	plan, err := duplex.CalculatePlan(info.PageCount, opts)
	if err != nil {
		return map[string]any{"success": false, "error": "calculate plan failed: " + err.Error()}
	}

	frontBytes, backBytes, zipBytes, err := pdf.GenerateDuplexBytes(ctx, data, plan)
	if err != nil {
		return map[string]any{"success": false, "error": "generate duplex failed: " + err.Error()}
	}

	res := map[string]any{
		"success":     true,
		"front_pdf":   goBytesToJS(frontBytes),
		"front_count": len(plan.FrontPages),
		"zip":         goBytesToJS(zipBytes),
		"plan": map[string]any{
			"total_pages":   plan.TotalPages,
			"total_sheets":  plan.TotalSheets,
			"front_pages":   plan.FrontPagesList(),
			"back_pages":    plan.BackPagesList(),
			"back_rotation": plan.BackRotation,
			"flip_mode":     string(plan.Options.FlipMode),
		},
	}
	if len(backBytes) > 0 {
		res["back_pdf"] = goBytesToJS(backBytes)
		res["back_count"] = len(plan.BackPages)
	}

	return res
}

func jsMerge(this js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return map[string]any{"success": false, "error": "expected array of PDF files"}
	}

	fileArray := args[0]
	length := fileArray.Get("length").Int()
	if length < 2 {
		return map[string]any{"success": false, "error": "need at least 2 PDF files to merge"}
	}

	divider := false
	if len(args) > 1 && args[1].Type() == js.TypeBoolean {
		divider = args[1].Bool()
	}

	files := make([][]byte, length)
	for i := 0; i < length; i++ {
		files[i] = jsBytesToGo(fileArray.Index(i))
	}

	ctx := context.Background()
	merged, err := pdf.MergePDFBytes(ctx, files, divider)
	if err != nil {
		return map[string]any{"success": false, "error": err.Error()}
	}

	return map[string]any{
		"success":    true,
		"merged_pdf": goBytesToJS(merged),
	}
}

func jsSplit(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return map[string]any{"success": false, "error": "missing file data"}
	}
	data := jsBytesToGo(args[0])

	ctx := context.Background()
	odd, even, err := pdf.SplitOddEvenBytes(ctx, data)
	if err != nil {
		return map[string]any{"success": false, "error": err.Error()}
	}

	res := map[string]any{
		"success": true,
		"odd_pdf": goBytesToJS(odd),
	}
	if len(even) > 0 {
		res["even_pdf"] = goBytesToJS(even)
	}
	return res
}

func main() {
	duplexObj := js.Global().Get("Object").New()
	duplexObj.Set("inspect", js.FuncOf(jsInspect))
	duplexObj.Set("generateDuplex", js.FuncOf(jsGenerateDuplex))
	duplexObj.Set("merge", js.FuncOf(jsMerge))
	duplexObj.Set("split", js.FuncOf(jsSplit))

	js.Global().Set("duplexWasm", duplexObj)
	js.Global().Set("duplexWasmReady", js.ValueOf(true))

	// Dispatch ready event
	customEvent := js.Global().Get("CustomEvent")
	if customEvent.Truthy() {
		evt := customEvent.New("duplex-wasm-ready")
		js.Global().Call("dispatchEvent", evt)
	}

	fmt.Println("⚡ Duplex WebAssembly engine initialized successfully (100% offline, on-device)")

	// Keep WebAssembly runtime alive
	select {}
}
