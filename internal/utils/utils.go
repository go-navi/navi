package utils

import (
	"fmt"
	"os"
	"strings"
)

// IsRunningInTestMode checks if the application is running in test environment
func IsRunningInTestMode() bool {
	return os.Getenv("NAVI_TEST_MODE") == "1"
}

// FormatDurationValue formats time values removing unnecessary decimals
func FormatDurationValue(timeValue float64) string {
	if timeValue == float64(int(timeValue)) {
		return fmt.Sprintf("%g", timeValue)
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", timeValue), ".0")
}

// AddQuotesToArgsWithSpaces wraps command arguments containing spaces with quotes
func AddQuotesToArgsWithSpaces(commandArgs []string) []string {
	quotedArgs := make([]string, len(commandArgs))
	for index, argument := range commandArgs {
		if strings.Contains(argument, " ") && !strings.HasPrefix(argument, "\"") {
			quotedArgs[index] = fmt.Sprintf("\"%s\"", argument)
		} else {
			quotedArgs[index] = argument
		}
	}
	return quotedArgs
}

// SliceContainsValue checks if a value exists within a slice or array
func SliceContainsValue[T comparable](collection []T, searchValue T) bool {
	for _, element := range collection {
		if element == searchValue {
			return true
		}
	}
	return false
}

// EnsureSuffix adds a suffix to a string if not already present
// Handles partial suffixes intelligently
func EnsureSuffix(text string, suffixToAdd string) string {
	if strings.HasSuffix(text, suffixToAdd) {
		return text
	}

	for charPos := 1; charPos < len(suffixToAdd); charPos++ {
		partialSuffix := suffixToAdd[:charPos]
		if strings.HasSuffix(text, partialSuffix) {
			return text + suffixToAdd[charPos:]
		}
	}

	return text + suffixToAdd
}

// IsNumber checks if a value is a numeric type
func IsNumber(val any) bool {
	switch val.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	}
	return false
}

// IsBool checks if a value is a boolean type
func IsBool(val any) bool {
	switch val.(type) {
	case bool:
		return true
	}
	return false
}

// ToFloat64 converts an interface value to float64 if it's a numeric type
func ToFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

// ToInt converts an interface value to int if it's a integer type
func ToInt(val any) (int, bool) {
	switch v := val.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	}
	return 0, false
}

// ReplaceAtIndex replaces characters at the specified index, preserving the original string length.
func ReplaceAtIndex(baseStr string, substr string, index int) string {
	if index < 0 {
		index = 0
	}

	if index >= len(baseStr) || substr == "" {
		return baseStr
	}

	baseRunes := []rune(baseStr)
	subRunes := []rune(substr)

	for i, ch := range subRunes {
		pos := index + i
		if pos >= len(baseRunes) {
			break
		}
		baseRunes[pos] = ch
	}

	return string(baseRunes)
}
