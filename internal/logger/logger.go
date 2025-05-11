package logger

import (
	"fmt"
	"strings"

	"github.com/go-navi/navi/internal/utils"
)

// ANSI color codes for terminal output prefixes
var AnsiColors = []string{
	"\033[0;36m", // Cyan
	"\033[0;90m", // Gray
	"\033[0;32m", // Green
	"\033[0;35m", // Magenta
	"\033[0;34m", // Blue
	"\033[0;33m", // Yellow
	"\033[0;95m", // Bright Magenta
}

// Tracks current color in the rotation
var currentColorIndex = 0

// ANSI text color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[0;33m"
)

// GetColorizedPrefix formats text with colors for terminal output
func GetColorizedPrefix(id, name, color string, showId bool) string {
	if !showId {
		id = ""
	} else {
		id = id + " "
	}

	prefix := id + name + " ⟫"
	if utils.IsRunningInTestMode() {
		return prefix
	}
	return color + prefix + colorReset
}

// GetLogPrefixColor returns the next color in rotation
func GetLogPrefixColor() string {
	if currentColorIndex >= len(AnsiColors) {
		currentColorIndex = 0
	}

	color := AnsiColors[currentColorIndex]
	currentColorIndex++
	return color
}

// formatMessage formats log messages with appropriate color and prefix
func formatMessage(prefix, msgType, color, format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)

	if utils.IsRunningInTestMode() {
		if prefix == "" {
			return fmt.Sprintf("%s: %s", msgType, msg)
		}
		return fmt.Sprintf("%s %s: %s", prefix, msgType, msg)
	}

	if prefix == "" {
		return fmt.Sprintf("%s%s: %s%s", color, msgType, msg, colorReset)
	}
	return fmt.Sprintf("%s %s%s: %s%s", prefix, color, msgType, msg, colorReset)
}

// print outputs a message with optional prefix and formatting
func print(prefix, msgType, color, format string, args ...any) {
	if strings.TrimSpace(prefix) == "" {
		prefix = ""
	}

	fmt.Println(formatMessage(prefix, msgType, color, format, args...))
}

// Error prints formatted error message in red
func Error(format string, args ...any) {
	print("", "ERROR", colorRed, format, args...)
}

// ErrorWithPrefix prints error with a custom prefix
func ErrorWithPrefix(prefix string, format string, args ...any) {
	print(prefix, "ERROR", colorRed, format, args...)
}

// Warn prints warning message in yellow
func Warn(format string, args ...any) {
	print("", "WARNING", colorYellow, format, args...)
}

// WarnWithPrefix prints warning with a custom prefix
func WarnWithPrefix(prefix string, format string, args ...any) {
	print(prefix, "WARNING", colorYellow, format, args...)
}

// Info prints information message in green
func Info(format string, args ...any) {
	if utils.IsRunningInTestMode() {
		fmt.Printf(format+"\n", args...)
		return
	}

	fmt.Printf("%s%s%s\n", colorGreen, fmt.Sprintf(format, args...), colorReset)
}

// InfoWithPrefix prints info with a custom prefix
func InfoWithPrefix(prefix string, format string, args ...any) {
	if strings.TrimSpace(prefix) == "" {
		Info(format, args...)
		return
	}

	if utils.IsRunningInTestMode() {
		fmt.Printf("%s "+format+"\n", append([]any{prefix}, args...)...)
		return
	}

	fmt.Printf("%s %s%s%s\n", prefix, colorGreen, fmt.Sprintf(format, args...), colorReset)
}
