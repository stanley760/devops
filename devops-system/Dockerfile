# devops-system 用户权限系统镜像（多阶段构建：静态编译 + alpine 运行时）。
#
# 构建上下文为仓库根。镜像内以 GOWORK=off 构建：只复制本模块源码，
# 兄弟模块（devops-common / devops-id / devops-infra）由 go.mod 的 require
# 从代理拉取，与外部使用者的真实拉取路径一致，因此这些版本必须已发布。
#   docker build -f devops-system/Dockerfile -t devops-system:dev .
#
# 镜像内配置仅为占位（config/config.yaml），实际环境差异通过
# APP_ENV 合并环境文件覆盖（挂载 devops-system-prod.yaml，见 deploy/）。
# go.mod 要求 go >= 1.27.1，且官方镜像锁定 GOTOOLCHAIN=local，
# 基础镜像必须与 go.work/go.mod 的 go 指令一致，否则 go 命令直接失败。
FROM golang:1.27-alpine AS builder

ARG VERSION=dev
# 默认走国内代理（与宿主机 go env GOPROXY 一致），海外构建可 --build-arg GOPROXY= 覆盖
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
# 关闭工作区模式：镜像内只存在本模块，兄弟模块按 require 版本从代理解析
ENV GOWORK=off

WORKDIR /src

# 依赖描述先于源码拷贝，go.sum 不变时命中层缓存
COPY devops-system/go.mod devops-system/go.sum devops-system/

RUN --mount=type=cache,target=/go/pkg/mod \
    cd devops-system && go mod download

COPY devops-system/ devops-system/

# CGO_ENABLED=0 静态编译；-trimpath 去掉本机路径
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

# 优雅退出由程序自身处理（刷新会话/关连接），务必用 exec 形式传信号
ENTRYPOINT ["devops-system"]
CMD ["server"]
