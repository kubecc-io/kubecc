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

GO ?= go

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"
CACHE ?= --cache-from type=local,src=/tmp/buildx-cache --cache-to type=local,dest=/tmp/buildx-cache
.PHONY: all setup
all: generate manifests fmt vet bin
setup:
	$(GO) get github.com/operator-framework/operator-sdk/cmd/operator-sdk
	$(GO) get sigs.k8s.io/controller-tools/cmd/controller-gen

# Tests
.PHONY: test-operator test-integration test-e2e test-unit test
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test-operator: generate fmt vet manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); $(GO) test -v ./... -coverprofile cover.out -tags operator

test-integration:
	@KUBECC_LOG_COLOR=1 $(GO) test ./test/integration -tags integration -v -count=1

test-e2e:
	$(GO) build -tags integration -coverprofile cover.out -o bin/test-e2e ./test/e2e
	bin/test-e2e

test-unit:
	$(GO) test ./... -race -coverprofile cover.out

test:
	ginkgo -coverprofile cover.out -race -skipPackage cmd ./...

# Installation and deployment
.PHONY: install uninstall deploy undeploy manifests
install: manifests
	kubectl kustomize config/crd | kubectl apply -f -

uninstall: manifests
	kubectl kustomize config/crd | kubectl delete -f -

deploy: manifests
	kubectl kustomize config/default | kubectl apply -f -

undeploy:
	kubectl kustomize config/default | kubectl delete -f -

manifests: 
	GOROOT=$(shell $(GO) env GOROOT) controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases


# Protobuf code generators
.PHONY: proto
proto:
	protoc proto/types.proto --go_out=. --go-grpc_out=.
	protoc proto/testpb.proto --go_out=. --go-grpc_out=.

# Code generating, formatting, vetting
.PHONY: fmt vet generate
# Run go fmt against code
fmt:
	$(GO) fmt ./...

# Run go vet against code
vet:
	$(GO) vet ./...

# Generate code
generate: 
	GOROOT=$(shell $(GO) env GOROOT) controller-gen object paths="./..."


# Build binaries
.PHONY: bin kubecc kcctl consumer make
bin: kubecc kcctl consumer make

kubecc:
	CGO_ENABLED=0 $(GO) build -o ./build/bin/kubecc ./cmd/kubecc

kcctl:
	CGO_ENABLED=0 $(GO) build -o ./build/bin/kcctl ./cmd/kcctl

consumer:
	CGO_ENABLED=0 $(GO) build -o ./build/bin/consumer ./cmd/consumer

make:
	CGO_ENABLED=0 $(GO) build -o ./build/bin/make ./cmd/make


# Build container images
.PHONY: docker-manager docker-kubecc docker-environment docker
docker-manager:
	docker buildx bake -f bake.hcl manager --push

docker-kubecc:
	docker buildx bake -f bake.hcl kubecc --push

docker-environment:
	docker buildx bake -f bake.hcl environment --push

docker: 
	docker buildx bake -f bake.hcl --push


# Generate bundle manifests and metadata
.PHONY: bundle bundle-build
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	kubectl kustomize config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .
