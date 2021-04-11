#!/bin/sh

cd "$(realpath $(dirname $0)/..)"

kubectl kustomize ./config/default > ./staging/staging_autogen.yaml