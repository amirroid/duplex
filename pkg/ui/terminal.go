package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI color and formatting codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Underline = "\033[4m"

	FgBlack   = "\033[30m"
	FgRed     = "\033[31m"
	FgGreen   = "\033[32m"
	FgYellow  = "\033[33m"
	FgBlue    = "\033[34m"
	FgMagenta = "\033[35m"
	FgCyan    = "\033[36m"
	FgWhite   = "\033[37m"

	BgBlue    = "\033[44m"
	BgCyan    = "\033[46m"
	BgGray    = "\033[100m"

	ClearScreen = "\033[2J\033[H"
	HideCursor  = "\033[?25l"
	ShowCursor  = "\033[?25h"
)

// Key types
type Key int

const (
	KeyUp Key = iota
	KeyDown
	KeyLeft
	KeyRight
	KeyEnter
	KeyEsc
	KeyQuit
	KeyOther
)

// ReadKey reads a single keypress in raw terminal mode.
func ReadKey() (Key, rune, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return KeyOther, 0, err
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 8)
	n, err := os.Stdin.Read(buf)
	if err != nil {
		return KeyOther, 0, err
	}

	if n == 1 {
		b := buf[0]
		switch b {
		case 3: // Ctrl+C
			return KeyQuit, 0, nil
		case 13, 10: // Enter
			return KeyEnter, 0, nil
		case 27: // Esc
			return KeyEsc, 0, nil
		case 'q', 'Q':
			return KeyQuit, 'q', nil
		case 'j', 'J':
			return KeyDown, 'j', nil
		case 'k', 'K':
			return KeyUp, 'k', nil
		default:
			return KeyOther, rune(b), nil
		}
	}

	// Escape sequences (arrows)
	if n >= 3 && buf[0] == 27 && buf[1] == '[' {
		switch buf[2] {
		case 'A': // Up
			return KeyUp, 0, nil
		case 'B': // Down
			return KeyDown, 0, nil
		case 'C': // Right
			return KeyRight, 0, nil
		case 'D': // Left
			return KeyLeft, 0, nil
		}
	}

	return KeyOther, 0, nil
}

// MenuItem represents an entry in an interactive select list.
type MenuItem struct {
	ID          string
	Label       string
	Description string
}

// SelectMenu presents an interactive keyboard-navigable list.
func SelectMenu(title string, items []MenuItem, defaultIndex int) (int, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Non-interactive fallback
		return fallbackSelect(title, items, defaultIndex)
	}

	selected := defaultIndex
	if selected < 0 || selected >= len(items) {
		selected = 0
	}

	// Hide cursor during navigation
	fmt.Print(HideCursor)
	defer fmt.Print(ShowCursor)

	firstRender := true
	linesRendered := 0

	for {
		// Clear previously drawn menu
		if !firstRender && linesRendered > 0 {
			// Move cursor up linesRendered times and clear
			fmt.Printf("\033[%dA\033[J", linesRendered)
		}
		firstRender = false

		linesRendered = 0
		if title != "" {
			fmt.Printf("%s%s%s\r\n", Bold, title, Reset)
			linesRendered++
		}

		for i, item := range items {
			if i == selected {
				desc := ""
				if item.Description != "" {
					desc = fmt.Sprintf(" %s(%s)%s", Dim, item.Description, Reset)
				}
				fmt.Printf("  %s%s❯ %s%s%s\r\n", Bold, FgCyan, item.Label, Reset, desc)
			} else {
				desc := ""
				if item.Description != "" {
					desc = fmt.Sprintf(" %s(%s)%s", Dim, item.Description, Reset)
				}
				fmt.Printf("    %s%s%s\r\n", item.Label, Reset, desc)
			}
			linesRendered++
		}

		key, _, err := ReadKey()
		if err != nil {
			return -1, err
		}

		switch key {
		case KeyUp:
			if selected > 0 {
				selected--
			} else {
				selected = len(items) - 1
			}
		case KeyDown:
			if selected < len(items)-1 {
				selected++
			} else {
				selected = 0
			}
		case KeyEnter:
			return selected, nil
		case KeyEsc, KeyQuit:
			return -1, nil
		}
	}
}

// fallbackSelect provides line-based selection for non-TTY environments.
func fallbackSelect(title string, items []MenuItem, defaultIndex int) (int, error) {
	if title != "" {
		fmt.Println(title)
	}
	for i, item := range items {
		fmt.Printf("  [%d] %s\n", i+1, item.Label)
	}
	fmt.Printf("Select [1-%d] (default %d): ", len(items), defaultIndex+1)

	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	if err != nil {
		return -1, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultIndex, nil
	}
	var choice int
	if _, err := fmt.Sscanf(text, "%d", &choice); err == nil && choice >= 1 && choice <= len(items) {
		return choice - 1, nil
	}
	return defaultIndex, nil
}

// PromptString reads a line of input with a prompt.
func PromptString(prompt string) string {
	fmt.Printf("%s%s%s ", Bold, prompt, Reset)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	clean := strings.TrimSpace(line)
	// Strip surrounding quotes or escaped spaces from macOS Finder drag & drop
	clean = strings.Trim(clean, `"'`)
	return clean
}

// WaitEnter pauses until the user presses Enter.
func WaitEnter(prompt string) {
	if prompt == "" {
		prompt = "Press [Enter] to continue..."
	}
	fmt.Printf("\r\n%s%s%s%s", Bold, FgYellow, prompt, Reset)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		for {
			key, _, _ := ReadKey()
			if key == KeyEnter || key == KeyQuit || key == KeyEsc {
				break
			}
		}
		fmt.Println()
	} else {
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')
	}
}

// Confirm asks a yes/no question.
func Confirm(question string, defaultYes bool) bool {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	fmt.Printf("%s%s %s%s ", Bold, question, hint, Reset)

	if term.IsTerminal(int(os.Stdin.Fd())) {
		for {
			key, r, _ := ReadKey()
			if key == KeyEnter {
				fmt.Println()
				return defaultYes
			}
			if r == 'y' || r == 'Y' {
				fmt.Println("y")
				return true
			}
			if r == 'n' || r == 'N' || key == KeyEsc || key == KeyQuit {
				fmt.Println("n")
				return false
			}
		}
	} else {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(strings.ToLower(text))
		if text == "" {
			return defaultYes
		}
		return text == "y" || text == "yes"
	}
}
