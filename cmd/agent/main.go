package main

import (
	"os"
	"path"
)

func main() {
	InitConfig()
	switch path.Base(os.Args[0]) {
	case "agent":
		Execute()
	default:
		// Assume we are symlinked to a compiler
		StartAgentOrDispatch()
	}
}
