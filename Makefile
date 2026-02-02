.PHONY: build install clean test help

# Build variables
BINARY_NAME=git-anticipate
INSTALL_PATH=/usr/local/bin

# Default target
all: build

# Build the binary
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) main.go
	@echo "✅ Build complete! Binary: $(BINARY_NAME)"

# Install the binary to PATH
install: build
	@echo "📦 Installing $(BINARY_NAME) to $(INSTALL_PATH)..."
	sudo mv $(BINARY_NAME) $(INSTALL_PATH)/
	@echo "✅ Installed! Run 'git anticipate --help' to get started"

# Install without sudo (to ~/bin or user-specified path)
install-user: build
	@if [ -d "$$HOME/bin" ]; then \
		echo "📦 Installing $(BINARY_NAME) to $$HOME/bin..."; \
		mv $(BINARY_NAME) $$HOME/bin/; \
		echo "✅ Installed to $$HOME/bin"; \
	elif [ -d "$$HOME/.local/bin" ]; then \
		echo "📦 Installing $(BINARY_NAME) to $$HOME/.local/bin..."; \
		mv $(BINARY_NAME) $$HOME/.local/bin/; \
		echo "✅ Installed to $$HOME/.local/bin"; \
	else \
		echo "❌ No user bin directory found. Please create ~/bin or ~/.local/bin"; \
		exit 1; \
	fi

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf dist/
	go clean
	@echo "✅ Clean complete"

# Download dependencies
deps:
	@echo "📥 Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies ready"

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...
	@echo "✅ Tests complete"

# Run linter
lint:
	@echo "🔍 Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed. Install with:"; \
		echo "    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Format code
fmt:
	@echo "✨ Formatting code..."
	go fmt ./...
	@echo "✅ Format complete"

# Create release builds for multiple platforms
release:
	@echo "📦 Building releases..."
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build -o dist/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build -o dist/$(BINARY_NAME)-darwin-arm64 main.go
	GOOS=linux GOARCH=amd64 go build -o dist/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 go build -o dist/$(BINARY_NAME)-linux-arm64 main.go
	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY_NAME)-windows-amd64.exe main.go
	@echo "✅ Release builds complete in dist/"

# Uninstall the binary
uninstall:
	@echo "🗑️  Uninstalling $(BINARY_NAME)..."
	@if [ -f "$(INSTALL_PATH)/$(BINARY_NAME)" ]; then \
		sudo rm $(INSTALL_PATH)/$(BINARY_NAME); \
		echo "✅ Uninstalled from $(INSTALL_PATH)"; \
	elif [ -f "$$HOME/bin/$(BINARY_NAME)" ]; then \
		rm $$HOME/bin/$(BINARY_NAME); \
		echo "✅ Uninstalled from $$HOME/bin"; \
	elif [ -f "$$HOME/.local/bin/$(BINARY_NAME)" ]; then \
		rm $$HOME/.local/bin/$(BINARY_NAME); \
		echo "✅ Uninstalled from $$HOME/.local/bin"; \
	else \
		echo "⚠️  $(BINARY_NAME) not found in PATH"; \
	fi

# Run the binary locally (without installing)
run:
	@go run main.go $(ARGS)

# Show help
help:
	@echo "git-anticipate Makefile commands:"
	@echo ""
	@echo "  make build         - Build the binary"
	@echo "  make install       - Build and install to /usr/local/bin (requires sudo)"
	@echo "  make install-user  - Build and install to ~/bin or ~/.local/bin"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make deps          - Download and tidy dependencies"
	@echo "  make test          - Run tests"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
	@echo "  make release       - Build for multiple platforms"
	@echo "  make uninstall     - Remove installed binary"
	@echo "  make run ARGS=...  - Run without installing (e.g., make run ARGS='--help')"
	@echo "  make help          - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make install-user"
	@echo "  make run ARGS='dev --help'"
