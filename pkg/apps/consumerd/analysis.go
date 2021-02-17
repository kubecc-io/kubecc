package consumerd

import (
	"errors"
	"strings"
)

// analyze error messages to find oddities and potential bugs.
func analyzeErrors(msg string) error {
	if strings.Contains(msg, "collect2") || strings.Contains(msg, "undefined reference") {
		return errors.New("*** BUG! Linker invoked on remote agent ***")
	}
	return nil
}
