#!/bin/bash

here="$(realpath $(dirname $0))"
for f in $(find "${here}/../" -name "*.go" | grep -v 'build/'); do
    if grep -q "Copyright 2021" "$f" || grep -q "DO NOT EDIT" "$f"; then
        continue
    fi
    echo "$(realpath $f)"
    cat ${here}/boilerplate.go.txt $f > $f.tmp && mv $f.tmp $f
done
