.PHONY: build run clean all test test-coverage lint deps

all: deps build

build:
	go build -o sitemap_checker main.go

run: build
	./sitemap_checker

clean:
	rm -f sitemap_checker
	rm -f coverage.out coverage.html

deps:
	go mod download
	go mod tidy

lint:
	go vet ./...
	go fmt ./...

test:
	@echo "Running tests..."
	go test -v ./...
	@echo -e "\nGenerating coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo -e "\nGenerating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report saved to coverage.html"
	@echo -e "\nTest completed."

test-coverage: test