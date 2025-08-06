# Contributing

## Build from Source

> Prerequisites
> 
> * Go 1.20 or higher
> * Git
> * Make (optional, for simplified builds)

1. **Clone the repository:**
   ```bash
   git clone https://github.com/wso2/open-mcp-auth-proxy
   cd open-mcp-auth-proxy
   ```

2. **Install dependencies:**
   ```bash
   go get -v -t -d ./...
   ```

3. **Build the application:**

   **Option A: Using Make**

   ```bash
   # Build for all platforms
   make all
   
   # Or build for specific platforms
   make build-linux      # For Linux (x86_64)
   make build-linux-arm  # For ARM-based Linux
   make build-darwin     # For macOS
   make build-windows    # For Windows
   ```

   **Option B: Manual build (works on all platforms)**

   ```bash
   # Build for your current platform
   go build -o openmcpauthproxy ./cmd/proxy
   
   # Cross-compile for other platforms
   GOOS=linux GOARCH=amd64 go build -o openmcpauthproxy-linux ./cmd/proxy
   GOOS=windows GOARCH=amd64 go build -o openmcpauthproxy.exe ./cmd/proxy
   GOOS=darwin GOARCH=amd64 go build -o openmcpauthproxy-macos ./cmd/proxy
   ```

After building, you'll find the executables in the `build` directory (when using Make) or in your project root (when building manually).

### Additional Make Targets

If you're using Make, these additional targets are available:

```bash
make test       # Run tests
make coverage   # Run tests with coverage report
make fmt        # Format code with gofmt
make vet        # Run go vet
make clean      # Clean build artifacts
make help       # Show all available targets
```