group "default" {
  targets = ["manager", "kubecc", "environment"]
}

target "manager" {
  dockerfile = "images/manager/Dockerfile"
  tags = ["gcr.io/kubecc/manager"]
  platforms = ["linux/amd64"]
  context = "."
  cache-from = ["type=local,src=build/cache/manager"]
  cache-to = ["type=local,dest=build/cache/manager"]
}

target "kubecc" {
  dockerfile = "images/kubecc/Dockerfile"
  tags = ["gcr.io/kubecc/kubecc"]
  platforms = ["linux/amd64"]
  context = "."
  cache-from = ["type=local,src=build/cache/kubecc"]
  cache-to = ["type=local,dest=build/cache/kubecc"]
}

target "kubecc" {
  dockerfile = "images/environment/Dockerfile"
  tags = ["gcr.io/kubecc/environment"]
  platforms = ["linux/amd64"]
  context = "."
  cache-from = ["type=local,src=build/cache/environment"]
  cache-to = ["type=local,dest=build/cache/environment"]
}
