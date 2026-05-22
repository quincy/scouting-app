# 0005-database-migrations-with-goose

Use `pressly/goose` for database migrations, starting with an embedded model for the MVP and outlining a transition path to out-of-process execution for production.

### Context
As the application schema evolves, we need a reliable tool to apply incremental, versioned database changes. 
* **MVP Stage:** We prioritize developer velocity and deployment simplicity, aiming for a "zero-configuration" setup where the database is automatically provisioned and kept up-to-date on application startup.
* **Production Stage:** In a multi-replica, distributed, zero-downtime production cluster, running migrations automatically inside the application binary on startup is a bad practice. It introduces race conditions between booting containers, risks database locks, can cause orchestrator boot timeouts on large datasets, and complicates rollbacks.

### Decision
To balance developer velocity now with absolute safety in the future:
1. **Migration Tool:** We select **`pressly/goose`** as our schema migration manager.
2. **MVP Embedded Phase:** We will place our migrations under `migrations/*.sql` and embed them into the Go binary using Go's native `embed` package. The storage initialization layer will programmatically execute `goose.Up()` on server start when `AUTO_MIGRATE=true`.
3. **Future Production Phase (Upgrade Path):** When out-of-process migrations are required:
   - We will set `AUTO_MIGRATE=false` in the production environment variables to disable automatic startup migrations.
   - We will use the standalone `goose` CLI binary in our CI/CD pipeline.
   - The pipeline will run migrations out-of-process (e.g., via a Kubernetes Job or a pre-deployment container release phase) before booting the new Go application instances.
