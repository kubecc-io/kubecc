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
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/kubecc-io/kubecc/internal/zapkc"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

func removeSystemConsumerd() {
	if active, err := unitIsActive(true, "consumerd"); err == nil && active {
		if err := stopService("system"); err != nil {
			printErr(err.Error())
		}
	}
	if _, err := os.Stat(systemServiceFilepath); err == nil {
		printStatus("Removing service (system)... ")
		if err := os.Remove(systemServiceFilepath); err != nil {
			printFailed()
			printErr(err.Error())
			return
		}
		printDone()
	}
	if err := daemonReload("system"); err != nil {
		printErr(err.Error())
	}
}

func removeUserConsumerd() {
	if active, err := unitIsActive(false, "consumerd"); err == nil && active {
		if err := stopService("user"); err != nil {
			printErr(err.Error())
		}
	}
	path, err := homedir.Expand("~/.config/systemd/user/consumerd.service")
	if err == nil {
		if _, err := os.Stat(path); err == nil {
			printStatus("Removing service (user)... ")
			if err := os.Remove(path); err != nil {
				printFailed()
				printErr(err.Error())
				return
			}
			printDone()
		}
	}
	if err := daemonReload("user"); err != nil {
		printErr(err.Error())
	}
}

func deleteConfig() {
	path, err := homedir.Expand("~/.kubecc")
	if err != nil {
		printErr(err.Error())
		return
	}

	printStatus("Deleting local configuration files... ")
	for _, f := range configFilenames {
		configPath := filepath.Join(path, f)
		if _, err := os.Stat(configPath); err == nil {
			if err := os.Remove(configPath); err != nil {
				printFailed()
				printErr(err.Error())
				return
			}
		}
	}
	printDone()
}

func deleteSystemConfig() {
	printStatus("Deleting system configuration files... ")
	for _, f := range configFilenames {
		configPath := filepath.Join("/etc/kubecc", f)
		if _, err := os.Stat(configPath); err == nil {
			if err := os.Remove(configPath); err != nil {
				printFailed()
				printErr(err.Error())
				return
			}
		}
	}
	printDone()
}

func deleteEnv() {
	path, err := homedir.Expand("~/.kubecc")
	if err != nil {
		printErr(err.Error())
		return
	}
	printStatus("Deleting local environment files... ")
	envFilePath := filepath.Join(path, "env")
	binPath := filepath.Join(path, "bin")
	if _, err := os.Stat(envFilePath); err == nil {
		if err := os.Remove(envFilePath); err != nil {
			printFailed()
			printErr(err.Error())
			return
		}
	}
	if _, err := os.Stat(binPath); err == nil {
		if err := os.RemoveAll(binPath); err != nil {
			printFailed()
			printErr(err.Error())
			return
		}
	}
	printDone()
}

func deleteBinary() {
	if binFromPath, err := exec.LookPath("kubecc"); err == nil {
		printStatus(fmt.Sprintf("Deleting %s... ", binFromPath))
		if err := os.Remove(binFromPath); err != nil {
			printFailed()
			printErr(err.Error())
		}
		printDone()
	}
}

var TeardownCmd = &cobra.Command{
	Use:     "teardown",
	Short:   "Remove an existing local kubecc installation",
	PreRun:  sudoPreRun,
	PostRun: sudoPostRun,
	Run: func(cmd *cobra.Command, args []string) {
		options := map[string]func(){}
		if active, err := unitIsActive(true, "consumerd"); err == nil && active {
			options["Consumerd service (system)"] = removeSystemConsumerd
		} else if _, err := os.Stat(systemServiceFilepath); err == nil {
			options["Consumerd service (system)"] = removeSystemConsumerd
		}

		if active, err := unitIsActive(false, "consumerd"); err == nil && active {
			options["Consumerd service (user)"] = removeUserConsumerd
		} else {
			path, err := homedir.Expand("~/.config/systemd/user/consumerd.service")
			if err == nil {
				if _, err := os.Stat(path); err == nil {
					options["Consumerd service (user)"] = removeUserConsumerd
				}
			}
		}

		kubeccHome, err := homedir.Expand("~/.kubecc")
		if err == nil {
			for _, f := range configFilenames {
				if _, err := os.Stat(filepath.Join(kubeccHome, f)); err == nil {
					options["Local configuration files"] = deleteConfig
					break
				}
			}
			for _, f := range configFilenames {
				if _, err := os.Stat(filepath.Join("/etc/kubecc", f)); err == nil {
					options["System configuration files"] = deleteSystemConfig
					break
				}
			}

			if _, err := os.Stat(filepath.Join(kubeccHome, "bin")); err == nil {
				options["Local environment files"] = deleteEnv
			} else if _, err := os.Stat(filepath.Join(kubeccHome, "env")); err == nil {
				options["Local environment files"] = deleteEnv
			}
		}

		if binary, err := exec.LookPath("kubecc"); err == nil {
			if _, err := os.Stat(binary); err == nil {
				options["Kubecc binary"] = deleteBinary
			}
		}

		if len(options) == 0 {
			fmt.Println(zapkc.Green.Add("Nothing to do."))
			return
		}

		keys := []string{}
		for k := range options {
			keys = append(keys, k)
		}

		selectedOptions := []string{}
		err = survey.AskOne(&survey.MultiSelect{
			Message: "Select the components of Kubecc you would like to remove",
			Options: keys,
		}, &selectedOptions)
		if err != nil {
			printErr(err.Error())
			return
		}

		for _, option := range selectedOptions {
			options[option]()
		}

		// Delete ~/.kubecc if it is empty
		if entries, err := os.ReadDir(kubeccHome); err == nil {
			if len(entries) == 0 {
				os.Remove(kubeccHome)
			}
		}
	},
}
