FROM golang:1.15 as builder

WORKDIR /workspace

COPY . .

RUN go mod download

RUN make scheduler

# Use distroless as minimal base image to package the scheduler binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/build/bin/scheduler .
USER 65532:65532

ENTRYPOINT ["/scheduler"]
