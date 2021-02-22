group "default" {
  targets = ["agent", "manager", "scheduler"]
}

target "agent" {
  dockerfile = "images/agent/Dockerfile"
  tags = ["gcr.io/kubecc/agent"]
  platforms = ["linux/amd64"]
  context = "."
  cache-from = ["type=local,src=build/cache/agent"]
  cache-to = ["type=local,dest=build/cache/agent"]
}

target "manager" {
  dockerfile = "images/manager/Dockerfile"
  tags = ["gcr.io/kubecc/manager"]
  platforms = ["linux/amd64"]
  context = "."
  cache-from = ["type=local,src=build/cache/manager"]
  cache-to = ["type=local,dest=build/cache/manager"]
}

target "scheduler" {
  dockerfile = "images/scheduler/Dockerfile"
  tags = ["gcr.io/kubecc/scheduler"]
  platforms = ["linux/amd64"]
  context = "."
  cache-from = ["type=local,src=build/cache/scheduler"]
  cache-to = ["type=local,dest=build/cache/scheduler"]
}

target "monitor" {
  dockerfile = "images/monitor/Dockerfile"
  tags = ["gcr.io/kubecc/monitor"]
  platforms = ["linux/amd64"]
  context = "."
  cache-from = ["type=local,src=build/cache/monitor"]
  cache-to = ["type=local,dest=build/cache/monitor"]
}
