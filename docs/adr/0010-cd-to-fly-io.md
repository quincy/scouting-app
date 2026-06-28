# ADR 0010: Continuous Deployment to Fly.io

## Status

Accepted

## Context

We need a way to automatically deploy the application to a production environment. Manual deployments are error-prone and slow. We need a hosting provider that supports Dockerized Go applications and integrates well with GitHub Actions.

## Decision

We will use **Fly.io** as our deployment target.

The CD pipeline is structured as follows:

### Workflows

All workflows live in `.github/workflows/` and are reusable via `workflow_call`:

1. **Continuous Integration Pipeline** (`continuous-integration-pipeline.yml`) — Reusable workflow that runs linting (`make check`), security scanning (`gosec`), vulnerability scanning (`govulncheck`), tests with coverage (uploaded to Codecov), and a production build.

2. **Build Pull Request** (`build-pull-request.yml`) — Triggers on PRs to `main`. Calls the CI pipeline for pre-merge validation.

3. **Dependency Submission** (`dependency-submission.yml`) — Reusable workflow that submits the Go dependency snapshot to GitHub's dependency graph.

4. **Deploy** (`deploy.yml`) — Reusable workflow that runs `flyctl deploy --remote-only` with `FLY_API_TOKEN`.

5. **Continuous Deployment Pipeline** (`continuous-delivery-pipeline.yml`) — Triggers on push to `main`. Runs CI first, then dependency submission and deploy in parallel.

### Containerization

- **Dockerfile:** Multi-stage build producing two binaries: `app` (main application) and `migrate` (standalone migration runner).
- **Base Image:** `gcr.io/distroless/static-debian12` for minimal attack surface.
- **Static Files:** Embedded into the binary via `//go:embed static` — no disk dependency.
- **Database Cert:** The CockroachDB Cloud TLS certificate is baked into the image via `ADD`.

### Infrastructure Config

All Fly.io configuration lives in `fly.toml`:
- `app = 'troop77-app'` — App name on Fly.io.
- Release command runs `/usr/local/bin/migrate` before each deploy.
- Health checks at `/healthcheck` (every 10s) and `/deepcheck` (every 60s).
- VM: 512MB RAM, 1 shared CPU.

### Secrets

Secrets are split across two layers:

**GitHub Secrets (set in repo settings):**

| Secret | Purpose |
|--------|---------|
| `FLY_API_TOKEN` | Fly.io deploy token for `flyctl` authentication |
| `CODECOV_TOKEN` | Token for uploading coverage reports to Codecov |

**Fly Secrets (set via `fly secrets set`):**

| Secret | Purpose |
|--------|---------|
| `DATABASE_URL` | Full CockroachDB connection string with `sslrootcert=/root.crt` |
| `SESSION_SECRET` | Encryption key for session cookies (generated via `openssl rand -hex 32`) |

### Database Migrations

See ADR 0005 for full details. In production, migrations run before each deploy via Fly's release command mechanism.

## Consequences

- The live application is always up-to-date with the `main` branch.
- Deployment is fully automated once code is merged to `main`.
- Migrations run before traffic switches to the new version.
- Secrets are managed at the platform level (Fly) rather than in the application.
- Minimal base image reduces attack surface and deployment size.
