package consumer

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
)

func DispatchAndWait(ctx context.Context, cc *grpc.ClientConn) {
	lg := meta.Log(ctx)

	lg.Info("Dispatching to consumerd")

	consumerd := types.NewConsumerdClient(cc)
	wd, err := os.Getwd()
	if err != nil {
		lg.Fatal(err.Error())
	}
	stdin := new(bytes.Buffer)

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		_, err := io.Copy(stdin, os.Stdin)
		if err != nil {
			lg.With(
				zap.Error(err),
			).Fatal("Error forwarding stdin")
		}
	}

	var compilerPath string
	if filepath.IsAbs(os.Args[0]) {
		compilerPath = os.Args[0]
	} else {
		compilerPath = findCompilerOrDie(ctx)
	}
	resp, err := consumerd.Run(ctx, &types.RunRequest{
		Compiler: &types.RunRequest_Path{
			Path: compilerPath,
		},
		Args:    os.Args[1:],
		Env:     os.Environ(),
		UID:     uint32(os.Getuid()),
		GID:     uint32(os.Getgid()),
		Stdin:   stdin.Bytes(),
		WorkDir: wd,
	}, grpc.WaitForReady(true), grpc.UseCompressor(gzip.Name))
	if err != nil {
		lg.With(zap.Error(err)).Error("Dispatch error")
		os.Exit(1)
	}
	if _, err := io.Copy(os.Stdout, bytes.NewReader(resp.Stdout)); err != nil {
		lg.With(
			zap.Error(err),
		).Fatal("Error forwarding stdout")
	}
	if _, err := io.Copy(os.Stderr, bytes.NewReader(resp.Stderr)); err != nil {
		lg.With(
			zap.Error(err),
		).Fatal("Error forwarding stderr")
	}
	os.Exit(int(resp.ReturnCode))
}
