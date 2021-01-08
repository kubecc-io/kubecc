package cc

import (
	"errors"
	"os"
	"strings"

	types "github.com/cobalt77/kube-distcc/types"

	log "github.com/sirupsen/logrus"
)

type RunMode int
type ActionOpt string

const (
	RunError = iota
	RunLocal
	RunRemote
)

var (
	ProfileArgs *types.StringSet = types.NewStringSet( // --arg or --arg value
		"-fprofile-arcs",
		"-ftest-coverage",
		"--coverage",
		"-fprofile-correction",
	)
	ProfilePrefixArgs []string = []string{ // --arg=value or --arg value
		"-fprofile-generate",
		"-fprofile-use",
		"-fauto-profile",
	}
	LocalArgsWithValues *types.StringSet = types.NewStringSet(
		"-D",
		"-I",
		"-U",
		"-L",
		"-l",
		"-MF",
		"-MT",
		"-MQ",
		"-include",
		"-imacros",
		"-iprefix",
		"-iwithprefix",
		"-isystem",
		"-iwithprefixbefore",
		"-idirafter",
	)
	LocalArgsNoValues *types.StringSet = types.NewStringSet(
		"-undef",
		"-nostdinc",
		"-nostdinc++",
		"-MD",
		"-MMD",
		"-MG",
		"-MP",
	)
	LocalPrefixArgs []string = []string{
		"-Wp,",
		"-Wl,",
		"-D",
		"-U",
		"-I",
		"-l",
		"-L",
		"-MF",
		"-MT",
		"-MQ",
		"-isystem",
		"-stdlib",
	}

	Compile     ActionOpt = "-c"
	Preprocess  ActionOpt = "-E"
	GenAssembly ActionOpt = "-S"
	None        ActionOpt = ""
)

// IsActionOpt returns true if the given string
// is an ActionOpt, otherwise false.
func IsActionOpt(str string) bool {
	return str == Compile.String() ||
		str == Preprocess.String() ||
		str == GenAssembly.String()
}

func (opt ActionOpt) String() string {
	return string(opt)
}

// ArgsInfo represents GCC arguments
type ArgsInfo struct {
	Compiler       string
	Args           []string
	Mode           RunMode
	InputArgIndex  int
	OutputArgIndex int
	FlagIndexMap   map[string]int
}

// NewArgsInfoFromOS creates a new ArgsInfo struct from os.Args
func NewArgsInfoFromOS() *ArgsInfo {
	return &ArgsInfo{
		Compiler: os.Args[0],
		Args:     os.Args[1:],
	}
}

// NewArgsInfo creates a new ArgsInfo struct from the provided args
func NewArgsInfo(command string, args []string) *ArgsInfo {
	return &ArgsInfo{
		Compiler: command,
		Args:     args,
	}
}

// Parse will parse the arguments in argsInfo.Args and store indexes of
// several flags and values.
func (info *ArgsInfo) Parse() {
	info.InputArgIndex = -1
	info.OutputArgIndex = -1
	info.FlagIndexMap = map[string]int{}

	var (
		skip, seenOptC, seenOptS bool
		inputArg, outputArg      string
	)

	for i, a := range info.Args {
		if skip {
			skip = false
			continue
		}
		if a[0] == '-' {
			// Option argument
			switch {
			case a == "-E": // Preprocess
				info.FlagIndexMap[a] = i
				info.Mode = RunLocal
			case a == "-MD" || a == "-MMD":
				info.FlagIndexMap[a] = i
				// OK
			case a == "-MG" || a == "-MP":
				info.FlagIndexMap[a] = i
				// OK
			case strings.HasPrefix(a, "-MF") ||
				strings.HasPrefix(a, "-MT") ||
				strings.HasPrefix(a, "-MQ"):
				// OK
				if len(a) == 3 {
					skip = true // --arg value
				}
				info.FlagIndexMap[a[:3]] = i
				// --arg=value
			case strings.HasPrefix(a, "-M"):
				info.FlagIndexMap[a] = i
				log.Tracef("%s possibly implies -E, compiling locally", a)
				info.Mode = RunLocal
			case a == "-march=native":
				log.Tracef("Compiling locally for %s", a)
				info.Mode = RunLocal
			case a == "-mtune=native":
				log.Tracef("Compiling locally for %s", a)
				info.Mode = RunLocal
			case strings.HasPrefix(a, "-Wa,"):
				info.FlagIndexMap["-Wa"] = i
				if strings.Contains(a, ",-a") || strings.Contains(a, "--MD") {
					log.Tracef("Compiling locally for %s", a)
					info.Mode = RunLocal
				}
			case strings.HasPrefix(a, "-specs="):
				log.Tracef("Compiling locally for %s", a)
				info.Mode = RunLocal
			case a == "-S":
				info.FlagIndexMap[a] = i
				seenOptS = true
			case ProfileArgs.Contains(a):
				log.Trace("Compiling locally for profiling")
				info.Mode = RunLocal
			case func() bool {
				for _, prefix := range ProfilePrefixArgs {
					if strings.HasPrefix(a, prefix) {
						return true
					}
				}
				return false
			}():
				log.Trace("Compiling locally for profiling")
				info.Mode = RunLocal
			case a == "-frepo":
				log.Trace("Compiling locally, compiler will emit .rpo files")
				info.Mode = RunLocal
			case strings.HasPrefix(a, "-x"):
				if len(info.Args) > i+1 &&
					!strings.HasPrefix(info.Args[i+1], "c") &&
					!strings.HasPrefix(info.Args[i+1], "c++") &&
					!strings.HasPrefix(info.Args[i+1], "objective-c") &&
					!strings.HasPrefix(info.Args[i+1], "objective-c++") &&
					!strings.HasPrefix(info.Args[i+1], "go") {
					log.Tracef("Compiling locally, possibly complex -x arguments %s", info.Args[i+1])
					info.Mode = RunLocal
				}
				// OK
			case strings.HasPrefix(a, "-dr"):
				log.Tracef("Compiling locally for debug option %s", a)
				info.Mode = RunLocal
			case a == "-c":
				info.FlagIndexMap[a] = i
				seenOptC = true
			case a == "-o":
				info.FlagIndexMap[a] = i
			}
		} else {
			// Non-option argument (filename, etc.)
			if IsSourceFile(a) && IsActionOpt(info.Args[i-1]) {
				log.Tracef("Found input file %s", a)
				if inputArg != "" {
					log.Warn("Found multiple input files, possible invalid arguments")
					info.Mode = RunLocal
				}
				inputArg = a
				info.InputArgIndex = i
			} else if strings.HasSuffix(a, ".o") && info.Args[i-1] == "-o" {
				log.Tracef("Found output file %s", a)
				if outputArg != "" {
					log.Warn("Found multiple output files, possible invalid arguments")
					info.Mode = RunLocal
				}
				outputArg = a
				info.OutputArgIndex = i
			} else {
				if info.Args[i-1] == "-o" {
					log.Tracef("Found executable target %s", a)
				} else {
					log.Tracef("Found object link target %s", a)
				}
			}
		}
	}

	if !seenOptC && !seenOptS {
		log.Trace("Compiler not called for a compile operation")
		info.Mode = RunLocal
	}

	if inputArg == "" {
		log.Trace("No input file given")
		info.Mode = RunLocal
	}

	if ShouldRunLocal(inputArg) {
		log.Tracef("Compiling %s locally as a special case", inputArg)
		info.Mode = RunLocal
	}

	if outputArg == "" {
		/* This is a commandline like "gcc -c hello.c".  They want
		 * hello.o, but they don't say so.  For example, the Ethereal
		 * makefile does this.
		 *
		 * Note: this doesn't handle a.out, the other implied
		 * filename, but that doesn't matter because it would already
		 * be excluded by not having -c or -S.
		 */

		/* -S takes precedence over -c, because it means "stop after
		 * preprocessing" rather than "stop after compilation." */
		if seenOptS {
			outputArg = ReplaceExtension(inputArg, ".s")
		} else if seenOptC {
			outputArg = ReplaceExtension(inputArg, ".o")
		}
		if outputArg != "" {
			log.Tracef("No output file specified, assuming %s", outputArg)
			info.Args = append(info.Args, "-o", outputArg)
		}
	} else if outputArg == "-" {
		// Write to stdout, or a file called '-', can't be sure
		log.Tracef("Compiling %s locally, stdout output requested", inputArg)
	}

	if info.Mode == RunError {
		log.Tracef("Remote compilation enabled for this operation")
		info.Mode = RunRemote
	}
}

// SetCompiler sets the value of Args[0].
func (info *ArgsInfo) SetCompiler(compiler string) {
	info.Compiler = compiler
}

// SetActionOpt modifies the arguments to replace the action opt
// with a new one.
func (info *ArgsInfo) SetActionOpt(opt ActionOpt) error {
	replace := func(i int, oldOpt ActionOpt) {
		info.Args[i] = opt.String()
		delete(info.FlagIndexMap, oldOpt.String())
		info.FlagIndexMap[opt.String()] = i
	}
	if i, ok := info.FlagIndexMap[Compile.String()]; ok {
		replace(i, Compile)
	} else if i, ok := info.FlagIndexMap[GenAssembly.String()]; ok {
		replace(i, GenAssembly)
	} else if i, ok := info.FlagIndexMap[Preprocess.String()]; ok {
		replace(i, Preprocess)
	} else {
		return errors.New("No -c -S or -E args found")
	}
	return nil
}

// ActionOpt returns the current action according to the
// argument list.
func (info *ArgsInfo) ActionOpt() ActionOpt {
	if _, ok := info.FlagIndexMap[Compile.String()]; ok {
		return Compile
	} else if _, ok := info.FlagIndexMap[GenAssembly.String()]; ok {
		return GenAssembly
	} else if _, ok := info.FlagIndexMap[Preprocess.String()]; ok {
		return Preprocess
	}
	return None
}

// ReplaceOutputPath replaces the output path (the path after -o)
// with a new path.
func (info *ArgsInfo) ReplaceOutputPath(newPath string) error {
	if info.OutputArgIndex != -1 {
		info.Args[info.OutputArgIndex] = newPath
		return nil
	}
	return errors.New("No -o arg found")
}

// ReplaceInputPath replaces the input path (the path after the action opt)
// with a new path.
func (info *ArgsInfo) ReplaceInputPath(newPath string) error {
	if info.InputArgIndex != -1 {
		info.Args[info.InputArgIndex] = newPath
		return nil
	}
	return errors.New("No input arg found")
}

// SubstitutePreprocessorOptions expands gcc preprocessor arguments
// according to the following rules:
// 1. Replace "-Wp,-X,-Y,-Z" with "-X -Y -Z"
// 2. Replace "-Wp,-MD,path" or "-Wp,-MMD,path" with "-MF path"
func (info *ArgsInfo) SubstitutePreprocessorOptions() {
	for i := 0; i < len(info.Args); i++ {
		arg := info.Args[i]
		if !strings.HasPrefix(arg, "-Wp") {
			continue
		}
		split := strings.Split(arg, ",")[1:]
		for ii, str := range split {
			if str == "-MD" || str == "-MMD" {
				if ii == len(split)-1 || split[ii+1][0] == '-' {
					log.Warnf("Possibly invalid options, missing path after -Wp,%s", str)
				} else {
					split[ii] = "-MF"
				}
			}
		}
		left := append(append([]string(nil), info.Args[:i]...), split...) // deep copy
		info.Args = append(left, info.Args[i+1:]...)
		i = len(left) - 1
	}
	info.Parse()
}

// RemoveLocalArgs removes arguments that do not need to be
// sent to the remote agent for compiling. These args are
// related to preprocessing and linking.
func (info *ArgsInfo) RemoveLocalArgs() {
	newArgs := append([]string(nil), info.Args...)
	for i := 0; i < len(info.Args); i++ {
		arg := info.Args[i]
		if LocalArgsWithValues.Contains(arg) {
			i++ // Skip value (--arg value)
		} else if func() bool {
			for _, p := range LocalPrefixArgs {
				if strings.HasPrefix(arg, p) {
					return true
				}
			}
			return false
		}() {
			continue
		} else if LocalArgsNoValues.Contains(arg) {
			continue
		}
		newArgs = append(newArgs, arg)
	}
	info.Args = newArgs
	info.Parse()
}
