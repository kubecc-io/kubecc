package main

import (
	"errors"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

type RunMode int

const (
	RunLocal = iota
	RunRemote
)

var (
	ProfileArgs *StringSet = NewStringSet( // --arg or --arg value
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
)

type argsInfo struct {
	Args           []string
	Compiler       string
	Mode           RunMode
	InputArgIndex  int
	OutputArgIndex int
	FlagIndexMap   map[string]int
}

func NewArgsInfo() *argsInfo {
	return &argsInfo{
		Args:           os.Args[1:],
		Compiler:       os.Args[0],
		InputArgIndex:  -1,
		OutputArgIndex: -1,
		FlagIndexMap:   make(map[string]int),
	}
}

// DetermineMode will attempt to determine if a remote compile is feasable
// for the given arguments.
//
// This logic is copied more-or-less verbatim from distcc, arg.c
func (info *argsInfo) Parse() {
	skip := false
	seenOptC := false
	seenOptS := false
	inputArg := ""
	outputArg := ""

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
			if isSourceFile(a) {
				log.Tracef("Found input file %s", a)
				if inputArg != "" {
					log.Warning("Found multiple input files, possible invalid arguments")
					info.Mode = RunLocal
				}
				inputArg = a
				info.InputArgIndex = i
			} else if strings.HasSuffix(a, ".o") {
				log.Tracef("Found output file %s", a)
				if outputArg != "" {
					log.Warning("Found multiple output files, possible invalid arguments")
					info.Mode = RunLocal
				}
				outputArg = a
				info.OutputArgIndex = i
			}
		}
	}

	if !seenOptC && !seenOptS {
		log.Info("Compiler not called for a compile operation")
		info.Mode = RunLocal
	}

	if inputArg == "" {
		log.Info("No input file given")
		info.Mode = RunLocal
	}

	if shouldRunLocal(inputArg) {
		log.Infof("Compiling %s locally as a special case", inputArg)
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
			outputArg = replaceExtension(inputArg, ".s")
		} else if seenOptC {
			outputArg = replaceExtension(inputArg, ".o")
		}
		log.Infof("No output file specified, assuming %s", outputArg)
		info.Args = append(info.Args, "-o", outputArg)
	} else if outputArg == "-" {
		// Write to stdout, or a file called '-', can't be sure
		log.Infof("Compiling %s locally, stdout output requested", inputArg)
	}

	info.Mode = RunRemote
}

func (info *argsInfo) SetCompiler(compiler string) {
	info.Compiler = compiler
}

func (info *argsInfo) SetActionOpt(opt string) error {
	if i, ok := info.FlagIndexMap["-c"]; ok {
		info.Args[i] = opt
	} else if i, ok := info.FlagIndexMap["-S"]; ok {
		info.Args[i] = opt
	} else if i, ok := info.FlagIndexMap["-E"]; ok {
		info.Args[i] = opt
	} else {
		return errors.New("No -c -S or -E args found")
	}
	return nil
}

func (info *argsInfo) ReplaceOutputPath(newPath string) error {
	if info.OutputArgIndex != -1 {
		info.Args[info.OutputArgIndex] = newPath
		return nil
	}
	return errors.New("No -o arg found")
}

// Replace -Wp,-X,-Y,-Z -> "-X -Y -Z"
// As a special case, replace -Wp,-MD,path or -Wp,-MMD,path -> "-MF path"
func (info *argsInfo) SubstitutePreprocessorOptions() {
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
		left := append(append([]string(nil), info.Args[:i]...), split...)
		info.Args = append(left, info.Args[i+1:]...)
		i = len(left) - 1
	}
	info.Parse() // Reparse the new args
}
