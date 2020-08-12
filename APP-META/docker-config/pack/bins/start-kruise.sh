#!/usr/bin/env bash

# 启动crond用于logrotate
pidof crond || {
crond -m/dev/null &
}
mkdir -p /home/admin/logs/ /home/admin/kruise/log/
chmod 755 /home/admin/kruise/log/
chmod 644 /etc/logrotate.d/kruise

#if [ -z "$WEBHOOK_HOST" ]; then
#	export WEBHOOK_HOST=$(hostname -i)
#fi

#if [ ! "$WEBHOOK_HOST" ];then
#    echo "failed to set WEBHOOK_HOST because of 'hostname -i'=$WEBHOOK_HOST"
#    exit 1
#fi

args="-rest-config-burst=500 -rest-config-qps=300 -logtostderr -enable-leader-election -leader-election-namespace=kube-system"

if [ ! -z "$METRICS_PORT" ]; then
	args="$args -metrics-addr=0.0.0.0:$METRICS_PORT"
fi

if [ ! -z "$HEALTH_PORT" ]; then
	args="$args -health-probe-addr=0.0.0.0:$HEALTH_PORT"
fi

if [ -z "$LOG_LEVEL" ]; then
	LOG_LEVEL="5"
fi
args="$args -v=$LOG_LEVEL"

if [ -z "$CLONESET_WORKERS" ]; then
	CLONESET_WORKERS="20"
fi
args="$args -cloneset-workers=$CLONESET_WORKERS"

pidof kruise-manager || {
    cd /home/admin/kruise
    ./bin/kubectl apply -f crds/
    ./bin/kruise-manager ${args} 2>&1 | tee -a log/manager.log
}