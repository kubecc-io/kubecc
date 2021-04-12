/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package commands

import (
	"embed"
	"fmt"

	"github.com/kubecc-io/kubecc/pkg/templates"
	"github.com/spf13/cobra"
)

var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Generate Kubernetes configs",
}

var spec struct {
	Name       string
	Namespace  string
	Image      string
	AgentImage string
	CpuLimit   string
	MemLimit   string
	CpuRequest string
	MemRequest string
}

//go:embed objects
var embedFS embed.FS

var buildclusterCmd = &cobra.Command{
	Use:   "buildcluster",
	Short: "Generate a new buildcluster",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := templates.Load(embedFS, "objects/buildcluster.yaml", templates.LoadSpec{
			Spec: spec,
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(string(data))
	},
}

func init() {
	buildclusterCmd.Flags().StringVar(&spec.Image,
		"image", "kubecc/kubecc:latest", "Kubecc image")
	buildclusterCmd.Flags().StringVar(&spec.AgentImage,
		"agent-environment", "kubecc/environment:latest", "Agent environment image")
	buildclusterCmd.Flags().StringVar(&spec.Name,
		"name", "", "Buildcluster name")
	buildclusterCmd.Flags().StringVar(&spec.Namespace,
		"namespace", "kubecc", "Buildcluster namespace")
	buildclusterCmd.Flags().StringVar(&spec.CpuLimit, "cpu-limit", "", "Agent CPU limit")
	buildclusterCmd.Flags().StringVar(&spec.MemLimit, "mem-limit", "8Gi", "Agent Memory limit")
	buildclusterCmd.Flags().StringVar(&spec.CpuRequest, "cpu-request", "4", "Agent CPU request")
	buildclusterCmd.Flags().StringVar(&spec.MemRequest, "mem-request", "", "Agent Memory request")
	buildclusterCmd.MarkFlagRequired("name")

	CreateCmd.AddCommand(buildclusterCmd)
}
