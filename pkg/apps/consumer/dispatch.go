package consumer

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/cobalt77/kubecc/internal/logkc"
	"github.com/cobalt77/kubecc/pkg/types"
	"go.uber.org/zap"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
)

func DispatchAndWait(ctx context.Context, cc *grpc.ClientConn) {
	lg := logkc.LogFromContext(ctx)

	lg.Info("Dispatching to consumerd")

	consumerd := types.NewConsumerdClient(cc)
	wd, err := os.Getwd()
	if err != nil {
		lg.Fatal(err.Error())
	}
	stdin := new(bytes.Buffer)

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		io.Copy(stdin, os.Stdin)
	}

	var compilerPath string
	if filepath.IsAbs(os.Args[0]) {
		compilerPath = os.Args[0]
	} else {
		compilerPath = findCompilerOrDie(ctx)
	}
	resp, err := consumerd.Run(context.Background(), &types.RunRequest{
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
	io.Copy(os.Stdout, bytes.NewReader(resp.Stdout))
	io.Copy(os.Stderr, bytes.NewReader(resp.Stderr))
	os.Exit(int(resp.ReturnCode))
}
