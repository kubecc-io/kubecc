## Welcome! This project is still in development, but we will be releasing an alpha build soon.

![logo](docs/media/logo.png)

[![Build](https://github.com/kubecc-io/kubecc/actions/workflows/build.yml/badge.svg)](https://github.com/kubecc-io/kubecc/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubecc-io/kubecc)](https://goreportcard.com/report/github.com/kubecc-io/kubecc)
[![Maintainability](https://api.codeclimate.com/v1/badges/ab96b98836f26d980429/maintainability)](https://codeclimate.com/github/kubecc-io/kubecc/maintainability)
[![Gitpod Ready-to-Code](https://img.shields.io/badge/Gitpod-ready--to--code-blue?logo=gitpod)](https://gitpod.io/#https://github.com/kubecc-io/kubecc)

Kubecc is a modern Kubernetes-native distributed build system for teams working on C/C++ projects. Inspired by the original [distcc](https://github.com/distcc/distcc), Kubecc was written from the ground up in Go to be a fast, highly-concurrent build system that lives on your on-prem development cluster and works silently in the background to speed up builds for your entire team. 

---

## Features

- Distribute builds across all machines connected to your cluster without manual setup or per-machine configuration
- Containerized build environments prevent the need to manually install compilers and tools on each machine
- A built-in shared cache enables all developers connected to the cluster to share previously-built object files, with multi-layered caching in memory and optional S3 storage
- Real-time monitoring using the CLI utility, and Prometheus integration to enable custom charts and graphs in Grafana
- Support for mixed-architecture clusters and cross-compiling
- Smart but simple task scheduling using Go's excellent concurrency tools
- Easily runnable outside Kubernetes if needed (requires some setup and configuration)
- OpenTracing integration to view build traces in Jaeger or other supported services

---

Documentation coming soon!
