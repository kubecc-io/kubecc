all: agent mgr operator

agent:
	docker buildx build -t ${KDISTCC_AGENT_IMAGE} --platform=linux/arm64 -f images/agent/Dockerfile . --push

mgr:
	docker buildx build -t ${KDISTCC_MGR_IMAGE} --platform=linux/amd64 -f images/mgr/Dockerfile . --push

operator:
	docker buildx build -t ${KDISTCC_IMAGE} --platform=linux/amd64 -f images/mgr/Dockerfile . --push

.PHONY: all agent mgr operator