#!/usr/bin/env bash

set -ex

# basic
export GO111MODULE=on
go version
go env

# build
#make manifests 1>&2
CGO_ENABLED=0 go build -mod=vendor -a -o APP-META/docker-config/pack/bins/kruise-manager main.go
CGO_ENABLED=0 go build -mod=vendor -a -o APP-META/docker-config/pack/bins/kruise-imagepuller ./cmd/imagepuller

# package
wget -P APP-META/docker-config/pack/bins/ http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kubectl/1.14/kubectl
chmod +x APP-META/docker-config/pack/bins/kubectl
cp -r config/crd/bases/apps.kruise.io_statefulsets.yaml APP-META/docker-config/pack/crds/
cp -r config/crd/bases/apps.kruise.io_daemonsets.yaml APP-META/docker-config/pack/crds/
cp -r config/crd/bases/apps.kruise.io_clonesets.yaml APP-META/docker-config/pack/crds/
cp -r config/crd/bases/apps.kruise.io_imagepulljobs.yaml APP-META/docker-config/pack/crds/
cp -r config/crd/bases/apps.kruise.io_nodeimages.yaml APP-META/docker-config/pack/crds/
