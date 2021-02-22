FROM golang:1.16 as builder

WORKDIR /workspace

COPY . .

RUN go mod download

RUN make agent

# IMPORTANT! All GCCs used in the build cluster must be configured
# with the same options, notably PIC/PIE defaults.
# todo: automatically check the compilers for potential issues
FROM ubuntu:20.10
WORKDIR /
COPY --from=builder /workspace/build/bin/agent .
RUN apt update \
  && apt -y install build-essential 
USER 65532:65532

ENTRYPOINT ["/agent"]
