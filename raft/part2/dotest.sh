#!/usr/bin/env bash
set -ex

logfile=../temp/rlog

# Check if arguments exist, then construct the command accordingly
if [[ $# -gt 0 ]]; then
    go test -v -race -run "$@" 2>&1 | tee "${logfile}"
else
    go test -v -race 2>&1 | tee "${logfile}"
fi

go run ../tools/raft-testlog-viz/main.go < "${logfile}"
