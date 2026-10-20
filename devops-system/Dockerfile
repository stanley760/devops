# devops-system user permission system image (multi-stage build: static compilation + alpine runtime).
#
# The build context is the repo root. The image builds with GOWORK=off: only this module's source is copied,
# sibling modules (devops-common / devops-id / devops-infra) are pulled from the proxy per the require in go.mod,
# matching the real pull path for external users, so these versions must already be published.
#   docker build -f devops-system/Dockerfile -t devops-system:dev .
#
# The config inside the image is only a placeholder (config/config.yaml); environment differences are overlaid via
# the APP_ENV env file (mount devops-system-prod.yaml, see deploy/).
# go.mod requires go >= 1.27.1, and the official image pins GOTOOLCHAIN=local,
# so the base image must match the go directive in go.work/go.mod, otherwise go commands fail outright.
FROM golang:1.27-alpine AS builder

ARG VERSION=dev
# Default to the domestic proxy (consistent with the host's go env GOPROXY); overseas builds can override with --build-arg GOPROXY=
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
# Disable workspace mode: only this module exists in the image; sibling modules are resolved from the proxy per their require versions
ENV GOWORK=off

WORKDIR /src

# Dependency descriptors are copied before the source, so the layer cache hits when go.sum is unchanged
COPY devops-system/go.mod devops-system/go.sum devops-system/

RUN --mount=type=cache,target=/go/pkg/mod \
    cd devops-system && go mod download

COPY devops-system/ devops-system/

# CGO_ENABLED=0 static build; -trimpath strips local paths
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd devops-system && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/devops-system .

FROM alpine:3.22

RUN apk add --no-cache tzdata ca-certificates \
    && addgroup -g 10001 -S app && adduser -u 10001 -S -G app app

WORKDIR /app
COPY --from=builder /out/devops-system /usr/local/bin/devops-system
COPY --chown=app:app devops-system/config/ /app/config/
RUN mkdir -p /app/log /app/logs \
    && chown -R app:app /app/log /app/logs

USER app
ENV TZ=Asia/Shanghai \
    APP_ENV=prod
EXPOSE 9999

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:9999/healthz || exit 1

# Graceful shutdown is handled by the program itself (refresh sessions/close connections); always use exec form so signals are passed through
ENTRYPOINT ["devops-system"]
CMD ["server"]
