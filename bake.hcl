group "default" {
  targets = ["kubecc", "environment"]
}

group "kubecc" {
  targets = ["kubecc-amd64"]#, "kubecc-arm64"]
}

group "environment" {
  targets = ["environment-amd64", "environment-arm64"]
}

target "kubecc-amd64" {
  dockerfile = "images/kubecc/Dockerfile"
  tags = ["kubecc/kubecc"]
  platforms = ["linux/amd64"]
  context = "."
  cache-from = ["kubecc/kubecc:cache-amd64"]
  cache-to = ["kubecc/kubecc:cache-amd64"]
}

target "kubecc-arm64" {
  dockerfile = "images/kubecc/Dockerfile"
  tags = ["kubecc/kubecc"]
  platforms = ["linux/arm64"]
  context = "."
  cache-from = ["kubecc/kubecc:cache-arm64"]
  cache-to = ["kubecc/kubecc:cache-arm64"]
}

target "environment-amd64" {
  dockerfile = "images/environment/Dockerfile"
  tags = ["kubecc/environment"]
  platforms = ["linux/amd64", "linux/arm64"]
  context = "."
  cache-from = ["kubecc/environment:cache-amd64"]
  cache-to = ["kubecc/environment:cache-amd64"]
}

target "environment-arm64" {
  dockerfile = "images/environment/Dockerfile"
  tags = ["kubecc/environment"]
  platforms = ["linux/arm64"]
  context = "."
  cache-from = ["kubecc/environment:cache-arm64"]
  cache-to = ["kubecc/environment:cache-arm64"]
}
