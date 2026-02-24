.PHONY: all clean build test

all: build

# Build all SDK examples
build:
	cd simple && go build -o simple-render .
	cd non-writable-fs && go build -o serverless-render .

# Clean built binaries
clean:
	rm -f simple/simple-render
	rm -f non-writable-fs/serverless-render

# Run go mod tidy on all examples
tidy:
	cd simple && go mod tidy
	cd non-writable-fs && go mod tidy

# Test that examples compile (no runtime tests yet)
test: build
	@echo "SDK examples built successfully"
