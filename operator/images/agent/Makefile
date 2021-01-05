all: build

IMG ?= ${KDISTCC_AGENT_IMAGE}

build:
	docker buildx build -t ${IMG} --platform=linux/amd64,linux/arm64 . --push