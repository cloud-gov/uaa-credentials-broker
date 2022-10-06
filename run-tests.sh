#!/bin/bash

set -e -x

go install github.com/onsi/ginkgo/ginkgo

pushd uaa-credentials-broker

go get -v -d ./...

ginkgo -r .

popd
