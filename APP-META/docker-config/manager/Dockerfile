# Build the manager binary
FROM --platform=$BUILDPLATFORM reg.docker.alibaba-inc.com/chorus-ci/golang:1.19 as builder
# Copy the go source
ADD ./ /go/src/gitlab.alibaba-inc.com/ak8s-ee/kruise
WORKDIR /go/src/gitlab.alibaba-inc.com/ak8s-ee/kruise
ARG TARGETARCH
# Build
RUN GOARCH=${TARGETARCH} CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -a -o bin/kruise-manager main.go \
    && GOARCH=${TARGETARCH} CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -a -o bin/kruise-daemon ./cmd/daemon/main.go \
    && GOARCH=${TARGETARCH} CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -a -o bin/entrypoint ./cmd/entrypoint/main.go

FROM --platform=$TARGETPLATFORM registry.cn-hangzhou.aliyuncs.com/acs/alpine:3.16-base
USER root
WORKDIR /home/admin/kruise
COPY --from=builder /go/src/gitlab.alibaba-inc.com/ak8s-ee/kruise/bin/kruise-manager /home/admin/kruise/bin/kruise-manager
COPY --from=builder /go/src/gitlab.alibaba-inc.com/ak8s-ee/kruise/bin/kruise-daemon /home/admin/kruise/bin/kruise-daemon
COPY --from=builder /go/src/gitlab.alibaba-inc.com/ak8s-ee/kruise/bin/entrypoint /entrypoint
