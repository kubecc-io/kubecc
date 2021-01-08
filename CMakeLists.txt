# kdc-agent CMakeLists.txt

cmake_minimum_required(VERSION 3.16)

project(kdc-agent)

include(go.cmake)

GO_GET(
    github.com/markbates/pkger # add version
)

GO_BUILD(
    OUTPUT kdc-agent
    INSTALL_DEPS
    FLAGS 
)

# mgr CMakeLists.txt

include(docker.cmake)
include(docker.cmake)
include(kustomize.cmake)

DOCKER_BUILD(
    BUILDX
    TAG $ENV{KDISTCC_MGR_IMAGE}
    PLATFORMS linux/amd64
    FILE ./Dockerfile
    PUSH
)

# Operator CMakeLists.txt

include(go.cmake)
include(docker.cmake)
include(kustomize.cmake)
include(operator_sdk.cmake)

GO_GET(
    sigs.k8s.io/kustomize/kustomize/v3@v3.8.7 DEST bin/kustomize
    sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 DEST bin/controller-gen
)

# Current Operator version
VERSION ?= 0.0.1
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Image URL to use all building/pushing image targets
IMG ?= ${KDISTCC_IMAGE}
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

======

GO_BUILD(
    TARGET manager
    OUTPUT bin/manager
    main.go
)

add_dependencies(manager
    generate fmt vet
)

GO_FMT(
    TARGET fmt
    ./...
)

GO_VET(
    TARGET vet
    ./...
)

CONTROLLER_GEN(
    TARGET generate
    OBJECTS
    PATHS ./...
)

GO_RUN(
    TARGET run
    main.go
)

add_dependencies(run
    generate fmt vet manifests
)

KUSTOMIZE(
    TARGET kustomize_install
    BUILD config/crd
    APPLY
) # $(KUSTOMIZE) build config/crd | kubectl apply -f -

add_dependencies(kustomize_install
    manifests
)

KUSTOMIZE(
    TARGET kustomize_uninstall
    BUILD config/crd
    DELETE
) # $(KUSTOMIZE) build config/crd | kubectl delete -f -

add_dependencies(kustomize_uninstall
    manifests
)

# beefed a bit
KUSTOMIZE(
    TARGET kustomize_deploy
    CWD config/manager
    EDIT
    SET IMAGE controller $ENV{KDISTCC_IMAGE}
    BUILD config/default
    APPLY
) #     cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
  #     $(KUSTOMIZE) build config/default | kubectl apply -f -

add_dependencies(kustomize_deploy
    manifests
)

KUSTOMIZE(
    TARGET kustomize_undeploy
    BUILD config/default
    DELETE
)

CRD_OPTIONS ?= crd:trivialVersions=true,preserveUnknownFields=false

MANIFESTS(
    TARGET manifests
	$(CONTROLLER_GEN) 
    $(CRD_OPTIONS) 
    rbac:roleName=manager-role 
    WEBHOOK
    PATHS ./...
    output:crd:artifacts:config=config/crd/bases
) # controller-gen

DOCKER_BUILD(
    TARGET docker
    BUILDX
    TAG $ENV{KDISTCC_MGR_IMAGE}
    PLATFORMS linux/amd64
    FILE ./Dockerfile # FILE /path/to/file, STDIN
    CONTEXT ..
    PUSH
)

add_subdirectory(images/distcc-agent)

OPERATOR_SDK(
    TARGET operator_sdk
    GENERATE kustomize manifests 
    QUIET
)

KUSTOMIZE(
    TARGET bundle
    CWD config/manager
    EDIT
    SET IMAGE controller $ENV{KDISTCC_IMAGE}
    BUILD config/manifests
    #| operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	# operator-sdk bundle validate ./bundle
    # this part seems hard imo
)

.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

add_dependencies(bundle
    operator_sdk manifests kustomize
)

DOCKER_BUILD(
    TARGET bundle-build
    FILE bundle.Dockerfile
    TAG controller-bundle.0.0.1
    CONTEXT .
)

======

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
