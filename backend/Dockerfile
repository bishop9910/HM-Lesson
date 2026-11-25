# 云托管构建：单服务，监听 80
FROM golang:1.25-alpine AS build
# 国内构建走 goproxy.cn；校验仍由 go.sum 保证。
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=1.3.6
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X github.com/bishop9910/hm-lesson/backend/internal/version.Version=${VERSION}" -o /out/hm ./cmd/hm

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
