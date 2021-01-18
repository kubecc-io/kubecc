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

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false"
CACHE ?= --cache-from type=local,src=/tmp/buildx-cache --cache-to type=local,dest=/tmp/buildx-cache
all: generate manifests proto fmt vet bin

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./... -coverprofile cover.out

install: manifests
	kubectl kustomize config/crd | kubectl apply -f -

uninstall: manifests
	kubectl kustomize config/crd | kubectl delete -f -

deploy: manifests
	kubectl kustomize config/default | kubectl apply -f -

undeploy:
	kubectl kustomize config/default | kubectl delete -f -

manifests: 
	controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

proto:
	protoc pkg/types/types.proto --go_out=plugins=grpc,paths=source_relative:.

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: 
	controller-gen object paths="./..."

bin: agent scheduler manager make kcctl consumer consumerd

agent:
	CGO_ENABLED=0 go build -o ./build/bin/agent ./cmd/agent

scheduler:
	CGO_ENABLED=0 go build -o ./build/bin/scheduler ./cmd/scheduler

manager:
	CGO_ENABLED=0 go build -o ./build/bin/manager ./cmd/manager

make:
	CGO_ENABLED=0 go build -o ./build/bin/make ./cmd/make

kcctl:
	CGO_ENABLED=0 go build -o ./build/bin/kcctl ./cmd/kcctl

consumer:
	CGO_ENABLED=0 go build -o ./build/bin/consumer ./cmd/consumer

consumerd:
	CGO_ENABLED=0 go build -o ./build/bin/consumerd ./cmd/consumerd

agent-docker:
	docker buildx bake -f bake.hcl agent --push

scheduler-docker:
	docker buildx bake -f bake.hcl scheduler --push

manager-docker:
	docker buildx bake -f bake.hcl manager --push

docker: 
	docker buildx bake -f bake.hcl --push

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	kubectl kustomize config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .
