# get all packages need to be tested
function GetKubePkgs() {
    pkgs=""
    cd $GOPATH/src/github.com/openkruise/kruise
    for i in $1 ; do
        for line in $(go list $i/...); do
                pkgs=$pkgs" $line "
        done
    done
    echo "$pkgs"
}

PKG=$*
if [[ $PKG == "" ]]; then
	echo "no test dir is specified, skip ut!"
	exit 0
fi

echo "will run ut for dirs: $PKG"
curdir=$(cd $(dirname $0); pwd)
pkgs=$(GetKubePkgs $PKG)
if [[ $pkgs == "" ]]; then
	echo "pkg is empty, skip"
	exit 0
fi
cd $GOPATH/src/gitlab.alibaba-inc.com/ak8s-ee/kruise

wget http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kuebuilder/1.0.8/kubebuilder_1.0.8_linux_amd64.tar.gz
tar -C /usr/local -xzf go1.13.3.linux-amd64.tar.gz
tar -C /tmp -xzf kubebuilder_1.0.8_linux_amd64.tar.gz
mv /tmp/kubebuilder_1.0.8_linux_amd64 /usr/local/kubebuilder

#time go test -v $pkgs
time go test -v -coverprofile=$CHORUS_UPLOAD_DIR/$CHORUS_TASK_NAME.partialCoverage $pkgs >/tmp/full.log 2>&1
ret=$?
cat /tmp/full.log

# 简化ut 日志的处理脚本
cat /tmp/full.log| python $curdir/simplify_ut_log.py >$CHORUS_UPLOAD_DIR/$CHORUS_TASK_NAME.simplifyUtLog.report

echo "ut return: $ret"
# 为了适配 tekton，总是返回 0，通过 grep fail/panic 来标识测试是否失败
exit 0
