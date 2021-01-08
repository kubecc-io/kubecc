package agent

import (
	"os"
	"path"

	"github.com/cobalt77/kube-cc/agent/cmd"
)

func main() {
	InitConfig()
	switch path.Base(os.Args[0]) {
	case "kdc-agent":
		cmd.Execute()
	default:
		// Assume we are symlinked to a compiler
		StartAgentOrDispatch()
	}
}
