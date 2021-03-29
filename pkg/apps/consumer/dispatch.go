/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package consumer

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/kubecc-io/kubecc/pkg/meta"
	"github.com/kubecc-io/kubecc/pkg/types"
	"go.uber.org/zap"
	"golang.org/x/term"
	"google.golang.org/grpc"
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
	}, grpc.WaitForReady(true))
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
