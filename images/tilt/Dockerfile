FROM golang:1.16 as builder

WORKDIR /workspace

COPY . .

#FROM gcr.io/distroless/static:nonroot
FROM busybox:latest
WORKDIR /
COPY --from=builder /workspace/bin/kubecc .
USER 65532:65532

ENTRYPOINT ["/kubecc"]
