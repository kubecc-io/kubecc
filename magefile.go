//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kralicky/ragu/pkg/ragu"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"sigs.k8s.io/kustomize/kustomize/v4/commands"
)

var Default = All

var (
	operatorSdkPath   = "github.com/operator-framework/operator-sdk/cmd/operator-sdk@latest"
	controllerGenPath = "https://github.com/kralicky/controller-tools/releases/download/v0.6.2-patched/controller-gen"
	ginkgoPath        = "github.com/onsi/ginkgo/ginkgo@latest"
)

func All() {
	mg.SerialDeps(Setup, Generate, StagingAndVet, Build)
}

func Staging() error {
	kustomize := commands.NewDefaultCommand()
	kustomize.SetArgs([]string{"build", "./config/default", "-o", "./staging/staging_autogen.yaml"})
	return kustomize.Execute()
}

func StagingAndVet() {
	mg.Deps(Staging, Vet)
}

func Generate() {
	mg.Deps(GenProto, ControllerGen)
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
		return sh.Run("chmod", "+x", filepath.Join(gobin, "controller-gen"))
	}
	return nil
}

func SetupDev() error {
	mg.Deps(Setup)
	if _, err := exec.LookPath("operator-sdk"); err != nil {
		fmt.Println("Installing dependency: operator-sdk")
		return sh.RunV(mg.GoCmd(), "install", controllerGenPath)
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

func GenProto() {
	mg.Deps(GenTypes, GenMetrics)
}

func ControllerGen() error {
	return sh.RunV("controller-gen",
		`object:headerFile="hack/boilerplate.go.txt"`,
		`crd:trivialVersions=true,preserveUnknownFields=false`,
		`rbac:roleName=manager-role`,
		`paths="./..."`,
		`output:crd:artifacts:config=config/crd/bases`,
	)
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
