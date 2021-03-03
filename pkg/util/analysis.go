package util

import (
	"errors"
	"strings"
)

// AnalyzeErrors analyzes error messages to find oddities and potential bugs.
func AnalyzeErrors(msg string) error {
	if strings.Contains(msg, "collect2") || strings.Contains(msg, "undefined reference") {
		return errors.New("*** BUG! Linker invoked on remote agent ***")
	}
	return nil
}
