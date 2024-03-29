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
	"crypto/md5"
	"errors"
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

	envEnabled = `# Edit this file using 'kubecc enable' and 'kubecc disable'
unset KUBECC_ENABLED
source %s/.env
kubecc_enable
`

	envDisabled = `# Edit this file using 'kubecc enable' and 'kubecc disable'
unset KUBECC_ENABLED
source %s/.env
kubecc_disable
`

	dotEnvTmpl = `# This file is auto-generated, do not edit!
export KUBECC_HOME="{home}"
export KUBECC_BINARY="{bin}"

kubecc_enable() {
  if [ "$KUBECC_ENABLED" = "1" ]; then
    return
  fi
{aliases}
  eval ${KUBECC_BINARY} enable > /dev/null
  export KUBECC_ENABLED="1"
}

kubecc_disable() {
  if [ "$KUBECC_ENABLED" = "0" ]; then
    return
  fi
{unaliases}
  eval ${KUBECC_BINARY} disable > /dev/null
  export KUBECC_ENABLED="0"
}
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

	nonInteractiveConfig = struct {
		Enabled             bool
		SchedulerAddress    string
		MonitorAddress      string
		InstallDir          string
		ConsumerdListenPort string
		TLSEnabled          bool
		ShellRCConfig       bool
	}{
		Enabled:             false,
		TLSEnabled:          false,
		ShellRCConfig:       true,
		ConsumerdListenPort: defaultConsumerdPort,
	}
)

func systemdClients() (system, user systemd.Systemd) {
	return systemd.New(systemd.SystemMode, nil), systemd.New(systemd.UserMode, nil)
}

func inPath() (string, bool) {
	path, err := exec.LookPath("kubecc")
	return path, err == nil
}

func md5sum(path string) ([]byte, error) {
	hash := md5.New()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

func copySelfTo(destFolder string) (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	self, err := os.Open(executable)
	if err != nil {
		return "", err
	}
	defer self.Close()
	// If the file already exists, remove it first. It may be in use by the
	// consumerd service, which will prevent us from truncating it in-place.
	// However, removing the file first will keep it around in memory for the
	// consumerd service to use until it is restarted.
	destFilePath := filepath.Join(destFolder, filepath.Base(executable))
	if _, err := os.Stat(destFilePath); err == nil {
		if err := os.Remove(destFilePath); err != nil {
			return "", err
		}
	}
	dest, err := os.OpenFile(destFilePath, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0o755)
	if err != nil {
		return "", err
	}
	defer dest.Close()
	_, err = io.Copy(dest, self)
	if err != nil {
		return "", err
	}
	return dest.Name(), nil
}

func binaryNeedsUpdate(existing string) (bool, error) {
	self, err := os.Executable()
	if err != nil {
		return false, err
	}
	selfMd5, err := md5sum(self)
	if err != nil {
		return false, err
	}
	pathMd5, err := md5sum(existing)
	if err != nil {
		return false, err
	}
	return !bytes.Equal(selfMd5, pathMd5), nil
}

type installResult int

const (
	installResultInstalled installResult = iota
	installResultUpdated
	installResultUpToDate
	installResultFailed
)

func installOrUpdateBinary() (string, installResult, error) {
	printStatus("Checking if kubecc is in your PATH... ")
	if path, ok := inPath(); ok {
		printYes()
		printStatus("Checking if the existing binary needs to be updated... ")
		needsUpdate, err := binaryNeedsUpdate(path)
		if err != nil {
			printFailed()
			return "", installResultFailed, err
		}
		if needsUpdate {
			printYes()
			printStatus("Updating the existing binary... ")
			if _, err := copySelfTo(filepath.Dir(path)); err != nil {
				printFailed()
				return "", installResultFailed, err
			}
			printDone()
			return path, installResultUpdated, nil
		}
		printNo()
		return path, installResultUpToDate, nil
	}
	printNo()
	var dirName string
	for {
		defaultPath := "~/.local/bin"
		if sudo {
			defaultPath = "/usr/local/bin"
		}
		if nonInteractiveConfig.Enabled {
			if nonInteractiveConfig.InstallDir != "" {
				dirName = nonInteractiveConfig.InstallDir
			} else {
				dirName = defaultPath
			}
		} else {
			err := survey.AskOne(&survey.Select{
				Message: "Choose an install location for the kubecc binary",
				Options: []string{"~/.local/bin", "~/bin", "/usr/local/bin", "(other)"},
				Default: defaultPath,
			}, &dirName)
			if err != nil {
				return "", installResultFailed, err
			}
			if dirName == "(other)" {
				err := survey.AskOne(&survey.Input{
					Message: "Enter an install path",
					Default: "",
				}, &dirName)
				if err != nil {
					return "", installResultFailed, err
				}
			}
		}
		dirName, err := homedir.Expand(dirName)
		if err != nil {
			return "", installResultFailed, err
		}
		if f, err := os.Stat(dirName); os.IsNotExist(err) {
			err = os.MkdirAll(dirName, 0o775)
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
	dest, err := copySelfTo(dirName)
	if err != nil {
		printErr(err.Error())
		return "", installResultFailed, err
	}
	fmt.Printf(zapkc.Green.Add("Installed kubecc to %s\n"), dest)
	return dest, installResultInstalled, nil
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
	if nonInteractiveConfig.Enabled {
		answers.LocalPort = nonInteractiveConfig.ConsumerdListenPort
		answers.SchedulerAddress = nonInteractiveConfig.SchedulerAddress
		answers.MonitorAddress = nonInteractiveConfig.MonitorAddress
		answers.TLSEnabled = nonInteractiveConfig.TLSEnabled
	} else {
		err := survey.Ask(questions, &answers)
		if err != nil {
			return err
		}
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

func restartConsumerd(sd systemd.Systemd) error {
	if err := sd.DaemonReload(); err != nil {
		return err
	}

	printStatus("Restarting service... ")
	if err := sd.Restart(consumerdUnit, systemdTimeout); err != nil {
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

var ErrShellUnsupported = errors.New("Sorry, your shell is not supported yet.")

func rcPath(shell string) (string, error) {
	switch filepath.Base(shell) {
	case "bash":
		return "~/.bashrc", nil
	case "zsh":
		return "~/.zshrc", nil
	case "ash":
		return "~/.ashrc", nil
	}
	return "", ErrShellUnsupported
}

func appendToShellRC() error {
	shell, err := loginshell.Shell()
	if err != nil {
		return err
	}
	rc, err := rcPath(shell)
	if err != nil {
		return err
	}
	return doAppend(rc)
}

func indent(str string) string {
	return "  " + strings.ReplaceAll(strings.TrimSpace(str), "\n", "\n  ")
}

func setupEnv(binPath string) error {
	printStatus("Writing environment files... ")
	kubeccHome, err := homedir.Expand("~/.kubecc")
	if err != nil {
		printFailed()
		return err
	}
	envPath, err := homedir.Expand("~/.kubecc/env")
	if err != nil {
		printFailed()
		return err
	}
	dotEnvPath, err := homedir.Expand("~/.kubecc/.env")
	if err != nil {
		printFailed()
		return err
	}
	aliases, err := makeAliases(binPath)
	if err != nil {
		printFailed()
		return err
	}
	unaliases := []string{}
	for _, alias := range aliases {
		unaliases = append(unaliases,
			strings.Split(strings.Replace(alias, "alias", "unalias", 1), "=")[0],
		)
	}
	contents := strings.NewReplacer(
		"{bin}", binPath,
		"{aliases}", indent(strings.Join(aliases, "\n")),
		"{unaliases}", indent(strings.Join(unaliases, "\n")),
		"{home}", kubeccHome,
	)
	if err := os.WriteFile(dotEnvPath, []byte(contents.Replace(dotEnvTmpl)), 0644); err != nil {
		printFailed()
		return err
	}
	if err := os.WriteFile(envPath, []byte(fmt.Sprintf(envEnabled, kubeccHome)), 0644); err != nil {
		printFailed()
		return err
	}
	printDone()

	for {
		printStatus("Checking if environment files are being sourced... ")
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
			if nonInteractiveConfig.Enabled {
				if nonInteractiveConfig.ShellRCConfig {
					response = "Do this for me"
				} else {
					response = "Skip this step"
				}
			} else {
				err := survey.AskOne(&survey.Select{
					Message: fmt.Sprintf("Please add '%s' to your shell's RC file, then select Retry to check again", `source "$HOME/.kubecc/env"`),
					Options: []string{"Retry", "Do this for me", "Skip this step"},
					Default: "Retry",
				}, &response)
				if err != nil {
					return err
				}
			}
			if response == "Skip this step" {
				break
			} else if response == "Do this for me" {
				var confirm bool
				if nonInteractiveConfig.Enabled {
					confirm = true
				} else {
					err := survey.AskOne(&survey.Confirm{
						Message: "This will attempt to append the necessary line to your shell RC file. Continue?",
						Default: false,
					}, &confirm)
					if err != nil {
						return err
					}
				}
				if confirm {
					if err := appendToShellRC(); err != nil {
						printErr(err.Error())
					}
					break
				}
			}
		} else {
			printYes()
			break
		}
	}
	return nil
}

func checkNonInteractiveConfig() {
	if sch, ok := os.LookupEnv("KUBECC_SETUP_SCHEDULER_ADDRESS"); ok {
		if mon, ok := os.LookupEnv("KUBECC_SETUP_MONITOR_ADDRESS"); ok {
			nonInteractiveConfig.Enabled = true
			nonInteractiveConfig.SchedulerAddress = sch
			nonInteractiveConfig.MonitorAddress = mon

			if dir, ok := os.LookupEnv("KUBECC_SETUP_INSTALL_DIR"); ok {
				nonInteractiveConfig.InstallDir = dir
			}
			if port, ok := os.LookupEnv("KUBECC_SETUP_CONSUMERD_LISTEN_PORT"); ok {
				nonInteractiveConfig.ConsumerdListenPort = port
			}
			if tls, ok := os.LookupEnv("KUBECC_SETUP_TLS_ENABLED"); ok {
				if fmt.Sprint(tls) == "true" {
					nonInteractiveConfig.TLSEnabled = true
				}
			}
			if rc, ok := os.LookupEnv("KUBECC_SETUP_SHELL_RC_CONFIG"); ok {
				if fmt.Sprint(rc) == "false" {
					nonInteractiveConfig.ShellRCConfig = false
				}
			}
		}
	}
}

var SetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up and configure Kubecc on your machine",
	Long: `The setup command will configure the client-side components needed to connect
to and use a kubecc cluster. This includes the consumerd service, config files,
and toolchain discovery. 

By default, the setup command will run interactively. If you need to run setup
non-interactively (i.e. through a script), you can set the following environment
variables which will skip the prompts:

Required:
KUBECC_SETUP_SCHEDULER_ADDRESS       [ip/fqdn:port to connect to the scheduler]
KUBECC_SETUP_MONITOR_ADDRESS 			   [ip/fqdn:port to connect to the monitor]

Optional:
KUBECC_SETUP_INSTALL_DIR             [default: ~/.local/bin or /usr/local/bin]
KUBECC_SETUP_CONSUMERD_LISTEN_PORT   [default: 10991]
KUBECC_SETUP_TLS_ENABLED 					   [true/false, default: false]
KUBECC_SETUP_SHELL_RC_CONFIG         [true/false, default: true]

If both of the required environment variables are set, the setup command will
run non-interactively. The optional environment variables can be set to
override the default values.
`,
	PreRun:  sudoPreRun,
	PostRun: sudoPostRun,
	Run: func(cmd *cobra.Command, args []string) {
		checkNonInteractiveConfig()
		binPath, result, err := installOrUpdateBinary()
		if err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
		sd, option, err := installConsumerd(binPath)
		if err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
		if result == installResultUpdated {
			if err := restartConsumerd(sd); err != nil {
				printErr(err.Error())
				os.Exit(1)
			}
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

func writeEnvFileOrDie(contents string) {
	if value, ok := os.LookupEnv("KUBECC_HOME"); !ok {
		printErr("Kubecc is not configured. Try running 'kubecc setup' first.")
		os.Exit(1)
	} else {
		if err := os.WriteFile(path.Join(value, "env"), []byte(contents), 0644); err != nil {
			printErr(err.Error())
			os.Exit(1)
		}
	}
}

var EnableCmd = &cobra.Command{
	Use:    "enable",
	Short:  "Re-enable kubecc in your local environment",
	PreRun: InitCLI,
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeccHome, err := homedir.Expand("~/.kubecc")
		if err != nil {
			return err
		}
		writeEnvFileOrDie(fmt.Sprintf(envEnabled, kubeccHome))
		CLILog.Info(zapkc.Green.Add("Kubecc enabled"))
		return nil
	},
}

var DisableCmd = &cobra.Command{
	Use:    "disable",
	Short:  "Temporarily disable kubecc in your local environment",
	PreRun: InitCLI,
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeccHome, err := homedir.Expand("~/.kubecc")
		if err != nil {
			return err
		}
		writeEnvFileOrDie(fmt.Sprintf(envDisabled, kubeccHome))
		CLILog.Info(zapkc.Yellow.Add("Kubecc disabled"))
		return nil
	},
}
