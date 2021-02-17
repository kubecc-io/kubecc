package tools

import (
	"io/fs"
	"os"
)

type ReadDirStatFS interface {
	fs.StatFS
	fs.ReadDirFS
}

type OSFS struct {
	ReadDirStatFS
}

func (rfs OSFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
func (rfs OSFS) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}
