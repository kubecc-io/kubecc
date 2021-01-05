# --allow-private

# distcc daemon

# gcc/g++ tools

# eventually run daemon as sidecar container, running alongside custom build env

FROM gcc:10.2 as builder

WORKDIR /tmp

RUN \
    mkdir -p /usr/sbin && \
    ln -s /usr/bin/dpkg-split /usr/sbin/dpkg-split && \
    ln -s /usr/bin/dpkg-deb /usr/sbin/dpkg-deb && \
    ln -s /usr/bin/tar /usr/sbin/tar && \
    echo $PATH && ls -al /usr/sbin && \
  # export DEBIAN_FRONTEND=noninteractive && \
    apt-get update && \

    # DEBIAN_FRONTEND=noninteractive apt-get install --reinstall -y dpkg && \
  # apt install -y python3 && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y python3-dev && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y libiberty-dev && \
    curl -sLO https://github.com/distcc/distcc/archive/v3.3.5.tar.gz && \
    tar -zxvf v3.3.5.tar.gz && \
    cd distcc-3.3.5 && \
    ./autogen.sh && \
    ./configure && \
    make

FROM gcc:10.2

WORKDIR /

COPY --from=builder /tmp/distcc-3.3.5/distccd /usr/bin
COPY --from=builder /tmp/distcc-3.3.5/distcc /usr/bin

RUN echo '#!/bin/sh \n\
distccd --no-detach --daemon --allow 10.0.0.0/8 --enable-tcp-insecure --verbose --user distcc "$@"' > /entrypoint.sh && chmod +x /entrypoint.sh \
 && useradd distcc

ENTRYPOINT [ "/entrypoint.sh" ]