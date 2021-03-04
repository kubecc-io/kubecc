package main

import (
	"fmt"
	"log"
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
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running make: %s\n", err)
		os.Exit(1)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}
