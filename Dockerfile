# 云托管「绑定 GitHub 仓库」构建入口。
# 云托管默认只读仓库根目录的 Dockerfile，而项目后端在 backend/，
# 所以这里显式把构建上下文指向 backend/；本地/CI 仍可用 backend/Dockerfile。
FROM golang:1.25-alpine AS build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
ARG VERSION=0.1.0
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X github.com/bishop9910/hm-lesson/backend/internal/version.Version=${VERSION}" \
    -o /out/hm ./cmd/hm

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/hm /app/hm
EXPOSE 80
ENV HM_HTTP_ADDR=0.0.0.0:80
ENTRYPOINT ["/app/hm"]
