#!/usr/bin/env bash

#### This script is only for mini-asi test.

set -ex

# basic
export GO111MODULE=on
go version
go env

# build
CGO_ENABLED=0 go build -mod=vendor -a -o APP-META/docker-config/pack/bins/kruise-manager main.go
CGO_ENABLED=0 go build -mod=vendor -a -o APP-META/docker-config/pack/bins/kruise-daemon ./cmd/daemon

# package
arch=$(uname -m)
if [[ $arch == x86_64 ]]; then
  wget -P APP-META/docker-config/pack/bins/ http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kubectl/1.18/amd64/kubectl
elif  [[ $arch == aarch64 ]]; then
  wget -P APP-META/docker-config/pack/bins/ http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kubectl/1.18/arm64/kubectl
else
  echo "unsupported arch: ${arch}"
  exit 1
fi
chmod +x APP-META/docker-config/pack/bins/kubectl
