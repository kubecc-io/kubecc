package cc

import (
	"errors"
	"os"
	"strings"

	"github.com/cobalt77/kubecc/pkg/tools"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type RunMode int
type ActionOpt string

const (
	Unset RunMode = iota
	RunLocal
	RunRemote
)

func (rm RunMode) String() string {
	switch rm {
	case Unset:
		return "RunError"
	case RunLocal:
		return "RunLocal"
	case RunRemote:
		return "RunRemote"
	}
	return "<unk>"
}

var (
	ProfileArgs *tools.StringSet = tools.NewStringSet( // --arg or --arg value
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
	LocalArgsWithValues *tools.StringSet = tools.NewStringSet(
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
	LocalArgsNoValues *tools.StringSet = tools.NewStringSet(
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
	Args           []string
	Mode           RunMode
	InputArgIndex  int
	OutputArgIndex int
	FlagIndexMap   map[string]int

	log *zap.Logger
}

// NewArgsInfoFromOS creates a new ArgsInfo struct from os.Args
func NewArgsInfoFromOS(logger *zap.Logger) *ArgsInfo {
	return &ArgsInfo{
		Args: append([]string(nil), os.Args[1:]...), // deep copy
		log:  logger,
	}
}

// NewArgsInfo creates a new ArgsInfo struct from the provided args
// Args should not include the command
func NewArgsInfo(args []string, logger *zap.Logger) *ArgsInfo {
	return &ArgsInfo{
		Args: args,
		log:  logger,
	}
}

func (a *ArgsInfo) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddArray("args", types.NewStringSliceEncoder(a.Args))
	enc.AddString("mode", a.Mode.String())
	return nil
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

	log := info.log

	for i, a := range info.Args {
		lg := info.log.With(zap.String("arg", a))
		if skip {
			skip = false
			continue
		}
		if a[0] == '-' && len(a) > 1 {
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
				lg.Debug("-E possibly implied, compiling locally")
				info.Mode = RunLocal
			case a == "-march=native":
				info.Mode = RunLocal
			case a == "-mtune=native":
				info.Mode = RunLocal
			case strings.HasPrefix(a, "-Wa,"):
				info.FlagIndexMap["-Wa"] = i
				if strings.Contains(a, ",-a") || strings.Contains(a, "--MD") {
					info.Mode = RunLocal
				}
			case strings.HasPrefix(a, "-specs="):
				info.Mode = RunLocal
			case a == "-S":
				info.FlagIndexMap[a] = i
				seenOptS = true
			case ProfileArgs.Contains(a):
				lg.Debug("Compiling locally for profiling")
				info.Mode = RunLocal
			case func() bool {
				for _, prefix := range ProfilePrefixArgs {
					if strings.HasPrefix(a, prefix) {
						return true
					}
				}
				return false
			}():
				lg.Debug("Compiling locally for profiling")
				info.Mode = RunLocal
			case a == "-frepo":
				lg.Debug("Compiling locally, compiler will emit .rpo files")
				info.Mode = RunLocal
			case strings.HasPrefix(a, "-x"):
				if len(info.Args) > i+1 &&
					!strings.HasPrefix(info.Args[i+1], "c") &&
					!strings.HasPrefix(info.Args[i+1], "c++") &&
					!strings.HasPrefix(info.Args[i+1], "objective-c") &&
					!strings.HasPrefix(info.Args[i+1], "objective-c++") &&
					!strings.HasPrefix(info.Args[i+1], "go") {
					lg.Debug("Compiling locally, possibly complex -x arguments")
					info.Mode = RunLocal
				}
				skip = true
				// OK
			case strings.HasPrefix(a, "-dr"):
				info.Mode = RunLocal
			case a == "-c":
				info.FlagIndexMap[a] = i
				seenOptC = true
			case a == "-o":
				info.FlagIndexMap[a] = i

				if i == len(info.Args)-1 {
					lg.Error("-o found as the last argument?")
					info.Mode = RunLocal
					break
				}

				if strings.HasSuffix(a, ".o") {
					// Args of the form `-o something.o`
					lg.Debug("Found output file")
					if outputArg != "" {
						lg.Warn("Found multiple output files, possible invalid arguments")
						info.Mode = RunLocal
					}
				} else {
					// Args of the form `-o something`
					lg.Debug("Found executable target")
					if outputArg != "" {
						lg.Warn("Found multiple executable targets, possible invalid arguments")
						info.Mode = RunLocal
					}
				}
				outputArg = info.Args[i+1]
				info.OutputArgIndex = i + 1
				skip = true
			}
		} else {
			isSource := IsSourceFile(a)
			if isSource || a == "-" { // Won't come up after -o or -x due to above logic
				lg.Debug("Found input file")
				if inputArg != "" {
					lg.Warn("Found multiple input files, compiling locally")
					info.Mode = RunLocal
				}
				inputArg = a
				info.InputArgIndex = i
			}
		}
	}

	if !seenOptC && !seenOptS && info.InputArgIndex == -1 {
		log.Debug("Compiler not called for a compile operation")
		info.Mode = RunLocal
	}

	if info.InputArgIndex == -1 {
		log.Debug("No input file given")
		info.Mode = RunLocal
	}

	if ShouldRunLocal(inputArg) {
		log.With(zap.String("input", inputArg)).
			Debug("Compiling %s locally as a special case")
		info.Mode = RunLocal
	}

	if outputArg == "" {
		if seenOptC || seenOptS {
			// If -c or -S is provided but no output filename is given,
			// the output file is assumed to be the same name as the
			// input, but with a '.o' or '.s' extension, respectively.

			if seenOptS {
				outputArg = ReplaceExtension(inputArg, ".s")
			} else if seenOptC {
				outputArg = ReplaceExtension(inputArg, ".o")
			}
			if outputArg != "" {
				log.With(zap.String("output", outputArg)).
					Debug("No output file specified, adding one to match input")
				info.Args = append(info.Args, "-o", outputArg)
				info.OutputArgIndex = len(info.Args) - 1
			}
		} else if inputArg != "" {
			// Input arg but no output and no action opt = a.out
			outputArg = "a.out"
			info.Args = append(info.Args, "-o", "a.out")
			info.OutputArgIndex = len(info.Args) - 1
		}
	}

	// Nothing set so far, allow remote
	if info.Mode == Unset {
		info.Mode = RunRemote
	}

	switch info.Mode {
	case RunLocal:
		log.Debug("Remote compilation disabled")
	case RunRemote:
		log.Debug("Remote compilation enabled")
	}
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
// If the new input path is '-', this function adds '-x <language>' to the arguments
func (info *ArgsInfo) ReplaceInputPath(newPath string) error {
	if info.InputArgIndex != -1 {
		old := info.Args[info.InputArgIndex]
		if old == newPath {
			return nil
		}
		info.Args[info.InputArgIndex] = newPath
		if newPath == "-" {
			info.log.Debug("Replacing input flag with '-', adding language flag to args")
			err := info.PrependLanguageFlag(old)
			if err != nil {
				return err
			}
		}
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
					info.log.Sugar().Warnf("Possibly invalid options, missing path after -Wp,%s", str)
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
	newArgs := []string{}
	for i := 0; i < len(info.Args); i++ {
		arg := info.Args[i]
		if LocalArgsWithValues.Contains(arg) {
			i++ // Skip value (--arg value)
			continue
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

func (info *ArgsInfo) PrependLanguageFlag(input string) error {
	lang, err := SourceFileLanguage(input)
	if err != nil {
		return err
	}
	info.Args = append([]string{"-x", lang}, info.Args...)
	info.Parse()
	return nil
}
