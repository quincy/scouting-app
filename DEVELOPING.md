# Developing

## Prerequisites

- [Go](https://go.dev/dl/) 1.25+ (match `go.mod`)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (for local CockroachDB)
- [Flyctl](https://fly.io/docs/flyctl/install/) (for deployment)
- [Goose](https://github.com/pressly/goose) (database migrations, installed via `go.mod`)

## Setup

1. Clone the repository:
   ```bash
   git clone git@github.com:quincy/scouting-app.git
   cd scout-app
   ```

2. Start the development database:
   ```bash
   make devloop-up
   ```

3. Run database migrations:
   ```bash
   make migrate
   ```

## Building

Compile the binary:

```bash
make build
```

This produces a `scout-app` binary in the project root.

## Testing

Run all tests (with race detection, sequential to avoid DB conflicts):

```bash
make test
```

Run tests with coverage:

```bash
go test -v -count=1 -p=1 -coverprofile=coverage.txt ./...
```

## Running Locally

Start the development database and run the application:

```bash
make run
```

This will:
1. Build the binary.
2. Start CockroachDB via Docker.
3. Run database migrations.
4. Start the web server on `http://localhost:8080`.

On the first run, you will be guided through the onboarding flow to create an admin account.

## Code Quality

Run all checks before submitting a PR:

```bash
make check    # fmt, vet, staticcheck
make sec      # gosec security scan
make vuln     # govulncheck vulnerability scan
make test     # run all tests
```

Or you can run all of them together with:

```bash
make ci
```

## Database Migrations

Migrations live in `migrations/` and use [goose](https://github.com/pressly/goose).

Run pending migrations:

```bash
make migrate
```

Reset the local database:

```bash
make devloop-reset
make devloop-up
make migrate
```

## CI/CD

- **CI**: Runs on every PR — `make check`, security scan, tests, and build. See `.github/workflows/continuous-integration-pipeline.yml`.
- **CD**: Automatically deploys to Fly.io on pushes to `main`.

## Deployment

Deployment is handled automatically by GitHub Actions. To deploy manually:

```bash
flyctl deploy --remote-only
```

See [production-operations.md](docs/production-operations.md) for full operations documentation.
