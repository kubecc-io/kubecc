# IMPORTANT! All GCCs used in the build cluster must be configured
# with the same options, notably PIC/PIE defaults.
# todo: automatically check the compilers for potential issues
FROM ubuntu:20.10
WORKDIR /
RUN apt update \
  && apt -y install build-essential 
USER 65532:65532
