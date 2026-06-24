# ADR 0009: Continuous Integration Pipeline for Pull Requests

## Status
Accepted

## Context
As the project grows, we need to ensure that Pull Requests (PRs) do not introduce regressions, security vulnerabilities, or unformatted code. Currently, we have a manual `make ci` target, but no automated enforcement on PRs.

## Decision
We will implement an automated CI pipeline using **GitHub Actions**. The pipeline will trigger on every PR and on pushes to the main branch.

The pipeline will include the following stages:
1.  **Environment Setup**: Install Go 1.25.
2.  **Database Service**: Spin up a **CockroachDB** service container to allow integration tests to run.
3.  **Code Quality**:
    *   `go fmt` (enforced via `make check`)
    *   `go vet`
    *   `staticcheck`
4.  **Security Scanning**:
    *   `gosec` for static analysis security testing.
    *   `govulncheck` to identify known vulnerabilities in dependencies.
5.  **Testing & Coverage**:
    *   Run all tests using `go test ./...`.
    *   Generate a coverage profile and upload it to **Codecov**.
6.  **Build**: Ensure the application compiles successfully.

## Consequences
- PRs will have immediate feedback on whether they meet quality and security standards.
- Developers must ensure their code is formatted and passes all tests before merging.
- We will have a historical record of code coverage.
- Increased CI consumption on GitHub Actions (well within free tier for open source/small projects).
