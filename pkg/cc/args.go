package cc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
	mapset "github.com/deckarep/golang-set"
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
	ProfileArgs = mapset.NewSet( // --arg or --arg value
		"-fprofile-arcs",
		"-ftest-coverage",
		"--coverage",
		"-fprofile-correction",
	)
	ProfilePrefixArgs = []string{ // --arg=value or --arg value
		"-fprofile-generate",
		"-fprofile-use",
		"-fauto-profile",
	}
	LocalArgsWithValues = mapset.NewSet(
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
	LocalArgsNoValues = mapset.NewSet(
		"-undef",
		"-nostdinc",
		"-nostdinc++",
		"-MD",
		"-MMD",
		"-MG",
		"-MP",
	)
	LocalPrefixArgs = []string{
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

// ArgParser represents GCC arguments
type ArgParser struct {
	lg             *zap.SugaredLogger
	Args           []string
	Mode           RunMode
	InputArgIndex  int
	OutputArgIndex int
	FlagIndexMap   map[string]int
}

// NewArgsInfoFromOS creates a new ArgsInfo struct from os.Args
func NewArgParserFromOS(ctx context.Context) *ArgParser {
	return NewArgParser(ctx, append([]string(nil), os.Args[1:]...)) // deep copy
}

// NewArgParser creates a new ArgsInfo struct from the provided args
// Args should NOT include the command (argv[0])
func NewArgParser(ctx context.Context, args []string) *ArgParser {
	return &ArgParser{
		lg:   logkc.LogFromContext(ctx),
		Args: args,
	}
}

func (a *ArgParser) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddArray("args", types.NewStringSliceEncoder(a.Args))
	enc.AddString("mode", a.Mode.String())
	return nil
}

// Parse will parse the arguments in argsInfo.Args and store indexes of
// several flags and values.
func (ap *ArgParser) Parse() {
	ap.InputArgIndex = -1
	ap.OutputArgIndex = -1
	ap.FlagIndexMap = map[string]int{}

	var (
		skip, seenOptC, seenOptS, seenOptE bool
		inputArg, outputArg                string
	)

	for i, a := range ap.Args {
		lg := ap.lg.With(zap.String("arg", a))
		if skip {
			skip = false
			continue
		}
		if a[0] == '-' && len(a) > 1 {
			// Option argument
			switch {
			case a == "-E": // Preprocess
				ap.FlagIndexMap[a] = i
				ap.Mode = RunLocal
				seenOptE = true
			case a == "-MD" || a == "-MMD":
				ap.FlagIndexMap[a] = i
				// OK
			case a == "-MG" || a == "-MP":
				ap.FlagIndexMap[a] = i
				// OK
			case strings.HasPrefix(a, "-MF") ||
				strings.HasPrefix(a, "-MT") ||
				strings.HasPrefix(a, "-MQ"):
				// OK
				if len(a) == 3 {
					skip = true // --arg value
				}
				ap.FlagIndexMap[a[:3]] = i
				// --arg=value
			case strings.HasPrefix(a, "-M"):
				ap.FlagIndexMap[a] = i
				lg.Debug("-E possibly implied, compiling locally")
				ap.Mode = RunLocal
			case a == "-march=native":
				ap.Mode = RunLocal
			case a == "-mtune=native":
				ap.Mode = RunLocal
			case strings.HasPrefix(a, "-Wa,"):
				ap.FlagIndexMap["-Wa"] = i
				if strings.Contains(a, ",-a") || strings.Contains(a, "--MD") {
					ap.Mode = RunLocal
				}
			case strings.HasPrefix(a, "-specs="):
				ap.Mode = RunLocal
			case a == "-S":
				ap.FlagIndexMap[a] = i
				seenOptS = true
			case ProfileArgs.Contains(a):
				lg.Debug("Compiling locally for profiling")
				ap.Mode = RunLocal
			case func() bool {
				for _, prefix := range ProfilePrefixArgs {
					if strings.HasPrefix(a, prefix) {
						return true
					}
				}
				return false
			}():
				lg.Debug("Compiling locally for profiling")
				ap.Mode = RunLocal
			case a == "-frepo":
				lg.Debug("Compiling locally, compiler will emit .rpo files")
				ap.Mode = RunLocal
			case strings.HasPrefix(a, "-x"):
				if len(ap.Args) > i+1 &&
					!strings.HasPrefix(ap.Args[i+1], "c") &&
					!strings.HasPrefix(ap.Args[i+1], "c++") &&
					!strings.HasPrefix(ap.Args[i+1], "objective-c") &&
					!strings.HasPrefix(ap.Args[i+1], "objective-c++") &&
					!strings.HasPrefix(ap.Args[i+1], "go") {
					lg.Debug("Compiling locally, possibly complex -x arguments")
					ap.Mode = RunLocal
				}
				if a == "-x" {
					skip = true
				}
				// OK
			case strings.HasPrefix(a, "-dr"):
				ap.Mode = RunLocal
			case a == "-c":
				ap.FlagIndexMap[a] = i
				seenOptC = true
			case a == "-o":
				ap.FlagIndexMap[a] = i

				if i == len(ap.Args)-1 {
					lg.Error("-o found as the last argument?")
					ap.Mode = RunLocal
					break
				}
				next := ap.Args[i+1]
				ext := filepath.Ext(next)
				if ext == ".o" {
					// Args of the form `-o something.o`
					ap.lg.With(zap.String("path", next)).
						Debug("Found output file")
					if outputArg != "" {
						ap.lg.With(zap.String("path", next)).
							Warn("Found multiple output files, possible invalid arguments")
						ap.Mode = RunLocal
					}
				} else if ext == "" {
					// Args of the form `-o something`
					ap.lg.With(zap.String("path", next)).
						Debug("Found executable target")
					if outputArg != "" {
						ap.lg.With(zap.String("path", next)).
							Warn("Found multiple executable targets, possible invalid arguments")
					}
					ap.Mode = RunLocal
				}
				outputArg = ap.Args[i+1]
				ap.OutputArgIndex = i + 1
				skip = true
			}
		} else {
			isSource := IsSourceFile(a)
			if isSource || a == "-" { // Won't come up after -o or -x due to above logic
				lg.Debug("Found input file")
				if inputArg != "" {
					lg.Warn("Found multiple input files, compiling locally")
					ap.Mode = RunLocal
				}
				inputArg = a
				ap.InputArgIndex = i
			}
		}
	}

	if !seenOptC && !seenOptS && ap.InputArgIndex == -1 {
		ap.lg.Debug("Compiler not called for a compile operation")
		ap.Mode = RunLocal
	}

	if ap.InputArgIndex == -1 {
		ap.lg.Debug("No input file given")
		ap.Mode = RunLocal
	}

	if ShouldRunLocal(inputArg) {
		ap.lg.With(zap.String("input", inputArg)).
			Debug("Compiling %s locally as a special case")
		ap.Mode = RunLocal
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
				ap.lg.With(zap.String("output", outputArg)).
					Debug("No output file specified, adding one to match input")
				ap.Args = append(ap.Args, "-o", outputArg)
				ap.OutputArgIndex = len(ap.Args) - 1
			}
		} else if inputArg != "" && !seenOptE {
			// If preprocessing, output goes to stdout
			outputArg = "a.out"
			ap.Args = append(ap.Args, "-o", "a.out")
			ap.OutputArgIndex = len(ap.Args) - 1
		}
	}

	// Nothing set so far, allow remote
	if ap.Mode == Unset {
		ap.Mode = RunRemote
	}

	switch ap.Mode {
	case RunLocal:
		ap.lg.Debug("Remote compilation disabled")
	case RunRemote:
		ap.lg.Debug("Remote compilation enabled")
	}
}

// SetActionOpt modifies the arguments to replace the action opt
// with a new one.
func (ap *ArgParser) SetActionOpt(opt ActionOpt) error {
	replace := func(i int, oldOpt ActionOpt) {
		ap.Args[i] = opt.String()
		delete(ap.FlagIndexMap, oldOpt.String())
		ap.FlagIndexMap[opt.String()] = i
	}
	if i, ok := ap.FlagIndexMap[Compile.String()]; ok {
		replace(i, Compile)
	} else if i, ok := ap.FlagIndexMap[GenAssembly.String()]; ok {
		replace(i, GenAssembly)
	} else if i, ok := ap.FlagIndexMap[Preprocess.String()]; ok {
		replace(i, Preprocess)
	} else {
		return errors.New("No -c -S or -E args found")
	}
	return nil
}

// ActionOpt returns the current action according to the
// argument list.
func (ap *ArgParser) ActionOpt() ActionOpt {
	if _, ok := ap.FlagIndexMap[Compile.String()]; ok {
		return Compile
	} else if _, ok := ap.FlagIndexMap[GenAssembly.String()]; ok {
		return GenAssembly
	} else if _, ok := ap.FlagIndexMap[Preprocess.String()]; ok {
		return Preprocess
	}
	return None
}

// ReplaceOutputPath replaces the output path (the path after -o)
// with a new path.
func (ap *ArgParser) ReplaceOutputPath(newPath string) error {
	if ap.OutputArgIndex != -1 {
		ap.Args[ap.OutputArgIndex] = newPath
		return nil
	}
	return errors.New("No -o arg found")
}

// ReplaceInputPath replaces the input path (the path after the action opt)
// with a new path.
// If the new input path is '-', this function adds '-x <language>' to the arguments
func (ap *ArgParser) ReplaceInputPath(newPath string) error {
	if ap.InputArgIndex != -1 {
		old := ap.Args[ap.InputArgIndex]
		if old == newPath {
			return nil
		}
		ap.Args[ap.InputArgIndex] = newPath
		if newPath == "-" {
			ap.lg.Debug("Replacing input flag with '-', adding language flag to args")
			err := ap.PrependLanguageFlag(old)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("No input arg found")
}

// ConfigurePreprocessorOptions expands gcc preprocessor arguments
// according to the following rules:
// 1. Replace "-Wp,-X,-Y,-Z" with "-X -Y -Z"
// 2. Replace "-Wp,-MD,path" or "-Wp,-MMD,path" with "-M[M]D -MF path"
// It also adds the -fdirectives-only option.
func (ap *ArgParser) ConfigurePreprocessorOptions() {
	for i := 0; i < len(ap.Args); i++ {
		arg := ap.Args[i]
		if !strings.HasPrefix(arg, "-Wp") {
			continue
		}
		split := strings.Split(arg, ",")[1:]
		for ii := 0; ii < len(split); ii++ {
			str := split[ii]
			if str == "-MD" || str == "-MMD" {
				if ii == len(split)-1 || split[ii+1][0] == '-' {
					ap.lg.Warnf("Possibly invalid options, missing path after -Wp,%s", str)
				} else {
					split = append(split[:ii+1], split[ii:]...)
					split[ii+1] = "-MF" // very important, without this GCC will [DATA EXPUNGED]
					ii++
				}
			}
		}
		left := append(append([]string(nil), ap.Args[:i]...), split...) // deep copy
		ap.Args = append(left, ap.Args[i+1:]...)
		i = len(left) - 1
	}
	ap.Args = append(ap.Args, "-fdirectives-only")
	ap.Parse()
}

// RemoveLocalArgs removes arguments that do not need to be
// sent to the remote agent for compiling. These args are
// related to preprocessing and linking.
// It also adds -fpreprocessed
func (ap *ArgParser) RemoveLocalArgs() {
	newArgs := []string{}
	for i := 0; i < len(ap.Args); i++ {
		arg := ap.Args[i]
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
	ap.Args = append(newArgs, "-fpreprocessed")
	ap.Parse()
}

func (ap *ArgParser) PrependLanguageFlag(input string) error {
	lang, err := SourceFileLanguage(input)
	if err != nil {
		return err
	}
	ap.Args = append([]string{"-x", lang}, ap.Args...)
	ap.Parse()
	return nil
}
