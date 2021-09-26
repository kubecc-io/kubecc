FROM golang:1.17 as builder

WORKDIR /workspace

RUN git clone https://github.com/magefile/mage \
  && cd mage \
  && go run bootstrap.go

COPY . .

RUN go mod download

RUN mage

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
FROM alpine:latest
WORKDIR /
COPY --from=builder /workspace/bin/kubecc .
USER 65532:65532

ENTRYPOINT ["/kubecc"]
