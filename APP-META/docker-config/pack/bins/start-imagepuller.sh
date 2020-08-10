#!/usr/bin/env bash

LOG_DIR="/home/admin/kruise/log"
mkdir -p $LOG_DIR
chmod 755 $LOG_DIR

if [ -z "$NODE_NAME" ]; then
	export NODE_NAME=$(hostname -i)
fi

if [ ! "$NODE_NAME" ]; then
    echo "failed to set NODE_NAME because of 'hostname -i'=$NODE_NAME"
    exit 1
fi

args="$@"

if ! echo "$args" | grep 'container-runtime-type' &>/dev/null; then
    if [ -z "$RUNTIME_PATH" ]; then
        RUNTIME_PATH="/hostvarrun"
    fi
    if [ -S "$RUNTIME_PATH/pouchd.sock" ]; then
        CONTAINER_RUNTIME="pouch"
        CONTAINER_URI="unix://$RUNTIME_PATH/pouchd.sock"
    elif [ -S "$RUNTIME_PATH/pouchcri.sock" ]; then
        CONTAINER_RUNTIME="containerd"
        CONTAINER_URI="$RUNTIME_PATH/pouchcri.sock"
    elif [ -S "$RUNTIME_PATH/docker.sock" ]; then
        CONTAINER_RUNTIME="docker"
        CONTAINER_URI="unix://$RUNTIME_PATH/docker.sock"
    else
        echo "Can not find container runtime"
        exit 2
    fi
    args="$args -container-runtime-type=$CONTAINER_RUNTIME -container-runtime-uri=$CONTAINER_URI"
fi

if [ -n "$METRICS_PORT" ]; then
	args="$args -metrics-addr=0.0.0.0:$METRICS_PORT"
fi

if [ -z "$LOG_LEVEL" ]; then
	LOG_LEVEL="3"
fi
args="$args -log_dir=$LOG_DIR"

pidof kruise-imagepuller || {
    cd /home/admin/kruise
    ./bin/kruise-imagepuller ${args} 2>&1
}
