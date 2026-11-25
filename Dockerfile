# 云托管「绑定 GitHub 仓库」构建入口。
# 云托管默认只读仓库根目录的 Dockerfile，而项目后端在 backend/，
# 所以这里显式把构建上下文指向 backend/；本地/CI 仍可用 backend/Dockerfile。
FROM golang:1.25-alpine AS build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG VERSION=1.3.6
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X github.com/bishop9910/hm-lesson/backend/internal/version.Version=${VERSION}" \
    -o /out/hm ./cmd/hm

# 运行镜像必须用 Debian/Ubuntu 系：云托管会注入 /app/cert/initenv.sh
# 做证书/环境变量初始化，该脚本对 Alpine 的 update-ca-certificates 不兼容。
FROM debian:12-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/hm /app/hm
EXPOSE 80
ENV HM_HTTP_ADDR=0.0.0.0:80
ENTRYPOINT ["/app/hm"]
