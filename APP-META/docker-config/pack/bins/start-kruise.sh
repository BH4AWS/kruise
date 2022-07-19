#!/usr/bin/env bash

mkdir -p /home/admin/logs/ /home/admin/kruise/log/
chmod 755 /home/admin/kruise/log/
chmod 644 /etc/logrotate.d/kruise

if [ ! -z "$LOG_ROTATE_LIMIT" ]; then
  sed -i 's/25/'"${LOG_ROTATE_LIMIT}"'/g' /etc/logrotate.d/kruise
fi

# 启动crond用于logrotate
pidof crond || {
crond -m/dev/null &
}

#if [ -z "$WEBHOOK_HOST" ]; then
#	export WEBHOOK_HOST=$(hostname -i)
#fi

#if [ ! "$WEBHOOK_HOST" ];then
#    echo "failed to set WEBHOOK_HOST because of 'hostname -i'=$WEBHOOK_HOST"
#    exit 1
#fi

args="--rest-config-burst=500 --rest-config-qps=300 --logtostderr --enable-leader-election --leader-election-namespace=kube-system --enable-pprof"

if [ ! -z "$METRICS_PORT" ]; then
	args="$args --metrics-addr=0.0.0.0:$METRICS_PORT"
fi

if [ ! -z "$HEALTH_PORT" ]; then
	args="$args --health-probe-addr=0.0.0.0:$HEALTH_PORT"
fi

if [ ! -z "$PPROF_PORT" ]; then
	args="$args --pprof-addr=0.0.0.0:$PPROF_PORT"
fi

if [ -z "$LOG_LEVEL" ]; then
	LOG_LEVEL="5"
fi
args="$args --v=$LOG_LEVEL"

if [ -z "$RATE_LIMITER_MAX_DELAY" ]; then
  RATE_LIMITER_MAX_DELAY="10m"
fi
args="$args --rate-limiter-max-delay=$RATE_LIMITER_MAX_DELAY"

if [ -z "$CLONESET_WORKERS" ]; then
	CLONESET_WORKERS="20"
fi
args="$args --cloneset-workers=$CLONESET_WORKERS"

if [ ! -z "$FEATURE_GATES" ]; then
	args="$args --feature-gates=$FEATURE_GATES"
fi

if [ -z "$RESYNC_PERIOD" ]; then
	RESYNC_PERIOD="0"
fi
args="$args --sync-period=$RESYNC_PERIOD"

if [ -z "$PUB_WORKERS" ]; then
	PUB_WORKERS="20"
fi
args="$args --podunavailablebudget-workers=$PUB_WORKERS"

if [ -z "$PUB_MODE" ]; then
	PUB_MODE="asi"
fi
args="$args --pub-mode=$PUB_MODE"

if [ -z "$LIFECYCLE_READINESS_MODE" ]; then
	LIFECYCLE_READINESS_MODE="asi"
fi
args="$args --lifecycle-readiness-mode=$LIFECYCLE_READINESS_MODE"

if [ -z "$CLONESET_SCALING_EXCLUDE_PREPARING_DELETE" ]; then
	CLONESET_SCALING_EXCLUDE_PREPARING_DELETE="true"
fi
args="$args --cloneset-scaling-exclude-preparing-delete=$CLONESET_SCALING_EXCLUDE_PREPARING_DELETE"

pidof kruise-manager || {
  cd /home/admin/kruise
  for FILE in crds/*.yaml; do
    ./bin/kubectl replace -f $FILE || ./bin/kubectl create -f $FILE
  done
  ./bin/kruise-manager ${args} 2>&1 | tee -a log/manager.log
}
