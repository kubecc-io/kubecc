//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kralicky/ragu/pkg/ragu"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = All

var (
	operatorSdkPath   = "github.com/operator-framework/operator-sdk/cmd/operator-sdk@latest"
	mockGenPath       = "github.com/golang/mock/mockgen@latest"
	controllerGenPath = "https://github.com/kralicky/controller-tools/releases/download/v0.7.0-patched/controller-gen"
	ginkgoPath        = "github.com/onsi/ginkgo/ginkgo@ver2"
)

func All() {
	mg.SerialDeps(Setup, Generate, Vet, Build)
}

func Generate() {
	mg.Deps(GenProto, MockGen)
	mg.SerialDeps(ControllerGen)
}

func Vet() error {
	return sh.RunV("go", "vet", "./...")
}

func Build() error {
	return sh.RunWithV(map[string]string{
		"CGO_ENABLED": "0",
	}, mg.GoCmd(), "build", "-ldflags", `-w -s`, "-o", "./bin/kubecc", "./cmd/kubecc")
}

func Setup() error {
	if _, err := exec.LookPath("controller-gen"); err != nil {
		fmt.Println("Installing dependency: controller-gen")
		gopath, err := sh.Output("go", "env", "GOPATH")
		if err != nil {
			return err
		}
		gobin := filepath.Join(gopath, "bin")
		err = sh.Run("curl", "-sfL", controllerGenPath, "-o", filepath.Join(gobin, "controller-gen"))
		if err != nil {
			return err
		}
		if err := sh.Run("chmod", "+x", filepath.Join(gobin, "controller-gen")); err != nil {
			return err
		}
	}
	if _, err := exec.LookPath("mockgen"); err != nil {
		fmt.Println("Installing dependency: mockgen")
		if err := sh.RunV(mg.GoCmd(), "install", mockGenPath); err != nil {
			return err
		}
	}
	return nil
}

func SetupDev() error {
	mg.Deps(Setup)
	if _, err := exec.LookPath("operator-sdk"); err != nil {
		fmt.Println("Installing dependency: operator-sdk")
		return sh.RunV(mg.GoCmd(), "install", operatorSdkPath)
	}
	return nil
}

func SetupTest() error {
	if _, err := exec.LookPath("ginkgo"); err != nil {
		fmt.Println("Installing dependency: ginkgo")
		return sh.RunV(mg.GoCmd(), "install", ginkgoPath)
	}
	return nil
}

func GenTypes() error {
	types, err := ragu.GenerateCode("pkg/types/types.proto", true)
	if err != nil {
		return err
	}
	for _, f := range types {
		err := os.WriteFile(filepath.Join("pkg/types", f.GetName()), []byte(f.GetContent()), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func GenMetrics() error {
	metrics, err := ragu.GenerateCode("pkg/metrics/metrics.proto", false)
	if err != nil {
		return err
	}
	for _, f := range metrics {
		err := os.WriteFile(filepath.Join("pkg/metrics", f.GetName()), []byte(f.GetContent()), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func GenTest() error {
	test, err := ragu.GenerateCode("pkg/test/test.proto", true)
	if err != nil {
		return err
	}
	for _, f := range test {
		err := os.WriteFile(filepath.Join("pkg/test", f.GetName()), []byte(f.GetContent()), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func GenProto() {
	mg.Deps(GenTypes, GenMetrics, GenTest)
}

func ControllerGen() error {
	return sh.RunV("controller-gen",
		`object:headerFile="hack/boilerplate.go.txt"`,
		`crd`,
		`rbac:roleName=manager-role`,
		`paths="./..."`,
		`output:crd:artifacts:config=config/crd/bases`,
	)
}

func MockGen() error {
	if err := sh.RunV("mockgen",
		"-source=pkg/types/types.pb.go",
		"-destination=pkg/test/mock_types/types_mock.go",
	); err != nil {
		return err
	}
	if err := sh.RunV("mockgen",
		"-source=pkg/types/types_grpc.pb.go",
		"-destination=pkg/test/mock_types/types_grpc_mock.go",
	); err != nil {
		return err
	}
	return nil
}

func Docker() error {
	return sh.RunWithV(map[string]string{
		"DOCKER_BUILDKIT": "1",
	},
		"docker", "build",
		"-t", "kubecc/kubecc",
		"-f", "images/kubecc/Dockerfile",
		".",
	)
}

func Test() error {
	mg.Deps(SetupTest)
	return sh.RunV("ginkgo", "-race", "-coverprofile=cover.out", "-covermode=atomic", "./...")
}
