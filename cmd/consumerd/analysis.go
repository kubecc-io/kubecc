package main

import (
	"strings"

	"github.com/cobalt77/kubecc/internal/lll"
)

// analyze error messages to find oddities and potential bugs
func analyzeErrors(msg string) error {
	lll.Info("Analyzing error:")
	if strings.Contains(msg, "collect2") || strings.Contains(msg, "undefined reference") {
		lll.DPanic(lll.Red.Add("*** BUG! Linker invoked on remote agent ***"))
	}
	return nil
}
