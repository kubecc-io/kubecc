package main

import (
	"bytes"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// Preprocess a file based on the given args.
func Preprocess(args *argsInfo) ([]byte, error) {
	log.Trace("Preprocessing")
	args.SubstitutePreprocessorOptions()
	args.SetActionOpt("-E")
	args.ReplaceOutputPath("-")
	args.SetCompiler("/usr/bin/gcc")
	cmd := exec.Command(args.Compiler, args.Args...)
	log.Trace(cmd.Args)
	buf := new(bytes.Buffer)
	cmd.Stdout = buf
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
