package consumer

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cobalt77/kubecc/internal/logkc"
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
	lg := logkc.LogFromContext(ctx)
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

// ErrNotFound is the error resulting if a path search failed to find an executable file.
var errNotFound = errors.New("executable file not found in $PATH")

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
		return "", &exec.Error{file, err}
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
	return "", &exec.Error{file, exec.ErrNotFound}
}
