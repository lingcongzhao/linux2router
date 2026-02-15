.PHONY: build run clean test install systemd

# Build variables
BINARY_NAME=router-gui
BUILD_DIR=./build
CMD_PATH=./cmd/server

# Go build flags
LDFLAGS=-ldflags "-s -w"

# Default target
all: build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for development (with debug info)
build-dev:
	@echo "Building $(BINARY_NAME) (development)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the application (requires root)
run: build
	@echo "Running $(BINARY_NAME)..."
	sudo $(BUILD_DIR)/$(BINARY_NAME)

# Run in development mode
dev:
	@echo "Running in development mode..."
	sudo go run $(CMD_PATH)/main.go

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f router-gui
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed" && exit 1)
	golangci-lint run

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Install to system
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Creating data directories..."
	sudo mkdir -p /var/lib/router-gui/data
	sudo mkdir -p /var/lib/router-gui/configs/iptables
	sudo mkdir -p /var/lib/router-gui/configs/routes
	sudo mkdir -p /var/lib/router-gui/configs/rules
	@echo "Installation complete"

# Generate and install systemd service
systemd: install
	@echo "Installing systemd service..."
	@echo '[Unit]' | sudo tee /etc/systemd/system/router-gui.service
	@echo 'Description=Linux Router GUI' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'After=network.target' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo '' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo '[Service]' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'Type=simple' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'ExecStart=/usr/local/bin/router-gui' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'Restart=always' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'RestartSec=5' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'User=root' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'WorkingDirectory=/var/lib/router-gui' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'Environment=ROUTER_DATA_DIR=/var/lib/router-gui/data' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'Environment=ROUTER_CONFIG_DIR=/var/lib/router-gui/configs' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo '' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo '[Install]' | sudo tee -a /etc/systemd/system/router-gui.service
	@echo 'WantedBy=multi-user.target' | sudo tee -a /etc/systemd/system/router-gui.service
	sudo systemctl daemon-reload
	@echo "Systemd service installed. Enable with: sudo systemctl enable router-gui"
	@echo "Start with: sudo systemctl start router-gui"

# Uninstall from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo systemctl stop router-gui 2>/dev/null || true
	sudo systemctl disable router-gui 2>/dev/null || true
	sudo rm -f /etc/systemd/system/router-gui.service
	sudo systemctl daemon-reload
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstall complete. Data directory preserved at /var/lib/router-gui"

# Show help
help:
	@echo "Linux Router GUI - Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build      Build the application"
	@echo "  build-dev  Build with debug info"
	@echo "  run        Build and run (requires root)"
	@echo "  dev        Run in development mode"
	@echo "  clean      Clean build artifacts"
	@echo "  test       Run tests"
	@echo "  fmt        Format code"
	@echo "  lint       Lint code"
	@echo "  deps       Download dependencies"
	@echo "  install    Install to /usr/local/bin"
	@echo "  systemd    Install systemd service"
	@echo "  uninstall  Uninstall from system"
	@echo "  help       Show this help message"
