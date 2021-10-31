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
	"os/user"
	"strconv"
	"syscall"

	"github.com/kubecc-io/kubecc/internal/zapkc"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

func init() {
	homedir.DisableCache = true
}

func printFailed() {
	fmt.Println(zapkc.Red.Add("failed"))
}

func printDone() {
	fmt.Println(zapkc.Green.Add("done"))
}

func printNo() {
	fmt.Println(zapkc.Red.Add("no"))
}

func printYes() {
	fmt.Println(zapkc.Green.Add("yes"))
}

func printStatus(msg string) {
	if sudo {
		fmt.Print(zapkc.Yellow.Add("❯ ") + zapkc.White.Add(msg))
	} else {
		fmt.Print(zapkc.Blue.Add("❯ ") + zapkc.White.Add(msg))
	}
}

func printErr(msg string) {
	fmt.Fprintln(os.Stderr, zapkc.Red.Add(msg))
}

func sudoUidGid() (uid int, gid int, runningInSudo bool) {
	if uidStr, ok := os.LookupEnv("SUDO_UID"); ok {
		if id, err := strconv.Atoi(uidStr); err != nil {
			return
		} else {
			uid = id
		}
	} else {
		return
	}
	if gidStr, ok := os.LookupEnv("SUDO_GID"); ok {
		if id, err := strconv.Atoi(gidStr); err != nil {
			return
		} else {
			gid = id
		}
	} else {
		return
	}
	runningInSudo = true
	return
}

var uid, gid int
var sudo bool

func sudoPreRun(cmd *cobra.Command, args []string) {
	uid, gid, sudo = sudoUidGid()
	if sudo {
		fmt.Println(zapkc.Yellow.Add(
			"You are running in sudo. This setup will run as root, then will run again as your real user."))
	}
}

func sudoPostRun(cmd *cobra.Command, args []string) {
	if !sudo {
		return
	}
	sudo = false

	fmt.Println(zapkc.Yellow.Add("De-escalating permissions"))
	if err := syscall.Setregid(gid, gid); err != nil {
		panic(err)
	}
	if err := syscall.Setreuid(uid, uid); err != nil {
		panic(err)
	}
	u, err := user.LookupId(fmt.Sprint(uid))
	if err != nil {
		panic(err)
	}

	os.Setenv("HOME", u.HomeDir)
	os.Setenv("UID", u.Uid)
	os.Setenv("GID", u.Gid)
	os.Setenv("USER", u.Username)
	os.Unsetenv("SUDO_UID")
	os.Unsetenv("SUDO_GID")
	os.Unsetenv("SUDO_USER")
	os.Unsetenv("SUDO_COMMAND")
	fmt.Printf(zapkc.Blue.Add("Switched to user %s\n"), u.Username)
	cmd.Run(cmd, args)
}
