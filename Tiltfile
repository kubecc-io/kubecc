allow_k8s_contexts('default')
allow_k8s_contexts('tilt-kubecc')

k8s_yaml(kustomize("config/default"))

local_resource('Sample YAML', 'kubectl apply -k ./config/samples', 
    deps=["./config/samples"], resource_deps=["kubecc-operator"])

local_resource('Watch & Compile', 
  'mage', 
  deps=['api','controllers','pkg'], 
  ignore=['**/zz_generated.deepcopy.go','**/*.pb.go']
)

dockerfile = '''FROM alpine:latest
WORKDIR /
COPY ./bin/kubecc /
ENTRYPOINT ["/kubecc"]
'''

default_registry("cr.lan.kralicky.dev")
docker_build(
  'kubecc/kubecc', 
  '.', 
  dockerfile_contents=dockerfile,
  only=['./bin/kubecc']
)