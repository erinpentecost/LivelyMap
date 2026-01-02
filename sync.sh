#!/usr/bin/env bash

pushd "$(dirname "$0")" || exit
pwd

# build the tool if it's not already built
if [ ! -f "./cmd/lively" ]; then
pushd ./cmd/lively
go build . || exit
popd
fi

# run it
./cmd/lively/lively -threads=3 -vanity=F -ramp="light" -cfg="$1"

popd
