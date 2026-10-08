package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"duplex/pkg/duplex"
	"duplex/pkg/pdf"
	"duplex/pkg/print"
	"duplex/pkg/ui"
	"duplex/pkg/web"
)

const (
	Version = "1.0.0"
	AppName = "duplex"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("%s version %s\n", AppName, Version)
			return
		case "merge":
			runMergeCommand(os.Args[2:])
			return
		case "gui", "web", "serve":
			runGUICommand(os.Args[2:])
			return
		case "invert":
			runInvertCommand(os.Args[2:])
			return
		case "install":
			runSelfInstall()
			return
		case "uninstall":
			runSelfUninstall()
			return
		case "help", "--help", "-h":
			printHelp()
			return
		}
	}

	// Parse flags
	var (
		outDir       string
		modeFlag     string
		rotateFlag   int
		nonInteract  bool
		splitOnly    bool
		invertOnly   bool
		omitBlank    bool
		dpiFlag      int
	)

	fs := flag.NewFlagSet(AppName, flag.ContinueOnError)
	fs.StringVar(&outDir, "o", "", "Custom output directory (default: <filename>-duplex)")
	fs.StringVar(&outDir, "output", "", "Custom output directory")
	fs.StringVar(&modeFlag, "m", "reverse", "Flip mode: 'reverse' (default) or 'forward'")
	fs.StringVar(&modeFlag, "mode", "reverse", "Flip mode: 'reverse' or 'forward'")
	fs.IntVar(&rotateFlag, "r", -1, "Back page rotation in degrees: 0, 180, or -1 (auto)")
	fs.IntVar(&rotateFlag, "rotate", -1, "Back page rotation")
	fs.BoolVar(&nonInteract, "generate", false, "Generate front/back PDFs non-interactively and exit")
	fs.BoolVar(&splitOnly, "split", false, "Split into odd/even PDFs non-interactively and exit")
	fs.BoolVar(&invertOnly, "invert", false, "Invert colors (dark mode to light mode) and exit")
	fs.BoolVar(&invertOnly, "i", false, "Invert colors")
	fs.IntVar(&dpiFlag, "dpi", 200, "Resolution DPI for color inversion (default: 200)")
	fs.BoolVar(&omitBlank, "omit-blank", false, "Omit blank page on odd page count (requires removing last sheet)")

	// Find the pdf path argument (could be before or after flags)
	args := os.Args[1:]
	var positionalArgs []string
	var flagArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			// Check if next arg is value for this flag
			if (arg == "-o" || arg == "--output" || arg == "-m" || arg == "--mode" || arg == "-r" || arg == "--rotate") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	_ = fs.Parse(flagArgs)

	var targetPath string
	if len(positionalArgs) > 0 {
		targetPath = positionalArgs[0]
	}

	ctx := context.Background()

	// If no PDF provided, guide user interactively
	if targetPath == "" {
		fmt.Printf("%s%s%s - Manual Double-Sided PDF Printing Utility%s\n\n", ui.Bold, AppName, ui.Reset, ui.Reset)
		fmt.Println("Usage: duplex <file.pdf> [options]")
		fmt.Println("  Or enter a PDF path below:")
		fmt.Println()

		pathInput := ui.PromptString("Enter path to PDF file (or drag & drop file here):")
		if pathInput == "" {
			fmt.Println("No file specified. Run 'duplex --help' for options.")
			os.Exit(0)
		}
		targetPath = pathInput
	}

	// Clean path (trim quotes from finder drag-and-drop, expand ~)
	targetPath = strings.Trim(targetPath, `"' `)
	if strings.HasPrefix(targetPath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			targetPath = filepath.Join(home, targetPath[2:])
		}
	}

	// Validate and inspect PDF
	info, err := pdf.InspectPDF(ctx, targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s%sError:%s %v\n", ui.Bold, ui.FgRed, ui.Reset, err)
		os.Exit(1)
	}

	// Configure options
	opts := duplex.DefaultOptions(info.Orientation)
	if strings.ToLower(modeFlag) == "forward" {
		opts.FlipMode = duplex.FlipModeForward
	}
	if rotateFlag == 0 {
		opts.Rotation = duplex.RotateNone
	} else if rotateFlag == 180 {
		opts.Rotation = duplex.Rotate180
	}
	if omitBlank {
		opts.OddPageMode = duplex.OddPageOmit
	}

	// Non-interactive generation flags
	if nonInteract {
		plan, err := duplex.CalculatePlan(info.PageCount, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Plan error: %v\n", err)
			os.Exit(1)
		}
		res, err := pdf.GenerateDuplex(ctx, info.Path, outDir, plan)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Generation error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Generated front PDF: %s (%d pages)\n", res.FrontPDF, res.FrontCount)
		fmt.Printf("Generated back PDF:  %s (%d pages)\n", res.BackPDF, res.BackCount)
		return
	}

	if splitOnly {
		odd, even, err := pdf.SplitOddEven(ctx, info.Path, outDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Split error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Odd pages:  %s\n", odd)
		fmt.Printf("Even pages: %s\n", even)
		return
	}

	if invertOnly {
		out, err := pdf.InvertPDF(ctx, info.Path, outDir, pdf.InvertOptions{DPI: dpiFlag})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invert error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Inverted PDF generated: %s\n", out)
		return
	}

	// Run interactive menu
	runInteractiveLoop(ctx, info, &opts, outDir)
}

func runInteractiveLoop(ctx context.Context, info *pdf.PDFInfo, opts *duplex.Options, outDir string) {
	defaultDir := outDir
	if defaultDir == "" {
		base := strings.TrimSuffix(filepath.Base(info.Path), filepath.Ext(info.Path))
		defaultDir = filepath.Join(filepath.Dir(info.Path), base+"-duplex")
	}

	for {
		ui.DisplayBanner(info, opts)

		menuItems := []ui.MenuItem{
			{
				ID:          "wizard",
				Label:       "Prepare manual duplex (Step-by-step wizard)",
				Description: "Recommended: generates files & guides reload process",
			},
			{
				ID:          "gen_both",
				Label:       "Generate front and back PDFs",
				Description: fmt.Sprintf("Creates front.pdf and back.pdf in %s", filepath.Base(defaultDir)),
			},
			{
				ID:          "merge",
				Label:       "Merge multiple PDFs into one",
				Description: "Combine multiple PDF documents into a single file",
			},
			{
				ID:          "gui",
				Label:       "Launch Web GUI (Desktop & Mobile)",
				Description: "Opens beautiful visual interface in your browser",
			},
			{
				ID:          "invert",
				Label:       "Invert colors (Dark mode to light mode / ink saver)",
				Description: "Reverses colors (black background becomes white) to save printer ink",
			},
			{
				ID:          "split_odd_even",
				Label:       "Split into odd/even pages",
				Description: "Creates separate odd.pdf and even.pdf files",
			},
			{
				ID:          "print_front",
				Label:       "Print front side",
				Description: "Sends front.pdf directly to system printer",
			},
			{
				ID:          "print_back",
				Label:       "Print back side",
				Description: "Sends back.pdf directly to system printer",
			},
			{
				ID:          "config",
				Label:       "Configure flip & reload settings",
				Description: "Change paper feeding order or 180° rotation",
			},
			{
				ID:          "info",
				Label:       "Show detailed PDF information",
				Description: "Page count, physical size, orientation, file size",
			},
			{
				ID:          "open_dir",
				Label:       "Open output directory",
				Description: defaultDir,
			},
			{
				ID:    "exit",
				Label: "Exit",
			},
		}

		choice, err := ui.SelectMenu("Select an action:", menuItems, 0)
		if err != nil || choice < 0 || menuItems[choice].ID == "exit" {
			fmt.Printf("\r\n%sGoodbye!%s\r\n", ui.Dim, ui.Reset)
			return
		}

		switch menuItems[choice].ID {
		case "wizard":
			_ = ui.RunDuplexWizard(ctx, info, opts, defaultDir)

		case "gen_both":
			plan, err := duplex.CalculatePlan(info.PageCount, *opts)
			if err != nil {
				fmt.Printf("%sError:%s %v\r\n", ui.FgRed, ui.Reset, err)
				ui.WaitEnter("")
				continue
			}
			res, err := pdf.GenerateDuplex(ctx, info.Path, defaultDir, plan)
			if err != nil {
				fmt.Printf("%sGeneration failed:%s %v\r\n", ui.FgRed, ui.Reset, err)
			} else {
				fmt.Printf("\r\n%s%s✓ Duplex PDFs generated successfully!%s\r\n", ui.Bold, ui.FgGreen, ui.Reset)
				fmt.Printf("  • Front: %s (%d pages: %s)\r\n", res.FrontPDF, res.FrontCount, plan.FrontPagesList())
				fmt.Printf("  • Back:  %s (%d pages: %s)\r\n", res.BackPDF, res.BackCount, plan.BackPagesList())
			}
			ui.WaitEnter("")

		case "invert":
			fmt.Printf("\r\n%sInverting colors for %s (DPI: 200)...%s\r\n", ui.Bold, info.Filename, ui.Reset)
			outPath, err := pdf.InvertPDF(ctx, info.Path, "", pdf.InvertOptions{DPI: 200})
			if err != nil {
				fmt.Printf("%sInversion failed:%s %v\r\n", ui.FgRed, ui.Reset, err)
			} else {
				fmt.Printf("\r\n%s%s✓ Successfully inverted PDF:%s %s\r\n", ui.Bold, ui.FgGreen, ui.Reset, outPath)
				if ui.Confirm("Would you like to switch active document to this inverted PDF for printing?", true) {
					newInfo, err := pdf.InspectPDF(ctx, outPath)
					if err == nil {
						info = newInfo
						base := strings.TrimSuffix(filepath.Base(outPath), filepath.Ext(outPath))
						defaultDir = filepath.Join(filepath.Dir(outPath), base+"-duplex")
						fmt.Printf("%s%s✓ Active document switched to: %s%s\r\n", ui.Bold, ui.FgCyan, info.Filename, ui.Reset)
					}
				}
			}
			ui.WaitEnter("")

		case "split_odd_even":
			odd, even, err := pdf.SplitOddEven(ctx, info.Path, "")
			if err != nil {
				fmt.Printf("%sSplit failed:%s %v\r\n", ui.FgRed, ui.Reset, err)
			} else {
				fmt.Printf("\r\n%s%s✓ Split complete!%s\r\n", ui.Bold, ui.FgGreen, ui.Reset)
				fmt.Printf("  • Odd pages:  %s\r\n", odd)
				fmt.Printf("  • Even pages: %s\r\n", even)
			}
			ui.WaitEnter("")

		case "print_front":
			frontFile := filepath.Join(defaultDir, "front.pdf")
			if _, err := os.Stat(frontFile); os.IsNotExist(err) {
				// Generate first
				plan, _ := duplex.CalculatePlan(info.PageCount, *opts)
				_, _ = pdf.GenerateDuplex(ctx, info.Path, defaultDir, plan)
			}
			fmt.Printf("Sending %s to default printer...\r\n", frontFile)
			if err := print.PrintPDF(frontFile); err != nil {
				fmt.Printf("%sPrint error: %v%s\r\n", ui.FgRed, err, ui.Reset)
			} else {
				fmt.Printf("%s%s✓ Front PDF sent to printer.%s\r\n", ui.Bold, ui.FgGreen, ui.Reset)
			}
			ui.WaitEnter("")

		case "print_back":
			backFile := filepath.Join(defaultDir, "back.pdf")
			if _, err := os.Stat(backFile); os.IsNotExist(err) {
				// Generate first
				plan, _ := duplex.CalculatePlan(info.PageCount, *opts)
				_, _ = pdf.GenerateDuplex(ctx, info.Path, defaultDir, plan)
			}
			if _, err := os.Stat(backFile); os.IsNotExist(err) {
				fmt.Printf("No back side exists for this document.\r\n")
				ui.WaitEnter("")
				continue
			}
			fmt.Printf("Sending %s to default printer...\r\n", backFile)
			if err := print.PrintPDF(backFile); err != nil {
				fmt.Printf("%sPrint error: %v%s\r\n", ui.FgRed, err, ui.Reset)
			} else {
				fmt.Printf("%s%s✓ Back PDF sent to printer.%s\r\n", ui.Bold, ui.FgGreen, ui.Reset)
			}
			ui.WaitEnter("")

		case "config":
			ui.ConfigureOptions(info.Orientation, opts)

		case "info":
			ui.ShowPDFDetails(info)

		case "merge":
			fmt.Printf("\r\n%sMerge PDFs%s\r\n", ui.Bold, ui.Reset)
			fmt.Println("Enter additional PDF file paths to merge with current document (separated by comma):")
			line := ui.PromptString("Additional PDF file(s):")
			parts := strings.Split(line, ",")
			files := []string{info.Path}
			for _, p := range parts {
				clean := strings.Trim(strings.TrimSpace(p), `"'`)
				if clean != "" {
					files = append(files, clean)
				}
			}
			if len(files) < 2 {
				fmt.Printf("%sNeed at least 1 additional PDF to merge.%s\r\n", ui.FgYellow, ui.Reset)
				ui.WaitEnter("")
				continue
			}
			outPath, err := pdf.MergePDFs(ctx, files, "", false)
			if err != nil {
				fmt.Printf("%sMerge failed:%s %v\r\n", ui.FgRed, ui.Reset, err)
			} else {
				fmt.Printf("\r\n%s%s✓ Successfully merged into:%s %s\r\n", ui.Bold, ui.FgGreen, ui.Reset, outPath)
			}
			ui.WaitEnter("")

		case "gui":
			fmt.Printf("\r\n%sLaunching Web GUI (Desktop & Mobile)...%s\r\n", ui.Bold, ui.Reset)
			if err := web.StartServer(8080, true); err != nil {
				fmt.Printf("%sServer error:%s %v\r\n", ui.FgRed, ui.Reset, err)
				ui.WaitEnter("")
			}

		case "open_dir":
			_ = os.MkdirAll(defaultDir, 0755)
			_ = print.OpenFolder(defaultDir)
		}
	}
}

func runMergeCommand(args []string) {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	var outPath string
	var divider bool
	fs.StringVar(&outPath, "o", "", "Output merged PDF path (default: <first_name>-merged.pdf)")
	fs.StringVar(&outPath, "output", "", "Output merged PDF path")
	fs.BoolVar(&divider, "divider", false, "Insert blank divider page between merged documents")

	var positionalArgs []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "-o" || arg == "--output") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	_ = fs.Parse(flagArgs)
	if len(positionalArgs) < 2 {
		fmt.Println("Usage: duplex merge <file1.pdf> <file2.pdf> [file3.pdf...] [-o output.pdf] [--divider]")
		os.Exit(1)
	}

	ctx := context.Background()
	fmt.Printf("Merging %d PDF files...\n", len(positionalArgs))
	out, err := pdf.MergePDFs(ctx, positionalArgs, outPath, divider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Merged PDF generated successfully: %s\n", out)
}

func runGUICommand(args []string) {
	fs := flag.NewFlagSet("gui", flag.ContinueOnError)
	var port int
	var noOpen bool
	fs.IntVar(&port, "p", 8080, "Port to listen on (default: 8080)")
	fs.IntVar(&port, "port", 8080, "Port to listen on (default: 8080)")
	fs.BoolVar(&noOpen, "no-open", false, "Do not open browser automatically")

	_ = fs.Parse(args)
	if err := web.StartServer(port, !noOpen); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func runInvertCommand(args []string) {
	var outPath string
	var dpi int
	fs := flag.NewFlagSet("invert", flag.ContinueOnError)
	fs.StringVar(&outPath, "o", "", "Output PDF path (default: <filename>-inverted.pdf)")
	fs.StringVar(&outPath, "output", "", "Output PDF path")
	fs.IntVar(&dpi, "dpi", 200, "Rendering resolution in DPI (default: 200)")

	var positionalArgs []string
	var flagArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			if (arg == "-o" || arg == "--output" || arg == "--dpi") && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}

	_ = fs.Parse(flagArgs)
	if len(positionalArgs) == 0 {
		fmt.Println("Usage: duplex invert <file.pdf> [-o output.pdf] [--dpi 200]")
		os.Exit(1)
	}

	target := positionalArgs[0]
	ctx := context.Background()
	fmt.Printf("Inverting colors for: %s (DPI: %d)...\n", target, dpi)
	out, err := pdf.InvertPDF(ctx, target, outPath, pdf.InvertOptions{DPI: dpi})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Inverted PDF generated successfully: %s\n", out)
}

func printHelp() {
	fmt.Printf("%s%s - Manual Double-Sided PDF Printing Utility%s\n\n", ui.Bold, AppName, ui.Reset)
	fmt.Println("SYNOPSIS:")
	fmt.Println("  duplex <file.pdf> [options]")
	fmt.Println("  duplex [command]")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  merge <files>  Merge multiple PDF files into a single document")
	fmt.Println("  gui            Launch beautiful minimal Web GUI for mobile & desktop")
	fmt.Println("  invert <file>  Invert PDF colors (convert dark mode to white for ink saving)")
	fmt.Println("  install        Install duplex executable globally to PATH")
	fmt.Println("  uninstall      Remove globally installed duplex executable")
	fmt.Println("  version        Show duplex version")
	fmt.Println("  help           Display this help manual")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  -o, --output <dir>    Custom destination directory for generated PDFs")
	fmt.Println("  -m, --mode <mode>     Paper reload mode: 'reverse' (default) or 'forward'")
	fmt.Println("  -r, --rotate <deg>    Back page rotation: 0, 180, or -1 (auto)")
	fmt.Println("      --generate        Generate front and back PDFs non-interactively and exit")
	fmt.Println("  -i, --invert          Invert PDF colors and exit")
	fmt.Println("      --dpi <n>         DPI resolution for color inversion (default: 200)")
	fmt.Println("      --split           Split into odd and even PDFs non-interactively and exit")
	fmt.Println("      --omit-blank      Omit blank page on odd counts (requires removing last sheet)")
	fmt.Println("  -v, --version         Show version")
	fmt.Println("  -h, --help            Show help")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Launch interactive duplex menu for a document:")
	fmt.Println("  duplex document.pdf")
	fmt.Println()
	fmt.Println("  # Launch minimal Web GUI (accessible from desktop and mobile):")
	fmt.Println("  duplex gui")
	fmt.Println()
	fmt.Println("  # Merge multiple PDFs into one:")
	fmt.Println("  duplex merge file1.pdf file2.pdf -o merged.pdf")
	fmt.Println()
	fmt.Println("  # Invert dark-mode PDF to light-mode to save printer ink:")
	fmt.Println("  duplex invert dark_document.pdf")
	fmt.Println()
	fmt.Println("  # Handles paths with spaces:")
	fmt.Println("  duplex \"/Users/me/Documents/Annual Report 2026.pdf\"")
	fmt.Println()
	fmt.Println("  # Generate duplex files non-interactively in scripts:")
	fmt.Println("  duplex document.pdf --generate")
	fmt.Println()
	fmt.Println("  # Install duplex globally:")
	fmt.Println("  duplex install")
}

func runSelfInstall() {
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot locate executable: %v\n", err)
		os.Exit(1)
	}

	destDirs := []string{"/usr/local/bin", filepath.Join(os.Getenv("HOME"), ".local", "bin")}
	var targetDir string

	for _, dir := range destDirs {
		if testWritable(dir) {
			targetDir = dir
			break
		}
	}

	if targetDir == "" {
		// Try creating ~/.local/bin
		localBin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
		if err := os.MkdirAll(localBin, 0755); err == nil {
			targetDir = localBin
		} else {
			targetDir = "/usr/local/bin"
		}
	}

	targetPath := filepath.Join(targetDir, AppName)
	if err := copyExecutable(execPath, targetPath); err != nil {
		fmt.Printf("Unable to install to %s: %v\n", targetPath, err)
		fmt.Println("Please run with elevated privileges (e.g. sudo make install) or use ./scripts/install.sh")
		os.Exit(1)
	}

	fmt.Printf("%s%s✓ Successfully installed %s to %s%s\n", ui.Bold, ui.FgGreen, AppName, targetPath, ui.Reset)
	ensurePathNotice(targetDir)
}

func runSelfUninstall() {
	candidates := []string{
		filepath.Join("/usr/local/bin", AppName),
		filepath.Join(os.Getenv("HOME"), ".local", "bin", AppName),
	}

	removed := false
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if err := os.Remove(p); err == nil {
				fmt.Printf("%s%s✓ Removed %s%s\n", ui.Bold, ui.FgGreen, p, ui.Reset)
				removed = true
			} else {
				fmt.Printf("Failed to remove %s: %v (try sudo)\n", p, err)
			}
		}
	}

	if !removed {
		fmt.Println("No installed duplex binary found in standard PATH directories.")
	}
}

func testWritable(dir string) bool {
	testFile := filepath.Join(dir, ".duplex_test")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return false
	}
	_ = os.Remove(testFile)
	return true
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_ = os.Remove(dst)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := os.Chmod(dst, 0755); err != nil {
		return err
	}
	_ = exec.Command("codesign", "-s", "-", "-f", dst).Run()
	return nil
}

func ensurePathNotice(dir string) {
	pathEnv := os.Getenv("PATH")
	if !strings.Contains(pathEnv, dir) {
		fmt.Printf("\n%s%sNote:%s %s is not in your current PATH.\n", ui.Bold, ui.FgYellow, ui.Reset, dir)
		fmt.Printf("Add it to your shell config (~/.zshrc or ~/.bashrc):\n")
		fmt.Printf("  export PATH=\"%s:$PATH\"\n", dir)
	} else {
		fmt.Printf("%s%s'duplex' is now globally accessible from any terminal!%s\n", ui.Bold, ui.FgCyan, ui.Reset)
	}
}
