# 云托管构建：单服务，监听 80
FROM golang:1.25-alpine AS build
# 国内构建走 goproxy.cn；校验仍由 go.sum 保证。
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=0.1.0
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X github.com/bishop9910/hm-lesson/backend/internal/version.Version=${VERSION}" -o /out/hm ./cmd/hm

# 运行镜像用 Alpine（Docker Hub 可达性优于 gcr.io；云托管构建更容易拉取）。
# MVP 以 root 运行以便直接监听 80；后续可改 8080 + 非 root 用户。
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/hm /app/hm
EXPOSE 80
ENV HM_HTTP_ADDR=0.0.0.0:80
ENTRYPOINT ["/app/hm"]
