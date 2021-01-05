# Current Operator version
VERSION ?= 0.0.1

# Image URL to use all building/pushing image targets
IMG ?= ${KDISTCC_MGR_IMAGE}

all: docker

# Build the docker image
docker:
	#docker buildx build -t ${IMG} --platform=linux/arm64,linux/amd64 -f ./Dockerfile .. --push
	docker buildx build -t ${IMG} --platform=linux/amd64 -f ./Dockerfile .. --push

