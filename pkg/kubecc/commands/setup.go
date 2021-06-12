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
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/kubecc-io/kubecc/internal/zapkc"
	"github.com/kubecc-io/kubecc/pkg/config"
	. "github.com/kubecc-io/kubecc/pkg/kubecc/internal"
	"github.com/kubecc-io/kubecc/pkg/servers"
	"github.com/kubecc-io/kubecc/pkg/types"
	"github.com/mitchellh/go-homedir"
	"github.com/riywo/loginshell"
	"github.com/snapcore/snapd/systemd"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	home "k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

/*
Setup sequence:
- Check the location of the kubecc binary, ensuring it is in the PATH
- Check if the consumerd service is running (system or user)
	- If not, prompt the user to install consumerd
- Check if a config file is available
	- If not, create one
- Wait for consumerd to start and check its toolchains
- Create symlinks to the toolchains in ~/.kubecc/bin
- Create ~/.kubecc/env and prompt the user to append it to their shell rc
*/

/*
Teardown sequence:
- Check if the consumerd service is running
	- If so, delete it
- Delete ~/.kubecc
*/

var (
	serviceTmpl = `[Unit]
Description=Kubecc Consumerd Service
After=network.target

[Install]
WantedBy=multi-user.target

[Service]
Type=simple
StandardOutput=journal
ExecStart=%s consumerd
Restart=on-failure
`

	envTmpl = `# This file is auto-generated, do not edit!
export KUBECC_HOME="$HOME/.kubecc"
export KUBECC_BINARY="%s"
%s
`

	defaultConsumerdPort  = "10991"
	systemServiceFilepath = "/etc/systemd/system/consumerd.service"
	configFilenames       = []string{
		"config.yaml",
		"config.yml",
		"config.json",
	}
	consumerdUnit  = "consumerd"
	systemdTimeout = 10 * time.Second
)

func systemdClients() (system, user systemd.Systemd) {
	return systemd.New(systemd.SystemMode, nil), systemd.New(systemd.UserMode, nil)
}

func inPath() (string, bool) {
	path, err := exec.LookPath("kubecc")
	return path, err == nil
}

func installBinary() (string, error) {
	printStatus("Checking if kubecc is in your PATH... ")
	if path, ok := inPath(); ok {
		printYes()
		return path, nil
	}
	fmt.Println(zapkc.Red.Add("no"))
	var pathName string
	for {
		defaultPath := "~/.local/bin"
		if sudo {
			defaultPath = "/usr/local/bin"
		}
		err := survey.AskOne(&survey.Select{
			Message: "Choose an install location for the kubecc binary",
			Options: []string{"~/.local/bin", "~/bin", "/usr/local/bin", "(other)"},
			Default: defaultPath,
		}, &pathName)
		if err != nil {
			return "", err
		}
		if pathName == "(other)" {
			err := survey.AskOne(&survey.Input{
				Message: "Enter an install path",
				Default: "",
			}, &pathName)
			if err != nil {
				return "", err
			}
		}
		pathName, err = homedir.Expand(pathName)
		if err != nil {
			return "", err
		}
		if f, err := os.Stat(pathName); os.IsNotExist(err) {
			err = os.MkdirAll(pathName, 0o775)
			if err != nil {
				printErr(fmt.Sprintf("Could not create necessary directories: %s", err.Error()))
				continue
			}
		} else if !f.IsDir() {
			printErr("The specified path already exists and is not a directory")
			continue
		}
		break
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	self, err := os.Open(executable)
	if err != nil {
		return "", err
	}
	defer self.Close()
	dest, err := os.Create(filepath.Join(pathName, filepath.Base(executable)))
	if err != nil {
		return "", err
	}
	defer dest.Close()
	err = dest.Chmod(0o0775)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(dest, self)
	if err != nil {
		return "", err
	}
	fmt.Printf(zapkc.Green.Add("Installed kubecc to %s\n"), dest.Name())
	return dest.Name(), nil
}

func configExists(option string) bool {
	switch option {
	case "user":
		for _, name := range configFilenames {
			if _, err := os.Stat(path.Join(path.Join(home.HomeDir(), ".kubecc"), name)); err == nil {
				return true
			}
		}
	case "system":
		for _, name := range configFilenames {
			if _, err := os.Stat(path.Join("/etc/kubecc", name)); err == nil {
				return true
			}
		}
	}
	return false
}

func validateAddress(address interface{}) error {
	host, port, err := net.SplitHostPort(fmt.Sprint(address))
	if err != nil {
		return err
	}
	if len(host) == 0 {
		return fmt.Errorf("host must not be empty")
	}
	if len(port) == 0 {
		return fmt.Errorf("port must not be empty")
	}
	_, err = strconv.ParseUint(port, 10, 16)
	return err
}

func installConfig(option string) error {
	questions := []*survey.Question{
		{
			Name: "listenPort",
			Prompt: &survey.Input{
				Message: "Enter a local port for the consumerd to listen on",
				Default: defaultConsumerdPort,
			},
			Validate: func(ans interface{}) error {
				_, err := strconv.ParseUint(fmt.Sprint(ans), 10, 16)
				return err
			},
		},
		{
			Name: "schedulerAddress",
			Prompt: &survey.Input{
				Message: "Enter the scheduler's address (ip:port)",
				Help:    "The address should be formatted as an IP or hostname, followed by a colon, followed by a port number.",
			},
			Validate: validateAddress,
		},
		{
			Name: "monitorAddress",
			Prompt: &survey.Input{
				Message: "Enter the monitor's address (ip:port)",
				Help:    "The address should be formatted as an IP or hostname, followed by a colon, followed by a port number.",
			},
			Validate: validateAddress,
		},
		{
			Name: "tlsEnabled",
			Prompt: &survey.Confirm{
				Message: "Connect to the monitor and scheduler using TLS?",
				Default: true,
			},
		},
	}

	answers := struct {
		LocalPort        string `survey:"listenPort"`
		SchedulerAddress string `survey:"schedulerAddress"`
		MonitorAddress   string `survey:"monitorAddress"`
		TLSEnabled       bool   `survey:"tlsEnabled"`
	}{}
	err := survey.Ask(questions, &answers)
	if err != nil {
		return err
	}

	conf := &config.KubeccSpec{
		Global: config.GlobalSpec{
			LogLevel: "info",
		},
		Consumer: config.ConsumerSpec{
			ConsumerdAddress: "127.0.0.1:" + answers.LocalPort,
		},
		Consumerd: config.ConsumerdSpec{
			ListenAddress:    "127.0.0.1:" + answers.LocalPort,
			SchedulerAddress: answers.SchedulerAddress,
			MonitorAddress:   answers.MonitorAddress,
			DisableTLS:       !answers.TLSEnabled,
			UsageLimits: &config.UsageLimitsSpec{
				ConcurrentProcessLimit: -1,
			},
		},
		Kcctl: config.KcctlSpec{
			GlobalSpec: config.GlobalSpec{
				LogLevel: "warn",
			},
			MonitorAddress:   answers.MonitorAddress,
			SchedulerAddress: answers.SchedulerAddress,
			DisableTLS:       !answers.TLSEnabled,
		},
	}

	printStatus("Writing config file... ")
	data, err := yaml.Marshal(conf)
	if err != nil {
		printFailed()
		return err
	}

	switch option {
	case "user":
		localKubeccPath := path.Join(home.HomeDir(), ".kubecc")
		err := os.MkdirAll(localKubeccPath, 0o755)
		if err != nil {
			printFailed()
			return err
		}
		err = os.WriteFile(filepath.Join(localKubeccPath, "config.yaml"), data, 0o644)
		if err != nil {
			printFailed()
			return err
		}

	case "system":
		err := os.MkdirAll("/etc/kubecc", 0o755)
		if err != nil {
			printFailed()
			return err
		}
		err = os.WriteFile("/etc/kubecc/config.yaml", data, 0o644)
		if err != nil {
			printFailed()
			return err
		}
	}

	printDone()
	return nil
}

func checkConfig(option string) error {
	printStatus("Checking if a configuration file is available... ")
	if configExists(option) {
		fmt.Printf(zapkc.Green.Add("yes (%s)\n"), option)
		return nil
	}
	printNo()
	return installConfig(option)
}

func catUnit(option string) (string, error) {
	cmd := exec.Command("/usr/bin/systemctl", "--"+option, "cat", "consumerd.service")
	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	output := strings.Split(stdout.String(), "\n")
	if len(output) <= 2 {
		return "", fmt.Errorf("unexpected output from %q", cmd.String())
	}
	// remove the comment at the beginning of the output
	outputStr := strings.Join(output[1:], "\n")
	return strings.TrimSpace(outputStr), nil
}

func writeUserService(serviceContents string) error {
	printStatus("Installing user service... ")
	systemdUser, err := homedir.Expand("~/.config/systemd/user")
	if err != nil {
		printFailed()
		return err
	}
	err = os.MkdirAll(systemdUser, 0o755)
	if err != nil {
		printFailed()
		return err
	}
	err = os.WriteFile(filepath.Join(systemdUser, "consumerd.service"),
		[]byte(serviceContents), 0o644)
	if err != nil {
		printFailed()
		return err
	}
	printDone()
	return nil
}

func writeSystemService(serviceContents string) error {
	printStatus("Installing system service... ")
	err := os.WriteFile(systemServiceFilepath, []byte(serviceContents), 0o644)
	if err != nil {
		printFailed()
		return err
	}
	printDone()
	return nil
}

func updateService(option, newContents string) error {
	system, user := systemdClients()

	var selected systemd.Systemd
	switch option {
	case "system":
		err := writeSystemService(newContents)
		if err != nil {
			return err
		}
		selected = system
	case "user":
		err := writeUserService(newContents)
		if err != nil {
			return err
		}
		selected = user
	}

	if err := selected.DaemonReload(); err != nil {
		return err
	}
	if err := selected.Restart(consumerdUnit, systemdTimeout); err != nil {
		return err
	}
	return nil
}

func installConsumerd(binaryPath string) (systemd.Systemd, string, error) {
	system := systemd.New(systemd.SystemMode, nil)
	user := systemd.New(systemd.UserMode, nil)

	printStatus("Checking if the consumerd service is running... ")
	serviceContents := strings.TrimSpace(fmt.Sprintf(serviceTmpl, binaryPath))

	active, err := system.IsActive(consumerdUnit)
	if err != nil {
		printFailed()
		return nil, "", err
	}

	if active {
		fmt.Println(zapkc.Green.Add("yes (system)"))
		// check if the file contents need updating
		if existing, err := catUnit("system"); err == nil && existing != serviceContents {
			if err := updateService("system", serviceContents); err != nil {
				return nil, "", err
			}
		}
		return system, "system", nil
	}

	option := "system"
	if os.Getuid() != 0 {
		option = "user"
		active, err = user.IsActive(consumerdUnit)
		if err != nil {
			printFailed()
			return nil, "", err
		}
		if active {
			fmt.Println(zapkc.Green.Add("yes (user)"))
			// check if the file contents need updating
			if existing, err := catUnit("user"); err == nil && existing != serviceContents {
				if err := updateService("user", serviceContents); err != nil {
					return nil, "", err
				}
			}
			return user, "user", nil
		}
	}
	fmt.Println(zapkc.Red.Add("no"))

	var selected systemd.Systemd
	switch option {
	case "user":
		if err := writeUserService(serviceContents); err != nil {
			return nil, "", err
		}
		selected = user
	case "system":
		if err := writeSystemService(serviceContents); err != nil {
			return nil, "", err
		}
		selected = system
	}
	return selected, option, nil
}

func startConsumerd(sd systemd.Systemd) error {
	if err := sd.DaemonReload(); err != nil {
		return err
	}

	printStatus("Starting service... ")
	if err := sd.Start(consumerdUnit); err != nil {
		printFailed()
		return err
	}
	if err := sd.Enable(consumerdUnit); err != nil {
		printFailed()
		return err
	}
	printDone()
	return nil
}

func connectToConsumerd() (*grpc.ClientConn, error) {
	printStatus("Connecting to consumerd... ")
	conf := config.ConfigMapProvider.Load()
	ctx, cancel := context.WithCancel(CLIContext)
	defer cancel()
	cc, err := servers.Dial(ctx, conf.Consumer.ConsumerdAddress,
		servers.WithDialOpts(grpc.WithBlock()))
	if err != nil {
		printFailed()
		return nil, err
	}
	printDone()
	return cc, nil
}

func setupToolchains(binaryPath string, cc *grpc.ClientConn) error {
	printStatus("Configuring toolchains... ")
	binDir, err := homedir.Expand("~/.kubecc/bin")
	if err != nil {
		printFailed()
		return err
	}
	if _, err := os.Stat(binDir); err == nil {
		err := os.RemoveAll(binDir)
		if err != nil {
			return err
		}
	}
	err = os.MkdirAll(binDir, 0o775)
	if err != nil {
		printFailed()
		return err
	}

	client := types.NewConsumerdClient(cc)
	ctx, cancel := context.WithTimeout(CLIContext, 10*time.Second)
	defer cancel()
	tcs, err := client.GetToolchains(
		ctx, &types.Empty{}, grpc.WaitForReady(true))
	if err != nil {
		printFailed()
		return err
	}
	for i := 0; i < 10; i++ {
		items := tcs.GetItems()
		if len(items) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}
		for _, tc := range items {
			if tc.Kind == types.Sleep {
				// Prevent creating a symlink named 'kubecc'
				continue
			}

			for _, name := range tc.CommonNames() {
				link := filepath.Join(binDir, name)
				// Remove old link if it exists
				if _, err := os.Stat(link); err == nil {
					err := os.Remove(link)
					if err != nil {
						printFailed()
						return err
					}
				}
				// Create new link
				err := os.Symlink(binaryPath, link)
				if err != nil {
					printFailed()
					return err
				}
			}
		}
		break
	}

	printDone()
	return nil
}

func makeAliases(binPath string) ([]string, error) {
	binDir, err := homedir.Expand("~/.kubecc/bin")
	if err != nil {
		return []string{}, err
	}
	aliases := []string{
		fmt.Sprintf(`alias make='PATH="%s${PATH:+:${PATH}}" make'`, binDir),
	}

	entries, err := os.ReadDir(binDir)
	if err != nil {
		return []string{}, err
	}

	for _, f := range entries {
		if f.IsDir() {
			continue
		}

		aliases = append(aliases,
			fmt.Sprintf(`alias %s='PATH="%s${PATH:+:${PATH}}" %s'`,
				f.Name(),
				binDir,
				f.Name(),
			),
		)
	}

	return aliases, nil
}

func doAppend(rc string) error {
	printStatus(fmt.Sprintf("Appending to %s... ", rc))
	expanded, err := homedir.Expand(rc)
	if err != nil {
		printFailed()
		return err
	}
	f, err := os.OpenFile(expanded, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		printFailed()
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte{'\n'})
	if err != nil {
		printFailed()
		return err
	}
	_, err = f.WriteString(`source "$HOME/.kubecc/env"`)
	if err != nil {
		printFailed()
		return err
	}
	_, err = f.Write([]byte{'\n'})
	if err != nil {
		printFailed()
		return err
	}
	printDone()
	return nil
}

func appendToShellRC() error {
	shell, err := loginshell.Shell()
	if err != nil {
		return err
	}
	switch filepath.Base(shell) {
	case "bash":
		return doAppend("~/.bashrc")
	case "zsh":
		return doAppend("~/.zshrc")
	case "ash":
		return doAppend("~/.ashrc")
	default:
		return fmt.Errorf("Sorry, your shell is not supported.")
	}
}

func setupEnv(binPath string) error {
	printStatus("Writing environment file... ")
	envPath, err := homedir.Expand("~/.kubecc/env")
	if err != nil {
		printFailed()
		return err
	}
	aliases, err := makeAliases(binPath)
	if err != nil {
		printFailed()
		return err
	}
	contents := fmt.Sprintf(envTmpl, binPath, strings.Join(aliases, "\n"))
	if err := os.WriteFile(envPath, []byte(contents), 0644); err != nil {
		printFailed()
		return err
	}
	printDone()

	for {
		printStatus("Checking if environment file is being sourced... ")
		shell, err := loginshell.Shell()
		if err != nil {
			printFailed()
			return err
		}
		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)
		cmd := exec.Command(shell, "-i", "-c", `echo $KUBECC_BINARY`)
		cmd.Env = []string{}
		cmd.Stdout = outBuf
		cmd.Stderr = errBuf
		if err := cmd.Run(); err != nil {
			printFailed()
			return fmt.Errorf("%w: %s", err, errBuf.String())
		}
		if strings.TrimSpace(outBuf.String()) != binPath {
			printNo()
			var response string
			err := survey.AskOne(&survey.Select{
				Message: fmt.Sprintf("Please add '%s' to your shell's RC file, then select Retry to check again", `source "$HOME/.kubecc/env"`),
				Options: []string{"Retry", "Do this for me", "Skip this step"},
				Default: "Retry",
			}, &response)
			if err != nil {
				return err
			}
			if response == "Skip this step" {
				break
			} else if response == "Do this for me" {
				var confirm bool
				err := survey.AskOne(&survey.Confirm{
					Message: "This will attempt to append the necessary line to your shell RC file. Continue?",
					Default: false,
				}, &confirm)
				if err != nil {
					return err
				}
				if confirm {
					if err := appendToShellRC(); err != nil {
						printErr(err.Error())
					}
				}
			}
		} else {
			printYes()
			break
		}
	}
	return nil
}

var SetupCmd = &cobra.Command{
	Use:     "setup",
	Short:   "Set up and configure Kubecc on your machine",
	PreRun:  sudoPreRun,
	PostRun: sudoPostRun,
	Run: func(cmd *cobra.Command, args []string) {
		binPath, err := installBinary()
		if err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
		sd, option, err := installConsumerd(binPath)
		if err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
		if err := checkConfig(option); err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
		InitCLIQuiet(cmd, args)
		if active, err := sd.IsActive(consumerdUnit); err == nil && !active {
			if err := startConsumerd(sd); err != nil {
				printErr(err.Error())
				os.Exit(1)
			}
		}

		if sudo {
			return
		}
		cc, err := connectToConsumerd()
		if err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
		if err := setupToolchains(binPath, cc); err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
		if err := setupEnv(binPath); err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
	},
}
