# Build the irons CLI application
build:
    @mkdir -p bin
    go build -o bin/irons .

# Run the application directly with go run
run *ARGS:
    go run . {{ ARGS }}

# Clean build artifacts
clean:
    rm -rf bin/

# Install dependencies
deps:
    go mod tidy

# Run tests
test:
    go test ./...

# Default recipe
default: build
