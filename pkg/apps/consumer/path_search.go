package consumer

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cobalt77/kubecc/pkg/meta"
	"go.uber.org/zap"
)

func samePath(a, b string) bool {
	var err error
	a, err = filepath.EvalSymlinks(a)
	if err != nil {
		return false
	}
	b, err = filepath.EvalSymlinks(b)
	if err != nil {
		return false
	}
	return a == b
}

func findCompilerOrDie(ctx context.Context) string {
	lg := meta.Log(ctx)
	basename := filepath.Base(os.Args[0])
	self, err := os.Executable()
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Could not locate current executable")
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Could not evaluate symlinks")
	}
	compiler, err := lookPath(basename, self)
	if err != nil {
		lg.With(zap.Error(err)).Fatal("Could not find suitable compiler")
	}
	return compiler
}

// Code below is copied from os/exec/lp_unix.go, but we need to modify it
// to exclude the currently running executable to avoid infinite loops

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	return fs.ErrPermission
}

func lookPath(file string, self string) (string, error) {
	if strings.Contains(file, "/") {
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", &exec.Error{Name: file, Err: err}
	}
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if samePath(path, self) {
			continue
		}
		if err := findExecutable(path); err == nil {
			return path, nil
		}
	}
	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}
