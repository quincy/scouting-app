# 0005-database-migrations-with-goose

Use `pressly/goose` for database migrations, transitioning from startup auto-migration to out-of-process execution via a release command.

### Context

As the application schema evolves, we need a reliable tool to apply incremental, versioned database changes. 
* **MVP Stage:** We prioritized developer velocity with a "zero-configuration" setup where the database was automatically provisioned on application startup.
* **Production Stage:** In a multi-replica, distributed, zero-downtime production cluster, running migrations automatically inside the application binary on startup introduces race conditions between booting containers, risks database locks, and complicates rollbacks.

### Decision

1. **Migration Tool:** We selected **`pressly/goose`** as our schema migration manager.
2. **Migration SQL:** Stored under `migrations/*.sql` with `-- +goose Up` / `-- +goose Down` annotations.
3. **Embedded Library:** A shared embed package at `migrations/embed.go` (`package migrations`) exposes the embedded SQL files via `migrations.FS`, importable by both the app and the migrate binary.
4. **Standalone Migrate Binary:** A separate binary at `cmd/migrate/main.go` opens the database, runs `goose.Up`, and exits. It reads `DATABASE_URL` from the environment and accepts an optional `--env` flag for local env file loading.
5. **Release Command:** On Fly.io, migrations run via the `[deploy] release_command = "/usr/local/bin/migrate"` setting in `fly.toml`. This executes the migrate binary in a one-off container before the new version starts serving traffic.
6. **Runtime Auto-Migrate Removed:** The `AutoMigrate` startup path was removed from the main app binary. The app no longer runs migrations on startup — that responsibility belongs solely to the release command.
7. **Local Dev:** Developers run migrations explicitly via `make migrate`, which calls `go run ./cmd/migrate/ --env=local.env`. The `make run` target chains `devloop-up → migrate → app start`.

### Consequences

- Migrations complete before traffic reaches the new version (zero-downtime safe).
- No race conditions between app instances on startup.
- The migrate binary is a separate artifact in the Docker image (~2MB), distinct from the app binary.
- Developers must run `make migrate` (or `go run ./cmd/migrate/ --env=local.env`) explicitly when iterating on schema changes.
