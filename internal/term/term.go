package term

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-navi/navi/internal/utils"
	xTerm "golang.org/x/term"
)

// Key codes for terminal input handling
const (
	KEY_UP = iota
	KEY_DOWN
	KEY_HOME
	KEY_END
	KEY_SHIFT_TAB
	KEY_PAGE_UP
	KEY_PAGE_DOWN
	KEY_TAB
	KEY_ENTER
	KEY_ESC
	KEY_CTRL_C
	KEY_CTRL_SPACE
	KEY_BACKSPACE

	// test key codes
	TEST_SNAPSHOT
)

// TermUI handles terminal UI operations with state management
type TermUI struct {
	colorStack   []string // Stack to manage color context
	cursorX      int      // Current cursor X position
	cursorY      int      // Current cursor Y position
	limitWidth   int      // Width limit for text output
	limitBuffer  string   // Buffer for limited text
	inLimit      bool     // Whether limits are active
	inWrapLimit  bool     // Whether multiline limits are active
	storedBuffer string   // Stored output for later retrieval
	store        bool     // Whether to store output
}

// originalTermState stores terminal state for restoration
var originalTermState *xTerm.State

// ansiCodePattern matches ANSI escape sequences
var ansiCodePattern = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

// testInputSequence holds command sequence for test mode
var testInputSequence []string

// testCursorPos tracks current position in test terminal
var testCursorPos struct{ X, Y int }

// TestScreen represents terminal screen in test mode
var TestScreen []string

// resizeTestScreen adjusts the test terminal dimensions
func resizeTestScreen(width, height int) {
	if height <= 0 {
		TestScreen = nil
		return
	}

	currentHeight := len(TestScreen)

	if height == currentHeight {
		return
	} else if height > currentHeight {
		for i := currentHeight; i < height; i++ {
			TestScreen = append(TestScreen, repeatStr(" ", width))
		}
	} else if height < currentHeight {
		TestScreen = TestScreen[:height]
	}
}

// removeAnsiCodes removes ANSI escape sequences from a string
func RemoveAnsiCodes(str string) string {
	return ansiCodePattern.ReplaceAllString(str, "")
}

// CountPrintableChars counts visible (non-ANSI) characters in string
func CountPrintableChars(s string) int {
	str := strings.ReplaceAll(s, "\033[1C", " ")
	return utf8.RuneCountInString(RemoveAnsiCodes(str))
}

// SplitPreservingAnsi splits string preserving ANSI escape codes
func SplitPreservingAnsi(s string) []string {
	matches := ansiCodePattern.FindAllStringIndex(s, -1)

	if len(matches) == 0 {
		return []string{s}
	}

	var result []string
	lastIndex := 0

	for _, match := range matches {
		start, end := match[0], match[1]

		if start > lastIndex {
			result = append(result, s[lastIndex:start])
		}

		result = append(result, s[start:end])
		lastIndex = end
	}

	if lastIndex < len(s) {
		result = append(result, s[lastIndex:])
	}

	return result
}

// clipText shortens text with ellipsis if exceeds maxLength
func clipText(text string, maxLength int) string {
	return formatTextBlock(text, maxLength, true)
}

// wrapText wraps text into multiple lines at maxLength
func wrapText(text string, maxLength int) string {
	return formatTextBlock(text, maxLength, false)
}

// formatTextBlock implements text truncation or line breaking
func formatTextBlock(text string, maxLength int, truncate bool) string {
	if maxLength <= 0 || text == "" {
		return ""
	}

	if CountPrintableChars(text) <= maxLength {
		return text
	}

	textParts := SplitPreservingAnsi(text)
	output := ""
	length := 0

	if truncate {
		length = 3 // 3 for ellipsis
	}

	for _, part := range textParts {
		if ansiCodePattern.MatchString(part) {
			if part == "\033[1C" {
				if length <= maxLength {
					length++

					if length > maxLength {
						if truncate {
							output = output[:len(output)-1] + "..."
						} else {
							output = output[:len(output)-1] + "\n"
							length = 0
						}
					}
				}
			}

			output += part
		} else if length <= maxLength {
			output += part
			length += utf8.RuneCountInString(part)

			if length > maxLength {
				diff := length - maxLength

				if diff > len(output) {
					return output
				}

				if truncate {
					output = output[:len(output)-diff] + "..."
				} else {
					before := output[:len(output)-diff]
					after := output[len(output)-diff:]
					output = before + "\n"

					// Handle the remaining text after line break
					currentLen := 0
					result := ""

					for _, c := range after {
						result += string(c)
						currentLen++

						if currentLen >= maxLength {
							result += "\n"
							currentLen = 0
						}
					}

					output += result
					length = currentLen
				}
			}
		}
	}

	return output
}

// repeatStr repeats a string safely, avoiding negative counts
func repeatStr(s string, count int) string {
	if count <= 0 {
		return ""
	}

	return strings.Repeat(s, count)
}

// ReadInput gets and interprets keyboard input
func ReadInput() (inputKey int, inputText string, err error) {
	if utils.IsRunningInTestMode() {
		if len(testInputSequence) == 0 {
			return 0, "", fmt.Errorf("No more test commands available")
		}

		nextCommand := testInputSequence[0]
		testInputSequence = testInputSequence[1:]

		inputKey, err := strconv.Atoi(nextCommand)
		if err == nil {
			return inputKey, "", nil
		} else {
			return -1, nextCommand, nil
		}
	}

	var inputBytes [256]byte
	var inputLength int

	inputLength, err = os.Stdin.Read(inputBytes[:])
	if err != nil {
		return 0, "", err
	}

	switch {
	case inputLength >= 3 && inputBytes[0] == 27 && inputBytes[1] == 91:
		switch {
		case inputLength == 3 && inputBytes[2] == 65:
			return KEY_UP, string(inputBytes[:inputLength]), nil

		case inputLength == 3 && inputBytes[2] == 66:
			return KEY_DOWN, string(inputBytes[:inputLength]), nil

		case inputLength == 3 && inputBytes[2] == 72:
			return KEY_HOME, string(inputBytes[:inputLength]), nil

		case inputLength == 3 && inputBytes[2] == 70:
			return KEY_END, string(inputBytes[:inputLength]), nil

		case inputLength == 3 && inputBytes[2] == 90:
			return KEY_SHIFT_TAB, string(inputBytes[:inputLength]), nil

		case inputLength == 4 && inputBytes[3] == 126 && inputBytes[2] == 53:
			return KEY_PAGE_UP, string(inputBytes[:inputLength]), nil

		case inputLength == 4 && inputBytes[3] == 126 && inputBytes[2] == 54:
			return KEY_PAGE_DOWN, string(inputBytes[:inputLength]), nil
		}

	case inputLength == 1 && inputBytes[0] == 27:
		return KEY_ESC, string(inputBytes[:inputLength]), nil

	case inputLength == 1 && inputBytes[0] == 13:
		return KEY_ENTER, string(inputBytes[:inputLength]), nil

	case inputLength == 1 && inputBytes[0] == 3:
		return KEY_CTRL_C, string(inputBytes[:inputLength]), nil

	case inputLength == 1 && inputBytes[0] == 0:
		return KEY_CTRL_SPACE, string(inputBytes[:inputLength]), nil

	case inputLength == 1 && inputBytes[0] == 9:
		return KEY_TAB, string(inputBytes[:inputLength]), nil

	case inputLength == 1 && (inputBytes[0] == 127 || inputBytes[0] == 8):
		return KEY_BACKSPACE, string(inputBytes[:inputLength]), nil
	}

	return -1, string(inputBytes[:inputLength]), nil
}

// renderTestOutput handles output in test mode
func renderTestOutput(s string) {
	parts := strings.Split(s, "\033[1C")

	for i, part := range parts {
		if i > 0 {
			testCursorPos.X++
		}

		if len(part) > 0 {
			strippedPart := RemoveAnsiCodes(part)

			if testCursorPos.Y >= 0 && testCursorPos.Y < len(TestScreen) {
				line := TestScreen[testCursorPos.Y]

				if testCursorPos.X < len(line) {
					TestScreen[testCursorPos.Y] = utils.ReplaceAtIndex(line, strippedPart, testCursorPos.X)
					testCursorPos.X += utf8.RuneCountInString(strippedPart)
				}
			}
		}
	}
}

// writeOutput writes to standard output with test mode support
func writeOutput(s string) {
	if utils.IsRunningInTestMode() {
		renderTestOutput(s)
	} else {
		os.Stdout.Write([]byte(s))
	}
}

// HideCursor hides the terminal cursor
func HideCursor() {
	if utils.IsRunningInTestMode() {
		return
	}

	writeOutput("\033[?25l")
}

// ShowCursor makes the terminal cursor visible
func ShowCursor() {
	if utils.IsRunningInTestMode() {
		return
	}

	writeOutput("\033[?25h")
}

// ClearScreen erases the terminal content
func ClearScreen() {
	if utils.IsRunningInTestMode() {
		for i := 0; i < len(TestScreen); i++ {
			TestScreen[i] = repeatStr(" ", len(TestScreen[i]))
		}

		return
	}

	writeOutput("\033[2J")
}

// EnableScreenBuffer activates alternate screen buffer
func EnableScreenBuffer() {
	if utils.IsRunningInTestMode() {
		return
	}

	writeOutput("\033[?1049h")
}

// DisableScreenBuffer returns from alternate screen buffer
func DisableScreenBuffer() {
	if utils.IsRunningInTestMode() {
		return
	}

	writeOutput("\033[?1049l")
}

// MakeRaw switches terminal to raw input mode
func MakeRaw() error {
	if utils.IsRunningInTestMode() {
		return nil
	}

	var err error
	originalTermState, err = xTerm.MakeRaw(int(os.Stdin.Fd()))
	return err
}

// Restore returns terminal to initial state
func Restore() error {
	if utils.IsRunningInTestMode() {
		return nil
	}

	return xTerm.Restore(int(os.Stdin.Fd()), originalTermState)
}

// GetTermSize returns terminal width and height
func GetTermSize() (int, int) {
	if utils.IsRunningInTestMode() {
		width, _ := strconv.Atoi(os.Getenv("NAVI_TEST_CLI_TERM_WIDTH"))
		height, _ := strconv.Atoi(os.Getenv("NAVI_TEST_CLI_TERM_HEIGHT"))
		resizeTestScreen(width, height)
		return width, height
	}

	width, height, err := xTerm.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, height = 83, 24
	}

	return width, height
}

// renderOutput handles output with support for storage
func (termUI *TermUI) renderOutput(s string) {
	if termUI.store {
		termUI.storedBuffer += s
		return
	}

	writeOutput(s)
}

// activeColor returns the current color from stack
func (termUI *TermUI) activeColor() string {
	if len(termUI.colorStack) == 0 {
		return ""
	}

	return termUI.colorStack[len(termUI.colorStack)-1]
}

// applyColor adds color to stack and outputs color code
func (termUI *TermUI) applyColor(code string) *TermUI {
	termUI.colorStack = append(termUI.colorStack, code)

	if termUI.inLimit {
		termUI.limitBuffer += "\033[" + code + "m"
	} else {
		termUI.renderOutput("\033[" + code + "m")
	}

	return termUI
}

// Cursor positions cursor at specific coordinates
func (termUI *TermUI) Cursor(x, y int) *TermUI {
	termUI.cursorX, termUI.cursorY = x, y

	if termUI.cursorX <= 1 {
		termUI.cursorX = 1
	}

	if termUI.cursorY <= 1 {
		termUI.cursorY = 1
	}

	if termUI.cursorX <= 1 && termUI.cursorY <= 1 {
		termUI.renderOutput("\033[H")
	} else {
		termUI.renderOutput("\033[" + strconv.Itoa(termUI.cursorY) + ";" + strconv.Itoa(termUI.cursorX) + "H")
	}

	testCursorPos.X, testCursorPos.Y = termUI.cursorX-1, termUI.cursorY-1
	return termUI
}

// ClearLine erases from cursor to end of line
func (termUI *TermUI) ClearLine() *TermUI {
	if utils.IsRunningInTestMode() {
		if testCursorPos.Y < len(TestScreen) {
			line := TestScreen[testCursorPos.Y]
			substr := repeatStr(" ", len(line)-testCursorPos.X)
			TestScreen[testCursorPos.Y] = utils.ReplaceAtIndex(line, substr, testCursorPos.X)
		}

		return termUI
	}

	termUI.renderOutput("\033[K")
	return termUI
}

// ClipToWidth enables single-line text width limiting
func (termUI *TermUI) ClipToWidth(width int) *TermUI {
	termUI.inLimit = true
	termUI.inWrapLimit = false
	termUI.limitWidth = width
	termUI.limitBuffer = ""
	return termUI
}

// WrapToWidth enables multi-line text width limiting
func (termUI *TermUI) WrapToWidth(width int) *TermUI {
	termUI.inLimit = true
	termUI.inWrapLimit = true
	termUI.limitWidth = width
	termUI.limitBuffer = ""
	return termUI
}

// ApplyTextLimits processes and outputs limited buffer
func (termUI *TermUI) ApplyTextLimits() *TermUI {
	text := ""

	if termUI.inWrapLimit {
		text = wrapText(termUI.limitBuffer, termUI.limitWidth)
	} else {
		text = clipText(termUI.limitBuffer, termUI.limitWidth)
	}

	termUI.renderOutput(text)
	termUI.inLimit = false
	termUI.inWrapLimit = false
	termUI.limitWidth = 0
	termUI.limitBuffer = ""
	return termUI
}

// GetFormattedText returns processed limited buffer
func (termUI *TermUI) GetFormattedText() string {
	text := ""

	if termUI.inWrapLimit {
		text = wrapText(termUI.limitBuffer, termUI.limitWidth)
	} else {
		text = clipText(termUI.limitBuffer, termUI.limitWidth)
	}

	termUI.inLimit = false
	termUI.inWrapLimit = false
	termUI.limitWidth = 0
	termUI.limitBuffer = ""
	return text
}

// MoveCursorRight moves cursor one position right
func (termUI *TermUI) MoveCursorRight() *TermUI {
	if termUI.inLimit {
		termUI.limitBuffer += "\033[1C"
		return termUI
	}

	termUI.renderOutput("\033[1C")
	return termUI
}

// Store enables output storage for later retrieval
func (termUI *TermUI) Store() *TermUI {
	termUI.store = true
	termUI.storedBuffer = ""
	return termUI
}

// GetStored returns stored output and disables storage
func (termUI *TermUI) GetStored() string {
	termUI.store = false
	return termUI.storedBuffer
}

// Print outputs text horizontally
func (termUI *TermUI) Print(text any) *TermUI {
	termUI.renderText(text, false)
	return termUI
}

// PrintVertical outputs text vertically
func (termUI *TermUI) PrintVertical(text any) *TermUI {
	termUI.renderText(text, true)
	return termUI
}

// renderText handles common print functionality
func (termUI *TermUI) renderText(text any, vertical bool) *TermUI {
	parsedText := fmt.Sprintf("%v", text)

	if termUI.inLimit {
		termUI.limitBuffer += parsedText
		return termUI
	}

	if !vertical {
		termUI.renderOutput(parsedText)
		return termUI
	}

	curCursorY := termUI.cursorY

	for i, char := range parsedText {
		termUI.Cursor(termUI.cursorX, curCursorY+i)
		currColor := termUI.activeColor()

		if currColor != "" {
			termUI.renderOutput("\033[" + currColor + "m" + string(char) + "\033[0m")
		} else {
			termUI.renderOutput(string(char))
		}
	}

	return termUI
}

// Repeat outputs text multiple times horizontally
func (termUI *TermUI) Repeat(text any, count int) *TermUI {
	termUI.renderRepeated(text, count, false)
	return termUI
}

// RepeatVertical outputs text multiple times vertically
func (termUI *TermUI) RepeatVertical(text any, count int) *TermUI {
	termUI.renderRepeated(text, count, true)
	return termUI
}

// renderRepeated implements repeated output functionality
func (termUI *TermUI) renderRepeated(text any, count int, vertical bool) *TermUI {
	parsedText := fmt.Sprintf("%v", text)

	if termUI.inLimit {
		termUI.limitBuffer += repeatStr(parsedText, count)
		return termUI
	}

	if count > 0 {
		if !vertical {
			termUI.renderOutput(repeatStr(parsedText, count))
			return termUI
		}

		curCursorY := termUI.cursorY

		for i := range count {
			termUI.Cursor(termUI.cursorX, curCursorY+i)
			currColor := termUI.activeColor()

			if currColor != "" {
				termUI.renderOutput("\033[" + currColor + "m" + parsedText + "\033[0m")
			} else {
				termUI.renderOutput(parsedText)
			}
		}

		return termUI
	}

	return termUI
}

// Green sets text color to green
func (termUI *TermUI) Green() *TermUI {
	termUI.applyColor("32")
	return termUI
}

// Yellow sets text color to yellow
func (termUI *TermUI) Yellow() *TermUI {
	termUI.applyColor("33")
	return termUI
}

// Cyan sets text color to cyan
func (termUI *TermUI) Cyan() *TermUI {
	termUI.applyColor("36")
	return termUI
}

// DarkGray sets text color to dark gray
func (termUI *TermUI) DarkGray() *TermUI {
	termUI.applyColor("38;5;240")
	return termUI
}

// BoldGreen sets text to bold green
func (termUI *TermUI) BoldGreen() *TermUI {
	termUI.applyColor("1;32")
	return termUI
}

// BoldYellow sets text to bold yellow
func (termUI *TermUI) BoldYellow() *TermUI {
	termUI.applyColor("1;33")
	return termUI
}

// ItalicGreen sets text to italic green
func (termUI *TermUI) ItalicGreen() *TermUI {
	termUI.applyColor("3;32")
	return termUI
}

// ReverseVideo inverts text and background colors
func (termUI *TermUI) ReverseVideo() *TermUI {
	termUI.applyColor("7")
	return termUI
}

// PrevColor restores previous color from stack
func (termUI *TermUI) PrevColor() *TermUI {
	if len(termUI.colorStack) > 0 {
		termUI.colorStack = termUI.colorStack[:len(termUI.colorStack)-1]

		if termUI.inLimit {
			termUI.limitBuffer += "\033[0m"
		} else {
			termUI.renderOutput("\033[0m")
		}

		if len(termUI.colorStack) > 0 {
			lastColor := termUI.colorStack[len(termUI.colorStack)-1]

			if termUI.inLimit {
				termUI.limitBuffer += "\033[" + lastColor + "m"
			} else {
				termUI.renderOutput("\033[" + lastColor + "m")
			}
		}
	}

	return termUI
}

// ClearRect clears a rectangular area of the terminal
func (termUI *TermUI) ClearRect(x, y, width, height int) *TermUI {
	for i := 0; i < height; i++ {
		termUI.Cursor(x, y+i)
		termUI.Repeat(" ", width)
	}

	return termUI
}

// DrawInputModal creates a bordered modal dialog
func (termUI *TermUI) DrawInputModal(title string, modalX, modalY, modalWidth, modalHeight int) *TermUI {
	termUI.ClearRect(modalX-3, modalY-1, modalWidth+6, modalHeight+2)

	for i := 0; i < modalHeight; i++ {
		termUI.Cursor(modalX, modalY+i)

		if i == 0 {
			termUI.Print("┌").Repeat("─", modalWidth-2).Print("┐")
		} else if i == modalHeight-1 {
			termUI.Print("└").Repeat("─", modalWidth-2).Print("┘")
		} else {
			termUI.Print("│").Repeat(" ", modalWidth-2).Print("│")
		}
	}

	if strings.TrimSpace(title) != "" {
		title = " " + strings.TrimSpace(title) + " "
		titleX := modalX + (modalWidth / 2) - (CountPrintableChars(title) / 2)

		if titleX < modalX {
			titleX = modalX
		}

		termUI.
			Cursor(titleX, modalY).
			Print(title)
	}

	hint := " [Enter] Confirm   [Esc] Cancel "

	if runtime.GOOS == "darwin" { // MacOS
		hint = " [Return] Confirm   [Esc] Cancel "
	}

	hintX := modalX + (modalWidth / 2) - (CountPrintableChars(hint) / 2)

	if hintX < modalX {
		hintX = modalX
	}

	termUI.
		Cursor(hintX, modalY+modalHeight-1).
		Print(hint)

	return termUI
}

// NewTermUI creates a new terminal UI instance
func NewTermUI() *TermUI {
	return &TermUI{}
}

func init() {
	if utils.IsRunningInTestMode() {
		commandStr := os.Getenv("NAVI_TEST_CLI_COMMANDS")
		testInputSequence = strings.Split(commandStr, "|")
	}
}
