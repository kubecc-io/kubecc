package main

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/cobalt77/kubecc/internal/lll"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
)

func dispatchAndWait(cc *grpc.ClientConn) {
	lll.Info("Dispatching to consumerd")

	consumerd := types.NewConsumerdClient(cc)
	wd, err := os.Getwd()
	if err != nil {
		lll.Fatal(err.Error())
	}
	stdin := new(bytes.Buffer)

	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		io.Copy(stdin, os.Stdin)
	}

	resp, err := consumerd.Run(context.Background(), &types.RunRequest{
		WorkDir: wd,
		Args:    os.Args[1:],
		Env:     os.Environ(),
		UID:     uint32(os.Getuid()),
		GID:     uint32(os.Getgid()),
		Stdin:   stdin.Bytes(),
	}, grpc.WaitForReady(true), grpc.UseCompressor(gzip.Name))
	if err != nil {
		lll.With(zap.Error(err)).Error("Dispatch error")
		os.Exit(1)
	}
	io.Copy(os.Stdout, bytes.NewReader(resp.Stdout))
	io.Copy(os.Stderr, bytes.NewReader(resp.Stderr))
	os.Exit(int(resp.ReturnCode))
}
