#!/bin/bash

protoc $(dirname $0)/api/api.proto --go_out=plugins=grpc:$(dirname $0)
