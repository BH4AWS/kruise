# Build the manager binary
FROM --platform=$TARGETPLATFORM openkruise-registry.cn-shanghai.cr.aliyuncs.com/openkruise/centos:20220802
# Copy the go source
ADD ./APP-META/docker-config/job/ /home/admin/kruise
WORKDIR /home/admin/kruise

RUN yum update -y \
    && yum -y install wget

ARG TARGETARCH
RUN if [[ $TARGETARCH == amd64 ]]; then \
      echo "install amd64 kubectl"; \
      wget http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kubectl/1.18/amd64/kubectl; \
    elif  [[ $TARGETARCH == arm64 ]]; then \
      echo "install arm64 kubectl"; \
      wget http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kubectl/1.18/arm64/kubectl; \
    else \
      echo "unsupported arch: ${arch}" && exit 1; \
    fi
RUN chmod +x kubectl
RUN chmod +x start-job.sh


