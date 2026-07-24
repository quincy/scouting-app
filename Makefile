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

cover:
	go test -coverprofile=coverage.out -count=1 ./...
	go tool cover -html=coverage.out
	rm -f coverage.out

cover-ci:
	@echo "Checking coverage on changed files..."
	@base=$$(git rev-parse origin/main 2>/dev/null || git rev-parse main 2>/dev/null || git rev-list --max-parents=0 HEAD); \
	diffcommitted=$$(git diff --name-only "$$base"...HEAD -- '*.go' 2>/dev/null || true); \
	diffunstaged=$$(git diff HEAD --name-only -- '*.go' 2>/dev/null || true); \
	diffstaged=$$(git diff --cached --name-only -- '*.go' 2>/dev/null || true); \
	changed=$$(printf '%s\n' "$$diffcommitted" "$$diffunstaged" "$$diffstaged" | grep -v '^$$' | sort -u | grep -v '_test\.go$$' | grep -v '^cmd/' | grep -v '^main\.go$$' || true); \
	if [ -z "$$changed" ]; then \
		echo "  No changed production Go files to check."; \
		exit 0; \
	fi; \
	fail=0; \
	for f in $$changed; do \
		case "$$f" in internal/storage/mock/*|internal/testhelper/*) continue ;; esac; \
		if ! grep -q '^func ' "$$f" 2>/dev/null; then \
			echo "  SKIP  $$f  (no executable functions)"; \
			continue; \
		fi; \
		dir=$$(dirname $$f); \
		pkg=./$$dir; \
		coverfile=/tmp/cover-$$(echo $$dir | tr '/' '-').out; \
		go test -coverprofile=$$coverfile "$$pkg" 2>/dev/null >/dev/null || true; \
		if [ -f "$$coverfile" ]; then \
			newfuncs=$$( (git diff "$$base"...HEAD -- "$$f" 2>/dev/null; git diff HEAD -- "$$f" 2>/dev/null; git diff --cached -- "$$f" 2>/dev/null) | grep '+func ' | sed 's/.*+func //' | sed 's/^([^)]*) //' | sed 's/(.*//' | sort -u); \
			if [ -z "$$newfuncs" ]; then \
				echo "  OK    $$f  (no new functions)"; \
			else \
				echo "  $$f:"; \
				anyfail=0; \
				for funcname in $$newfuncs; do \
					cov=$$(go tool cover -func=$$coverfile | awk -v fp="^scout-app/$$f:" -v fn="$$funcname" '$$0 ~ fp && $$2 == fn {print $$NF}' | sed 's/%//'); \
					if [ -z "$$cov" ]; then \
						echo "    ???   $$funcname  (not found in coverage)"; \
						continue; \
					fi; \
					if [ "$$(echo "$$cov < 80" | bc 2>/dev/null)" -eq 1 ]; then \
						echo "    FAIL  $$funcname  ($$cov% < 80%)"; \
						fail=1; anyfail=1; \
					else \
						echo "    OK    $$funcname  ($$cov%)"; \
					fi; \
				done; \
				if [ "$$anyfail" -eq 0 ]; then \
					echo "    OK    (all new functions >= 80%)"; \
				fi; \
			fi; \
			rm -f $$coverfile; \
		fi; \
	done; \
	if [ "$$fail" -eq 1 ]; then \
		echo ""; \
		echo "Some new functions have less than 80% coverage. Write tests before committing."; \
		exit 1; \
	fi; \
	echo "  All new functions have adequate coverage."

clean:
	go clean
	rm -f scout-app

ci: clean check test cover-ci build

devloop-up:
	@echo "Starting dev services..."
	@docker compose up -d
	@echo "Waiting for database to be ready..."
	@until docker compose exec -T cockroachdb cockroach sql --insecure -e "SELECT 1" 2>/dev/null; do \
		sleep 1; \
	done
	@docker compose exec -T cockroachdb cockroach sql --insecure -e "CREATE DATABASE IF NOT EXISTS scoutapp"
	@echo "Dev services ready."
	@echo "MailHog UI: http://localhost:8025"

migrate:
	@echo "Running database migrations..."
	@go run ./cmd/migrate/ --env=local.env

seed-inactive:
	@echo "Creating inactive test profiles..."
	@go run ./cmd/seed-inactive/ --env=local.env

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

.PHONY: build test fmt vet lint check cover cover-ci clean ci devloop-up migrate seed-inactive devloop-down devloop-reset run
