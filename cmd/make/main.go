package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("/usr/bin/make", os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("KUBECC_MAKE_PID=%d", os.Getpid()))
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error running make: %s\n", err)
		os.Exit(1)
	}
	cmd.Wait()
}
