group "default" {
  targets = ["agent", "manager", "scheduler"]
}

target "agent" {
  dockerfile = "images/agent/Dockerfile"
  tags = ["gcr.io/kubecc/agent"]
  platforms = ["linux/arm64","linux/amd64"]
  context = "."
}

target "manager" {
  dockerfile = "images/manager/Dockerfile"
  tags = ["gcr.io/kubecc/manager"]
  platforms = ["linux/amd64"]
  context = "."
}

target "scheduler" {
  dockerfile = "images/scheduler/Dockerfile"
  tags = ["gcr.io/kubecc/scheduler"]
  platforms = ["linux/amd64"]
  context = "."
}
