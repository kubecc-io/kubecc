k8s_yaml(kustomize('config/default'))
local_resource(
  'go-compile', 
  'CGO_ENABLED=0 GOARCH=amd64 go build -o bin/kubecc ./cmd/kubecc',
)
docker_build(
  'kubecc/kubecc', 
  '.', 
  dockerfile='images/tilt/Dockerfile', 
  only=[
    './bin',
  ]
)