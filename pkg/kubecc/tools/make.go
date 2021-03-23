package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func pathToMake() string {
	locations := []string{"/usr/bin/make", "/bin/make"}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	fmt.Fprintf(os.Stderr, "make not found (checked %s)", strings.Join(locations, ", "))
	os.Exit(1)
	return ""
}

var MakeCmd = &cobra.Command{
	Use:   "make",
	Short: "A wrapper around make",
	Long: `A wrapper around make that will allow all sub-processes to be grouped for tracing purposes.
This tool will automatically be run if the kubecc binary is invoked with the name 'make'.`,
	Run: func(_ *cobra.Command, args []string) {
		cmd := exec.Command(pathToMake(), args...)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("KUBECC_MAKE_PID=%d", os.Getpid()))
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running make: %s\n", err)
			os.Exit(1)
		}
		if err := cmd.Wait(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	},
}
