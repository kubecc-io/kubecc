group "default" {
  targets = ["kubecc", "environment"]
}

target "kubecc" {
  dockerfile = "images/kubecc/Dockerfile"
  tags = ["gcr.io/kubecc/kubecc"]
  platforms = ["linux/amd64", "linux/arm64"]
  context = "."
  cache-from = ["type=local,src=build/cache/kubecc"]
  cache-to = ["type=local,dest=build/cache/kubecc"]
}

target "environment" {
  dockerfile = "images/environment/Dockerfile"
  tags = ["gcr.io/kubecc/environment"]
  platforms = ["linux/amd64", "linux/arm64"]
  context = "."
  cache-from = ["type=local,src=build/cache/environment"]
  cache-to = ["type=local,dest=build/cache/environment"]
}
