#!/bin/bash

set -e

if [ ! "$(docker ps -q -f name=prometheus)" ]; then
    echo "Starting local Prometheus"
    docker run -it --rm --name prometheus \
        --net host \
        -v $(dirname $(realpath $0)):/config prom/prometheus:latest \
        --config.file /config/prometheus-test-config.yaml \
        --web.enable-lifecycle \
        --web.listen-address=127.0.0.1:9999
    echo "Prometheus listening on port 9999"
else
    echo "Prometheus already running, sending reload request"
    curl -X POST '127.0.0.1:9999/-/reload'
fi
