package print

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// PrintPDF sends a PDF document to the default system printer.
func PrintPDF(filePath string) error {
	switch runtime.GOOS {
	case "darwin", "linux":
		// Check for lpr or lp
		if _, err := exec.LookPath("lpr"); err == nil {
			cmd := exec.Command("lpr", filePath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("lpr error: %s (%w)", strings.TrimSpace(string(out)), err)
			}
			return nil
		}
		if _, err := exec.LookPath("lp"); err == nil {
			cmd := exec.Command("lp", filePath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("lp error: %s (%w)", strings.TrimSpace(string(out)), err)
			}
			return nil
		}
		return fmt.Errorf("neither 'lpr' nor 'lp' command was found on this system")

	case "windows":
		// Use PowerShell Start-Process with Print verb
		cmd := exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf(`Start-Process -FilePath "%s" -Verb Print`, filePath))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("windows print error: %s (%w)", strings.TrimSpace(string(out)), err)
		}
		return nil

	default:
		return fmt.Errorf("printing is not supported directly on %s", runtime.GOOS)
	}
}

// OpenPDF opens the PDF in the operating system's default viewer.
func OpenPDF(filePath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", filePath)
	case "linux":
		cmd = exec.Command("xdg-open", filePath)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", filePath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// OpenFolder opens the specified directory in the system's file manager (Finder / Explorer).
func OpenFolder(folderPath string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", folderPath)
	case "linux":
		cmd = exec.Command("xdg-open", folderPath)
	case "windows":
		cmd = exec.Command("explorer.exe", folderPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// GetDefaultPrinter attempts to retrieve the name of the system's default printer.
func GetDefaultPrinter() string {
	switch runtime.GOOS {
	case "darwin", "linux":
		out, err := exec.Command("lpstat", "-d").Output()
		if err == nil {
			str := strings.TrimSpace(string(out))
			// e.g. "system default destination: HP_LaserJet"
			if idx := strings.Index(str, ":"); idx != -1 {
				return strings.TrimSpace(str[idx+1:])
			}
			return str
		}
	}
	return "System Default Printer"
}
