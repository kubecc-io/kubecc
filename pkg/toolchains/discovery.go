package toolchains

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
	mapset "github.com/deckarep/golang-set"
	"go.uber.org/zap"
)

var picCheck = `
#if defined __PIC__ || defined __pic__ || defined PIC || defined pic            
# error                                                    
#endif                                                                          
`

func isPicDefault(compiler string) (bool, error) {
	cmd := exec.Command(compiler, "-E -o /dev/null -")
	stderrBuf := new(bytes.Buffer)
	cmd.Stdin = strings.NewReader(picCheck)
	cmd.Stdout = nil
	cmd.Stderr = stderrBuf
	cmd.Env = []string{}
	err := cmd.Run()
	if _, ok := err.(*exec.ExitError); ok {
		return false, err
	}
	if err == nil {
		return false, nil
	}
	return strings.Contains(stderrBuf.String(), "#error"), nil
}

func targetArch(compiler string) (string, error) {
	cmd := exec.Command(compiler, "-dumpmachine")
	stdoutBuf := new(bytes.Buffer)
	cmd.Stdin = nil
	cmd.Stdout = stdoutBuf
	cmd.Stderr = nil
	cmd.Env = []string{}
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	triple := strings.Split(stdoutBuf.String(), "-")
	if len(triple) != 3 {
		return "", errors.New("GCC returned an invalid target triple with -dumpmachine")
	}
	return triple[0], nil
}

func isCanonical(compiler string) bool {
	basename := filepath.Base(compiler)
	parts := strings.SplitN(basename, "-", 4)
	// If the dereferenced symlink's filename is a canonical
	// host triple concatenated with the binary name
	// ex: x86_64-linux-gnu-gcc
	// We aren't checking the actual contents of the host triple,
	// just that the filename matches this pattern.
	// Some binaries have dashes in the name (i.e. gcc-10) which
	// would make the filename something like x86_64-linux-gnu-gcc-10
	// so the split count is capped at 4. The parts would then be
	// ["x86_64", "linux", "gnu", "gcc-10"] and we can match the
	// given prefix against the "gcc-10" part to see if it is the
	// expected binary.
	return len(parts) == 4 &&
		(strings.HasPrefix(parts[3], "gcc") ||
			strings.HasPrefix(parts[3], "g++") ||
			strings.HasPrefix(parts[3], "clang"))
}

func compilerKindAndLanguage(
	compiler string,
) (types.ToolchainKind, types.ToolchainLang, error) {
	switch base := filepath.Base(compiler); {
	case strings.Contains(base, "clang++"):
		return types.Clang, types.CXX, nil
	case strings.Contains(base, "clang"):
		return types.Clang, types.C, nil
	case strings.Contains(base, "g++"):
		return types.Gnu, types.CXX, nil
	case strings.Contains(base, "gcc"):
		return types.Gnu, types.C, nil
	}
	return 0, 0, errors.New("Unknown compiler")
}

func FindToolchains(ctx context.Context) (tcs []*types.Toolchain) {
	lg := logkc.LogFromContext(ctx)
	tcs = []*types.Toolchain{}
	searchPaths := mapset.NewSet()
	addPath := func(set mapset.Set, path string) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			lg.With("path", path).Debug("Symlink eval failed")
			return
		}
		set.Add(realPath)
	}
	addPath(searchPaths, "/usr/bin")
	addPath(searchPaths, "/usr/local/bin")
	addPath(searchPaths, "/bin")

	if paths, ok := os.LookupEnv("PATH"); ok {
		for _, path := range strings.Split(paths, ":") {
			addPath(searchPaths, path)
		}
	}

	// Matches the following:
	// (beginning of line)                 followed by
	// (a host triple) or (empty)          followed by
	// (one of: gcc, g++, clang, clang++)  followed by
	// ('-' and a number) or (empty)       followed by
	// (end of line)
	pattern := `^(?:\w+\-\w+\-\w+\-)?(?:(?:g([c+])\1)|(?:clang(?:\+{2})?))(?:-[\d.]+)?$`
	re := regexp.MustCompile(pattern)

	compilers := mapset.NewSet()
	for p := range searchPaths.Iter() {
		dirname := p.(string)
		infos, err := ioutil.ReadDir(dirname)
		if err != nil {
			lg.With(zap.Error(err)).Debug("Error listing directory contents")
			continue
		}
		for _, info := range infos {
			if re.Match([]byte(filepath.Base(info.Name()))) {
				addPath(compilers, filepath.Join(dirname, info.Name()))
			}
		}
	}

	for c := range compilers.Iter() {
		compiler := c.(string)
		canonical := isCanonical(compiler)
		arch, err := targetArch(compiler)
		if err != nil {
			lg.With(
				"compiler", compiler,
				zap.Error(err),
			).Warn("Could not determine target arch")
			continue
		}
		pic, err := isPicDefault(compiler)
		if err != nil {
			lg.With(
				"compiler", compiler,
				zap.Error(err),
			).Warn("Could not determine compiler PIC defaults")
			continue
		}
		kind, lang, err := compilerKindAndLanguage(compiler)
		if err != nil {
			lg.With(
				"compiler", compiler,
				zap.Error(err),
			).Warn("Could not determine compiler kind (gcc/clang) or language (c/cxx)")
			continue
		}
		tcs = append(tcs, &types.Toolchain{
			Kind:       kind,
			Lang:       lang,
			Canonical:  canonical,
			Executable: compiler,
			TargetArch: arch,
			PicDefault: pic,
		})
	}

	return
}
