# Build the manager binary
FROM golang:1.15 as builder

WORKDIR /workspace

# Copy the go source
COPY . .

# Build
RUN cd cmd/agent \
 && CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build 

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/cmd/agent/agent .
USER 65532:65532

ENTRYPOINT ["/agent"]
