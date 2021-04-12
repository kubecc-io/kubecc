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

package util

import (
	"io/fs"
	"os"
)

type ReadDirStatFS interface {
	fs.StatFS
	fs.ReadDirFS
}

// OSFS represents the host operating system's FS.
var OSFS osfs

type osfs struct{}

var _ ReadDirStatFS = osfs{}

func (osfs) Open(name string) (fs.File, error) {
	return os.Open(name)
}
func (osfs) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}
func (osfs) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

var PreferredTempDirectory string

func init() {
	if _, err := os.Stat("/dev/shm"); err == nil {
		PreferredTempDirectory = "/dev/shm"
	}
	PreferredTempDirectory = "/tmp"
}
