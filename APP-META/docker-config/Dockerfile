# Build the manager binary
FROM --platform=$BUILDPLATFORM openkruise-registry.cn-shanghai.cr.aliyuncs.com/openkruise/golang:1.19-alpine3.17 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY vendor/ vendor/
COPY APP-META/ APP-META/

ARG TARGETARCH
RUN if [[ $TARGETARCH == amd64 ]]; then \
      echo "install amd64 kubectl"; \
      wget -P APP-META/docker-config/pack/bins/ http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kubectl/1.18/amd64/kubectl; \
    elif  [[ $TARGETARCH == arm64 ]]; then \
      echo "install arm64 kubectl"; \
      wget -P APP-META/docker-config/pack/bins/ http://iops.oss-cn-hangzhou-zmf.aliyuncs.com/kubectl/1.18/arm64/kubectl; \
    else \
      echo "unsupported arch: ${arch}" && exit 1; \
    fi
RUN chmod +x APP-META/docker-config/pack/bins/kubectl
# Build
RUN GOARCH=${TARGETARCH} CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -a -o APP-META/docker-config/pack/bins/kruise-manager main.go \
    && GOARCH=${TARGETARCH} CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -a -o APP-META/docker-config/pack/bins/kruise-daemon ./cmd/daemon/main.go \
    && GOARCH=${TARGETARCH} CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -a -o APP-META/docker-config/pack/bins/entrypoint ./cmd/entrypoint/main.go

FROM --platform=$TARGETPLATFORM openkruise-registry.cn-shanghai.cr.aliyuncs.com/openkruise/centos:20220802

WORKDIR /home/admin/kruise

COPY --from=builder /workspace/APP-META/docker-config/pack/hack/logrotate /etc/cron.hourly/
COPY --from=builder /workspace/APP-META/docker-config/pack/hack/logrotate.conf /etc/logrotate.d/kruise
COPY --from=builder /workspace/APP-META/docker-config/pack/bins/ /home/admin/kruise/bin
COPY --from=builder /workspace/APP-META/docker-config/pack/crds /home/admin/kruise/crds
COPY --from=builder /workspace/APP-META/docker-config/pack/bins/entrypoint /entrypoint
