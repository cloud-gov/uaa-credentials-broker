#!/bin/sh

set -e -x

export GOPATH=$(pwd)/gopath
export PATH=$PATH:$GOPATH/bin

go get github.com/onsi/ginkgo/ginkgo
go get github.com/onsi/gomega

cd gopath/src/github.com/18F/deployer-account-broker

curl https://glide.sh/get | sh
glide install

ginkgo -r .
