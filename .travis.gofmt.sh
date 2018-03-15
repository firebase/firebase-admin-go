#!/bin/bash
if [[ ! -z "$(gofmt -l -s .)" ]]; then
    echo "Go code is not formatted:"
    gofmt -d -s .
    exit 1
fi
