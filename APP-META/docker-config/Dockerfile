# Build the manager binary
FROM --platform=$BUILDPLATFORM openkruise-registry.cn-shanghai.cr.aliyuncs.com/openkruise/golang:1.17 as builder

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

# Build
ARG TARGETARCH
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
