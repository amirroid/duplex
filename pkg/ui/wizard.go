package ui

import (
	"context"
	"fmt"
	"path/filepath"

	"duplex/pkg/duplex"
	"duplex/pkg/pdf"
	"duplex/pkg/print"
)

// DisplayBanner prints the application header and document summary.
func DisplayBanner(info *pdf.PDFInfo, opts *duplex.Options) {
	fmt.Println()
	fmt.Printf("%s%s╔══════════════════════════════════════════════════════════════════════════╗%s\r\n", Bold, FgCyan, Reset)
	fmt.Printf("%s%s║                         DUPLEX PRINT UTILITY                             ║%s\r\n", Bold, FgCyan, Reset)
	fmt.Printf("%s%s║           Manual Double-Sided Printing on Single-Sided Printers          ║%s\r\n", Bold, FgCyan, Reset)
	fmt.Printf("%s%s╚══════════════════════════════════════════════════════════════════════════╝%s\r\n", Bold, FgCyan, Reset)

	sheets := (info.PageCount + 1) / 2
	rotStr := "None (0°)"
	autoRot := duplex.DetermineAutoRotation(opts.Orientation, opts.Binding)
	if opts.Rotation == duplex.Rotate180 || (opts.Rotation == duplex.RotateAuto && autoRot == 180) {
		rotStr = "Rotated 180°"
	}

	oddStr := "Keep whole stack (blank sheet inserted)"
	if opts.OddPageMode == duplex.OddPageOmit {
		oddStr = "Remove last printed sheet before reload"
	}

	fmt.Printf("  %sDocument:%s    %s%s%s (%s)\r\n", Bold, Reset, FgGreen, info.Filename, Reset, info.FormatFileSize())
	fmt.Printf("  %sPages:%s       %s%d%s pages (%s%d physical sheets%s needed)\r\n", Bold, Reset, Bold, info.PageCount, Reset, FgYellow, sheets, Reset)
	fmt.Printf("  %sFormat:%s      %s (%s)\r\n", Bold, Reset, info.FormatDims(), info.Orientation)
	fmt.Printf("  %sReload Mode:%s %s (Feeding: %s, Back Rotation: %s)\r\n", Bold, Reset, opts.FlipMode, opts.FlipMode, rotStr)
	if info.PageCount%2 == 1 {
		fmt.Printf("  %sOdd Handling:%s %s%s%s\r\n", Bold, Reset, Dim, oddStr, Reset)
	}
	fmt.Println()
}

// RunDuplexWizard guides the user through the 3-step manual duplex process.
func RunDuplexWizard(ctx context.Context, info *pdf.PDFInfo, opts *duplex.Options, outputDir string) error {
	plan, err := duplex.CalculatePlan(info.PageCount, *opts)
	if err != nil {
		return fmt.Errorf("planning error: %w", err)
	}

	fmt.Printf("%s[1/3] Generating print files...%s\r\n", Bold, Reset)
	res, err := pdf.GenerateDuplex(ctx, info.Path, outputDir, plan)
	if err != nil {
		return err
	}

	fmt.Printf("%s%s✓ Generation complete!%s\r\n", Bold, FgGreen, Reset)
	fmt.Printf("  • Front PDF: %s%s%s (%d pages: %s)\r\n", FgCyan, res.FrontPDF, Reset, res.FrontCount, plan.FrontPagesList())
	if res.BackCount > 0 {
		fmt.Printf("  • Back PDF:  %s%s%s (%d pages: %s)\r\n", FgCyan, res.BackPDF, Reset, res.BackCount, plan.BackPagesList())
	} else {
		fmt.Printf("  • Back PDF:  (None required for 1-page document)\r\n")
	}
	fmt.Println()

	// Step 1: Front side
	fmt.Printf("%s%s═══════════════════════ STEP 1: PRINT FRONT SIDES ════════════════════════%s\r\n", Bold, BgBlue, Reset)
	fmt.Println()
	fmt.Println("  The Front PDF contains all odd-numbered pages.")

	printer := print.GetDefaultPrinter()
	actionItems := []MenuItem{
		{ID: "print", Label: fmt.Sprintf("Send front PDF to printer (%s)", printer)},
		{ID: "open", Label: "Open front PDF in default viewer (Preview/Reader)"},
		{ID: "skip", Label: "I will print manually (continue to reload instructions)"},
	}

	choice, err := SelectMenu("How would you like to print the front side?", actionItems, 0)
	if err != nil || choice < 0 {
		return nil
	}

	switch actionItems[choice].ID {
	case "print":
		fmt.Printf("Sending %s to printer...\r\n", filepath.Base(res.FrontPDF))
		if err := print.PrintPDF(res.FrontPDF); err != nil {
			fmt.Printf("%s%sWarning: Print command failed: %v%s\r\n", Bold, FgRed, err, Reset)
			fmt.Println("Opening file in default viewer instead...")
			_ = print.OpenPDF(res.FrontPDF)
		} else {
			fmt.Printf("%s%s✓ Front PDF sent to printer!%s\r\n", Bold, FgGreen, Reset)
		}
	case "open":
		_ = print.OpenPDF(res.FrontPDF)
	}

	if res.BackCount == 0 {
		fmt.Printf("\r\n%s%s✓ Finished! Single-page document printed.%s\r\n", Bold, FgGreen, Reset)
		WaitEnter("")
		return nil
	}

	// Step 2: Paper Reload Guide
	fmt.Println()
	fmt.Printf("%s%s══════════════════════ STEP 2: RELOAD PAPER STACK ══════════════════════%s\r\n", Bold, BgCyan, Reset)
	fmt.Println()
	fmt.Println("  1. Collect the printed pages from your printer's output tray.")
	fmt.Println("  2. Keep the pages in the EXACT order they came out (DO NOT shuffle).")

	if opts.OddPageMode == duplex.OddPageOmit && info.PageCount%2 == 1 {
		fmt.Printf("  %s3. REMOVE the last printed sheet (Page %d) and set it aside.%s\r\n", Bold, info.PageCount, Reset)
	} else if opts.OddPageMode == duplex.OddPageInsertBlank && info.PageCount%2 == 1 {
		fmt.Println("  3. Keep the ENTIRE stack intact (a blank page is automatically inserted for the back).")
	}

	if opts.FlipMode == duplex.FlipModeReverse {
		fmt.Println("  4. Turn the paper stack over:")
		fmt.Println("     ┌───────────────────────┐            ┌───────────────────────┐")
		fmt.Println("     │ [Pass 1 Output Tray]  │            │  [Pass 2 Input Tray]  │")
		fmt.Println("     │ Front pages printed   │  ───────►  │  Blank side facing up │")
		fmt.Println("     │ (Sheet S on top)      │    FLIP    │  (Sheet S fed first)  │")
		fmt.Println("     └───────────────────────┘            └───────────────────────┘")
		if plan.BackRotation == 180 {
			fmt.Println("     * Back pages are rotated 180° for top-edge / calendar binding.")
		} else {
			fmt.Println("     * Flip along the long edge (standard book binding).")
		}
	} else {
		fmt.Println("  4. Re-collate or place stack so Sheet 1 is fed first.")
	}

	fmt.Println("  5. Place the stack back into your printer's paper feed tray.")
	WaitEnter("Press [Enter] when the paper stack is reloaded into the printer...")

	// Step 3: Back side
	fmt.Println()
	fmt.Printf("%s%s════════════════════════ STEP 3: PRINT BACK SIDES ════════════════════════%s\r\n", Bold, BgBlue, Reset)
	fmt.Println()
	fmt.Println("  The Back PDF will now print on the reverse side of each sheet.")

	backActionItems := []MenuItem{
		{ID: "print", Label: fmt.Sprintf("Send back PDF to printer (%s)", printer)},
		{ID: "open", Label: "Open back PDF in default viewer"},
		{ID: "skip", Label: "I will print manually (done)"},
	}

	choiceBack, err := SelectMenu("How would you like to print the back side?", backActionItems, 0)
	if err == nil && choiceBack >= 0 {
		switch backActionItems[choiceBack].ID {
		case "print":
			fmt.Printf("Sending %s to printer...\r\n", filepath.Base(res.BackPDF))
			if err := print.PrintPDF(res.BackPDF); err != nil {
				fmt.Printf("%s%sWarning: Print command failed: %v%s\r\n", Bold, FgRed, err, Reset)
				fmt.Println("Opening file in default viewer instead...")
				_ = print.OpenPDF(res.BackPDF)
			} else {
				fmt.Printf("%s%s✓ Back PDF sent to printer!%s\r\n", Bold, FgGreen, Reset)
			}
		case "open":
			_ = print.OpenPDF(res.BackPDF)
		}
	}

	fmt.Println()
	fmt.Printf("%s%s🎉 All done!%s Collect your completed double-sided document from the printer.\r\n", Bold, FgGreen, Reset)
	WaitEnter("")
	return nil
}

// ConfigureOptions allows interactive adjustment of flip, rotation, and odd-page settings.
func ConfigureOptions(orient duplex.Orientation, opts *duplex.Options) {
	for {
		items := []MenuItem{
			{
				ID:          "preset_std",
				Label:       "Standard Flip (Reverse stack, 0° rotation)",
				Description: "Most common for standard printers and book binding",
			},
			{
				ID:          "preset_rot",
				Label:       "Flip + Rotate 180°",
				Description: "For calendar / notepad binding or short-edge reload",
			},
			{
				ID:          "preset_fwd",
				Label:       "Forward Order (Sheet 1 fed first)",
				Description: "For printers with bottom feeders or manually re-collated stack",
			},
			{
				ID:          "preset_fwd_rot",
				Label:       "Forward Order + Rotate 180°",
				Description: "Forward feed with 180° inversion",
			},
			{
				ID:          "toggle_odd",
				Label:       fmt.Sprintf("Odd Page Mode: [Current: %s]", opts.OddPageMode),
				Description: "InsertBlank keeps whole stack; Omit requires removing last sheet",
			},
			{
				ID:    "back",
				Label: "← Back to Main Menu",
			},
		}

		choice, err := SelectMenu("Configure Paper Reload & Flip Mode:", items, 0)
		if err != nil || choice < 0 || items[choice].ID == "back" {
			return
		}

		switch items[choice].ID {
		case "preset_std":
			opts.FlipMode = duplex.FlipModeReverse
			opts.Rotation = duplex.RotateNone
			opts.Binding = duplex.BindingLongEdge
			fmt.Printf("%s%s✓ Set to Standard Flip.%s\r\n", Bold, FgGreen, Reset)
			return
		case "preset_rot":
			opts.FlipMode = duplex.FlipModeReverse
			opts.Rotation = duplex.Rotate180
			opts.Binding = duplex.BindingShortEdge
			fmt.Printf("%s%s✓ Set to Flip + Rotate 180°.%s\r\n", Bold, FgGreen, Reset)
			return
		case "preset_fwd":
			opts.FlipMode = duplex.FlipModeForward
			opts.Rotation = duplex.RotateNone
			fmt.Printf("%s%s✓ Set to Forward Order.%s\r\n", Bold, FgGreen, Reset)
			return
		case "preset_fwd_rot":
			opts.FlipMode = duplex.FlipModeForward
			opts.Rotation = duplex.Rotate180
			fmt.Printf("%s%s✓ Set to Forward Order + Rotate 180°.%s\r\n", Bold, FgGreen, Reset)
			return
		case "toggle_odd":
			if opts.OddPageMode == duplex.OddPageInsertBlank {
				opts.OddPageMode = duplex.OddPageOmit
			} else {
				opts.OddPageMode = duplex.OddPageInsertBlank
			}
		}
	}
}

// ShowPDFDetails displays full document specifications.
func ShowPDFDetails(info *pdf.PDFInfo) {
	fmt.Println()
	fmt.Printf("%s%s── Detailed Document Information ──%s\r\n", Bold, FgYellow, Reset)
	fmt.Printf("  File Name:       %s\r\n", info.Filename)
	fmt.Printf("  Absolute Path:   %s\r\n", info.Path)
	fmt.Printf("  File Size:       %s (%d bytes)\r\n", info.FormatFileSize(), info.FileSize)
	fmt.Printf("  Page Count:      %d pages\r\n", info.PageCount)
	fmt.Printf("  Sheets Required: %d sheets\r\n", (info.PageCount+1)/2)
	fmt.Printf("  Dimensions:      %.2f x %.2f pt\r\n", info.WidthPt, info.HeightPt)
	fmt.Printf("  Physical Size:   %s\r\n", info.FormatDims())
	fmt.Printf("  Orientation:     %s\r\n", info.Orientation)
	fmt.Println()
	WaitEnter("")
}
