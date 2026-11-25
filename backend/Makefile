APP=hm
VERSION?=1.3.6

.PHONY: build run dev test fmt fmt-check api migrate migrate-up migrate-down

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

migrate:
	go build -trimpath -o bin/hm-migrate ./cmd/migrate

migrate-up: migrate
	./bin/hm-migrate

migrate-down: migrate
	./bin/hm-migrate -down 1

