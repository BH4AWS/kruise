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

cd /home/admin/kruise
while :; do
  ./bin/kruise-daemon ${args} 2>&1
  sleep 1
done
