build:
	go build -o scout-app .

test:
	go test -v -count=1 ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

check:
	@output=$$(go fmt ./...); \
	[ -z "$$output" ] || (echo "unformatted files:"; echo "$$output"; exit 1)
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

clean:
	go clean
	rm -f scout-app

ci: clean check test build

run: build
	./scout-app --env=local.env

.PHONY: build test vet lint check clean ci run
