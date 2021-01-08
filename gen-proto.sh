#!/bin/bash

protoc $(dirname $0)/types/types.proto --go_out=plugins=grpc,paths=source_relative:.
