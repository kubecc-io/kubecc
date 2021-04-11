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
	"fmt"

	"github.com/kubecc-io/kubecc/staging"
	"github.com/spf13/cobra"
)

var InstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Output the Kubernetes configuration for Kubecc",
	Example: `kubecc install | kubectl apply -f -
kubecc install | kubectl delete -f -`,
	SuggestFor: []string{"uninstall"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(staging.StagingAutogenYaml)
	},
}
