# devops-id ID generation service image (multi-stage build: static compile + alpine runtime).
#
# The build context is the repository root. The image builds with GOWORK=off: only this module's source is copied,
# and sibling modules (devops-infra -> devops-common) are pulled from the proxy via go.mod requires,
# matching the real resolution path of external users, so those versions must already be published.
#   docker build -f devops-id/Dockerfile -t devops-id:dev .
#
# The in-image config is only a placeholder (config/config.yaml); real environment differences are applied
# by merging environment files via APP_ENV (mounting devops-id-prod.yaml, see deploy/).
FROM golang:1.27-alpine AS builder

ARG VERSION=dev
# Uses a domestic proxy by default (matching the host's go env GOPROXY); overseas builds can override with --build-arg GOPROXY=
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
# Disable workspace mode: only this module exists in the image; sibling modules resolve from the proxy at their require versions
ENV GOWORK=off

WORKDIR /src

# Copy dependency descriptors before source so the layer cache hits when go.sum is unchanged
COPY devops-id/go.mod devops-id/go.sum devops-id/

RUN --mount=type=cache,target=/go/pkg/mod \
    cd devops-id && go mod download

COPY devops-id/ devops-id/

# CGO_ENABLED=0 for a static build; -trimpath removes local paths
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd devops-id && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/devops-id .

FROM alpine:3.22

RUN apk add --no-cache tzdata ca-certificates \
    && addgroup -g 10001 -S app && adduser -u 10001 -S -G app app

WORKDIR /app
COPY --from=builder /out/devops-id /usr/local/bin/devops-id
COPY --chown=app:app devops-id/config/ /app/config/
RUN mkdir -p /app/log /app/logs /app/data \
    && chown -R app:app /app/log /app/logs /app/data

USER app
ENV TZ=Asia/Shanghai \
    APP_ENV=prod
EXPOSE 9990

# gRPC has no HTTP endpoint, so the container-level health check degrades to a TCP probe;
# on K8s use the native grpc probe (see deploy/k8s/devops.yaml)
HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD nc -z 127.0.0.1 9990 || exit 1

# Graceful shutdown is handled by the program itself (revoking the lease releases the slot immediately); always use exec form so signals are delivered
ENTRYPOINT ["devops-id"]
CMD ["server"]
