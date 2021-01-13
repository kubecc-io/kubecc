# Build the manager binary
FROM golang:1.15 as builder

WORKDIR /workspace

# Copy the go source
COPY . .

# Build
RUN cd cmd/agent \
 && CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build 

# IMPORTANT! All GCCs used in the build cluster must be configured
# with the same options, notably PIC/PIE defaults.
# todo: automatically check the compilers for potential issues
FROM ubuntu:20.10
WORKDIR /
COPY --from=builder /workspace/cmd/agent/agent .
RUN apt update \
 && apt -y install build-essential 

USER 65532:65532

ENTRYPOINT ["/agent"]
