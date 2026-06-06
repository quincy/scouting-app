# 0007-env-file-config

Runtime configuration via `.env` files and a typed Go struct with OS-environment override.

### Context

The application needs runtime configuration for secrets (session keys, API tokens), environment-specific settings (database URLs, mock mode), and integration endpoints (Scoutbook API, SMTP). These values vary between development, staging, and production and must not be hardcoded or committed to version control.

The existing code uses ad-hoc `os.Getenv()` calls scattered across `main.go` with inconsistent default handling and no structured validation. New settings are added by copying the pattern, which does not scale.

### Decision

Introduce a `config` package with a typed `Config` struct loaded from the environment in three tiers of precedence:

1. **OS environment variables** (highest priority — set by the runtime, e.g. Fly.io secrets, shell exports)
2. **`.env` file** (loaded via `--env=path` flag, or skipped if the flag is absent)
3. **Code defaults** (lowest priority — baked into the binary)

The `.env` file format follows standard conventions: `KEY=value` lines, `#` comments, blank lines skipped. The loader never overwrites an already-set OS environment variable — OS env always wins.

A single `--env` CLI flag, parsed by the standard library `flag` package, points to the `.env` file. This avoids ambient file discovery (e.g. searching for `.env` in parent directories) and makes the config source explicit in the invocation.

### Considered Options

- **Environment-only (no .env file):** Relies entirely on the shell or container runtime to supply every variable. Works well in production (Fly.io, Docker Compose) but burdens developers with exporting a dozen variables before every `go run`. Rejected.
- **TOML/YAML config file:** Typed, structured, supports nesting and arrays. Would require a new dependency and would need to merge with environment variables anyway. Over-engineered for flat key-value config.
- **`.env` file (chosen):** No new dependencies (parsed with `bufio.Scanner` + `strings.Cut`), familiar convention, naturally complements environment-variable injection from deployment platforms. Flat key-value maps directly to `os.Getenv`.
- **Viper:** Powerful but pulls in a dependency tree (fsnotify, etc.) for features this app does not need.

### Consequences

- All runtime configuration is declared in `internal/config/config.go` as a single `Config` struct with doc comments and defaults.
- `config.Load()` must be called at the top of `main()` before any business logic.
- New configuration keys are added in one place (struct field, env var read, default) rather than scattered across the codebase.
- Validation (e.g. `SessionSecret` must be non-empty) happens in one place and fails fast at startup.
- Any `.env` file is gitignored via the `*.env` rule in `.gitignore`. `local.env` in the repository root serves as a reference template.
- Tests can exercise config logic without the `--env` flag by calling `ConfigFromEnv()` and `loadFile()` directly.
