#!/usr/bin/env bash

# download kubebuilder
if [[ $(which kubebuilder) == '' ]]; then
    wget http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kuebuilder/2.3.0/kubebuilder_2.3.0_linux_amd64.tar.gz
    tar -C /tmp -xzf kubebuilder_2.3.0_linux_amd64.tar.gz
    export PATH=$PATH:/tmp/kubebuilder_2.3.0_linux_amd64/bin
    export KUBEBUILDER_ASSETS=/tmp/kubebuilder_2.3.0_linux_amd64/bin
    export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT=300s
fi
