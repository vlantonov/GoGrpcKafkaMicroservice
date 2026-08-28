.PHONY: proto build test lint docker-up docker-down help

## proto: Generate Go code from proto/orders/v1/orders.proto
proto:
	protoc \
	  --proto_path=proto \
	  --go_out=gen --go_opt=paths=source_relative \
	  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
	  orders/v1/orders.proto

## build: Compile the server binary to bin/server
build:
	go build -o bin/server ./cmd/server

## test: Run all tests with the race detector and coverage
test:
	go test -race -coverprofile=coverage.out ./...

## lint: Run golangci-lint
lint:
	golangci-lint run

## docker-up: Build image and start all services with Docker Compose
docker-up:
	docker compose up --build -d

## docker-down: Stop and remove all Docker Compose services
docker-down:
	docker compose down

## help: Show this help message
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //'
