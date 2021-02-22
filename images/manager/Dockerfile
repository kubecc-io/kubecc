FROM golang:1.16 as builder

WORKDIR /workspace

COPY . .

RUN go mod download

RUN make manager

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/build/bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
