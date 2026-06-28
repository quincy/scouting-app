build:
	go build -o scout-app .

test:
	go test -v -count=1 -p=1 ./...

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

sec:
	go run github.com/securego/gosec/v2/cmd/gosec@latest -no-fail ./...

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

clean:
	go clean
	rm -f scout-app

ci: clean check test build

devloop-up:
	@echo "Starting dev services..."
	@docker compose up -d cockroachdb
	@echo "Waiting for database to be ready..."
	@until docker compose exec -T cockroachdb cockroach sql --insecure -e "SELECT 1" 2>/dev/null; do \
		sleep 1; \
	done
	@docker compose exec -T cockroachdb cockroach sql --insecure -e "CREATE DATABASE IF NOT EXISTS scoutapp"
	@echo "Dev services ready."

migrate:
	@echo "Running database migrations..."
	@go run ./cmd/migrate/ --env=local.env

devloop-down:
	@echo "Stopping dev services..."
	@docker compose down
	@echo "Dev services stopped."

devloop-reset:
	@echo "Resetting dev database..."
	@docker compose down -v
	@echo "Data volumes removed. Run 'make devloop-up' to start fresh."

run: build devloop-up migrate
	./scout-app --env=local.env

.PHONY: build test fmt vet lint check clean ci devloop-up migrate devloop-down devloop-reset run
