.PHONY: build run test clean dev

BINARY_NAME=nikode-api
BUILD_DIR=./bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/nikode-api

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

dev:
	go run ./cmd/nikode-api

test:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)
	go clean

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

migrate:
	go run ./cmd/nikode-api -migrate

.DEFAULT_GOAL := build
