APP=hm
VERSION?=0.1.0-dev

.PHONY: build run dev test fmt fmt-check api migrate-up migrate-down

build:
	go build -trimpath -ldflags "-X github.com/bishop9910/hm-lesson/backend/internal/version.Version=$(VERSION)" -o bin/$(APP) ./cmd/hm

run:
	go run ./cmd/hm

dev:
	go run ./cmd/hm

test:
	go test ./...

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

api:
	buf lint
	buf generate

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down 1
