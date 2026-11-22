# 云托管构建：单服务，监听 80
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X github.com/bishop9910/hm-lesson/backend/internal/version.Version=0.1.0" -o /out/hm ./cmd/hm

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/hm /app/hm
EXPOSE 80
ENV HM_HTTP_ADDR=0.0.0.0:80
USER nonroot:nonroot
ENTRYPOINT ["/app/hm"]
