#!/usr/bin/env bash

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

# update the image tag
GIT_VERSION_SHORT=$(git rev-parse --verify HEAD --short)
ARCH=$(uname -m)
DATE=$(TZ=Asia/Shanghai date '+%Y%m%d%H%M')
version=${CODE_BRANCH}_${ARCH}_${GIT_VERSION_SHORT}_${DATE}
sed -i "s/docker.tag=.*/docker.tag=${version}/" kruise.release
