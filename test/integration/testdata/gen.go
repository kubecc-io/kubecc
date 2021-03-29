///usr/bin/true; exec /usr/bin/env go run "$0" "$@"
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubecc-io/kubecc/pkg/templates"
	"github.com/kubecc-io/kubecc/pkg/util"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: ./gen.go <filepath>")
		os.Exit(1)
	}
	abs, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Println(util.OSFS)
	data, err := templates.Load(util.OSFS, abs, struct{}{})
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Println(string(data))
}
