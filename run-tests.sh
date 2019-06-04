#!/bin/sh

set -e -x

go get -v github.com/onsi/ginkgo/ginkgo
go mod download

ginkgo -r .
