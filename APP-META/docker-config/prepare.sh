#!/usr/bin/env bash

# download kubebuilder
mkdir -p /tmp/kubebuilder
wget http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kuebuilder-tools/kubebuilder-tools-1.19.2-linux-amd64.tar.gz
tar -C /tmp/kubebuilder --strip-components=1 -zvxf kubebuilder-tools-1.19.2-linux-amd64.tar.gz
export PATH=/tmp/kubebuilder/bin:$PATH
export KUBEBUILDER_ASSETS=/tmp/kubebuilder/bin
export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT=300s
