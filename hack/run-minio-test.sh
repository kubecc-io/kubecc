#!/bin/bash

set -e

docker run -it --rm --name minio \
  -p 9998:9000 \
  minio/minio server /data
