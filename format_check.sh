#!/bin/bash -e

format_diff=$(gofmt -l -s .)
if [ -n "$format_diff" ]; then
    echo "Formatting errors in:"
    echo "$format_diff"
    exit 1
fi
