#!/bin/bash

set -e -x

go install github.com/onsi/ginkgo/ginkgo@latest

pushd uaa-credentials-broker

go get -v -d ./...

go test -v ./...

popd
