package util

import (
	"io/fs"
	"os"
)

type ReadDirStatFS interface {
	fs.StatFS
	fs.ReadDirFS
}

// OSFS represents the host operating system's FS
var OSFS = osfs{}

type osfs struct {
	ReadDirStatFS
}

func (osfs) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
func (osfs) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}
