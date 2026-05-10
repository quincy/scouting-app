# Build the Go application
build:
	go build -o scout-app main.go

# Run tests
verify:
	go test -v ./...

# Run the application
run:
	./scout-app

.PHONY: build test run