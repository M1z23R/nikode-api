.PHONY: build run test test-unit test-integration test-coverage clean dev install deploy tidy lint migrate setup

# Build configuration
BINARY_NAME=nikode-api
BUILD_DIR=./bin
CMD_PATH=./cmd/nikode-api

# Deployment configuration
INSTALL_DIR=/opt/nikode-api
SERVICE_NAME=nikode-api
SERVICE_USER=nikode

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

dev:
	go run $(CMD_PATH)

test: test-unit test-integration

test-unit:
	go test -v -race -short ./internal/...

test-integration:
	go test -v -race ./tests/integration/...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

clean:
	rm -rf $(BUILD_DIR)
	go clean

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

migrate:
	go run $(CMD_PATH) -migrate

# Create systemd service, user, and install directory
setup: build
	@echo "Creating user $(SERVICE_USER) if not exists..."
	-sudo useradd -r -s /bin/false $(SERVICE_USER) 2>/dev/null || true
	@echo "Creating install directory..."
	sudo mkdir -p $(INSTALL_DIR)
	sudo chown $(SERVICE_USER):$(SERVICE_USER) $(INSTALL_DIR)
	@echo "Installing systemd service..."
	sudo cp $(SERVICE_NAME).service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable $(SERVICE_NAME)
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	sudo chown $(SERVICE_USER):$(SERVICE_USER) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Setup complete. Create $(INSTALL_DIR)/.env then run: sudo systemctl start $(SERVICE_NAME)"

# Install: build, stop service, copy, start service
install: build
	@if ! systemctl is-enabled --quiet $(SERVICE_NAME) 2>/dev/null; then \
		echo "Service not found. Run 'make setup' first."; \
		exit 1; \
	fi
	-sudo systemctl stop $(SERVICE_NAME)
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	sudo chown $(SERVICE_USER):$(SERVICE_USER) $(INSTALL_DIR)/$(BINARY_NAME)
	sudo systemctl start $(SERVICE_NAME)
	@echo "Deployed $(BINARY_NAME) to $(INSTALL_DIR) and restarted $(SERVICE_NAME)"

# Full deploy including git pull
deploy:
	git pull
	$(MAKE) install

.DEFAULT_GOAL := build
