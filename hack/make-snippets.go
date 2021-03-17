///usr/bin/true; exec /usr/bin/env go run "$0" "$@"
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

type snippets struct {
	Items map[string]snippet `json:",inline"`
}

type snippet struct {
	Scope       string `json:"scope,omitempty"`
	Prefix      string `json:"prefix"`
	Body        string `json:"body"`
	Description string `json:"description,omitempty"`
}

func main() {
	here, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(here, "hack/snippets.yaml")); os.IsNotExist(err) {
			here = filepath.Dir(here)
		}
		break
	}
	data, err := os.ReadFile(filepath.Join(here, "hack/snippets.yaml"))
	if err != nil {
		log.Fatal(err)
	}
	items := map[string]snippet{}
	err = yaml.Unmarshal(data, &items)
	if err != nil {
		log.Fatal(err)
	}
	if len(items) == 0 {
		log.Fatal("No snippets found")
	}
	for name, sn := range items {
		if sn.Scope == "" {
			sn.Scope = "go"
		}
		if sn.Description == "" {
			sn.Description = name
		}
		sn.Body = strings.ReplaceAll(sn.Body, "\t", `\t`)
		sn.Body = strings.ReplaceAll(sn.Body, "\n", `\n`)
		items[name] = sn
	}
	jsonData, err := json.MarshalIndent(items, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	os.MkdirAll("../.vscode", 0o775)
	err = os.WriteFile("../.vscode/kubecc.code-snippets", jsonData, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Generated %d snippets\n", len(items))
}
