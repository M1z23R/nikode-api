.PHONY: build run test clean dev install deploy tidy lint migrate

# Build configuration
BINARY_NAME=nikode-api
BUILD_DIR=./bin
CMD_PATH=./cmd/nikode-api

# Deployment configuration
INSTALL_DIR=/opt/nikode-api
SERVICE_NAME=nikode-api

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

dev:
	go run $(CMD_PATH)

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
	go run $(CMD_PATH) -migrate

# Install: build, stop service, copy, start service
install: build
	sudo systemctl stop $(SERVICE_NAME)
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	sudo systemctl start $(SERVICE_NAME)
	@echo "Deployed $(BINARY_NAME) to $(INSTALL_DIR) and restarted $(SERVICE_NAME)"

# Full deploy including git pull
deploy:
	git pull
	$(MAKE) install

.DEFAULT_GOAL := build
