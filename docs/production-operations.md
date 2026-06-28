# Production Operations

## Architecture Overview

```
PR → Build Pull Request → CI (lint, sec, test, build)
main push → CD Pipeline → CI → dependency-submission
                           └──→ deploy (flyctl deploy --remote-only)
                                  ├── release_command: /usr/local/bin/migrate
                                  └── app starts serving traffic
```

## Prerequisites

- [Fly.io](https://fly.io) account with `flyctl` installed.
- [GitHub CLI](https://cli.github.com/) (`gh`) for managing repo secrets.
- Access to the CockroachDB Cloud cluster.

## Services

| Service        | Provider                | Location                        |
|----------------|-------------------------|---------------------------------|
| Hosting        | Fly.io                  | `troop77-app` (SJC region)      |
| Database       | CockroachDB Cloud       | Serverless cluster, `defaultdb` |
| CI/CD          | GitHub Actions          | `quincy/scouting-app`           |
| Code Quality   | Codecov                 | PR coverage reporting           |
| Image Registry | Fly.io builder (remote) | Built on each deploy            |

## Secrets

### GitHub Secrets

Set in `quincy/scouting-app` → Settings → Secrets and variables → Actions:

```bash
gh secret set FLY_API_TOKEN -R quincy/scouting-app
gh secret set CODECOV_TOKEN -R quincy/scouting-app
```

| Secret          | How to get                    |
|-----------------|-------------------------------|
| `FLY_API_TOKEN` | `flyctl tokens create deploy` |
| `CODECOV_TOKEN` | Codecov repo settings page    |

### Fly Secrets

Set via `flyctl` (one-time, persist across deploys):

```bash
fly secrets set DATABASE_URL="postgres://user:pass@host:26257/defaultdb?sslmode=verify-full&sslrootcert=/root.crt"
fly secrets set SESSION_SECRET="$(openssl rand -hex 32)"
```

| Secret           | Source                                                         |
|------------------|----------------------------------------------------------------|
| `DATABASE_URL`   | CockroachDB Cloud connection string + `&sslrootcert=/root.crt` |
| `SESSION_SECRET` | Generated locally, never committed                             |

## Deployment

### Automated (CI/CD)

Merge to `main` → GitHub Actions runs the CD pipeline. No manual steps.

### Manual / Test Deploy

```bash
# Build and verify locally
docker build -t scout-app .

# Deploy to Fly
flyctl deploy --remote-only
```

To test locally with a real CockroachDB container:

```bash
make devloop-up     # Start CockroachDB in Docker
make migrate        # Run schema migrations
make run            # Build and start the app
```

## Database Migrations

### Adding a Migration

Create a new SQL file in `migrations/`:

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN phone TEXT;

-- +goose Down
ALTER TABLE users DROP COLUMN phone;
```

Number files sequentially: `00010_description.sql`.

### Running Migrations Locally

```bash
make migrate
```

This runs `go run ./cmd/migrate/ --env=local.env`.

### How Migrations Work in Production

1. On `git push main`, the CD pipeline triggers.
2. Docker build produces two binaries: `app` and `migrate`.
3. Fly deploys the image and runs `release_command = "/usr/local/bin/migrate"` in a one-off container.
4. The migrate binary connects to the database using `DATABASE_URL` (Fly secret) and runs `goose.Up`.
5. Only on success does Fly route traffic to the new version.

## CI/CD Workflows

| File                                  | Trigger        | What it does                                       |
|---------------------------------------|----------------|----------------------------------------------------|
| `build-pull-request.yml`              | PR to `main`   | Runs CI pipeline only                              |
| `continuous-delivery-pipeline.yml`    | Push to `main` | CI → dependency-submission + deploy                |
| `continuous-integration-pipeline.yml` | (reusable)     | Lint, sec scan, vuln scan, test, build             |
| `dependency-submission.yml`           | (reusable)     | Submits Go dependency graph to GitHub              |
| `deploy.yml`                          | (reusable)     | `flyctl deploy --remote-only` with `FLY_API_TOKEN` |

## Binary Structure

The Docker image contains two binaries:

| Binary                   | Source                | Purpose                        |
|--------------------------|-----------------------|--------------------------------|
| `/usr/local/bin/app`     | Root `main.go`        | Application server (HTMX + Go) |
| `/usr/local/bin/migrate` | `cmd/migrate/main.go` | Standalone migration runner    |

Static files (`app.css`, `htmx.min.js`) are compiled into the `app` binary via `//go:embed static`.

## Monitoring

- **Fly.io Dashboard:** `https://fly.io/apps/troop77-app/monitoring`
- **Health Check:** `GET /healthcheck` (simple) and `GET /deepcheck` (includes DB ping)
- **Logs:** `flyctl logs`
- **SSH into VM:** `flyctl ssh console`
