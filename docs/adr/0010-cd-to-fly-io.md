# ADR 0010: Continuous Deployment to Fly.io

## Status
Accepted

## Context
We need a way to automatically deploy the application to a production environment. Manual deployments are error-prone and slow. We need a hosting provider that supports Dockerized Go applications and integrates well with GitHub Actions.

## Decision
We will use **Fly.io** as our deployment target. 

The CD pipeline will be implemented as follows:
1.  **Infrastructure**: A `fly.toml` configuration file will define the production environment.
2.  **Containerization**: The application will be deployed as a Docker container.
3.  **Automation**: A GitHub Actions workflow (`.github/workflows/cd.yml`) will trigger on every push to the `main` branch that passes the CI pipeline.
4.  **Secrets**: Production secrets (DB connection strings, API keys) will be managed via **GitHub Actions Secrets** and injected into the Fly.io environment using the Fly CLI (`fly secrets set`).
5.  **Migrations**: Database migrations will be run as part of the deployment process (e.g., via a release command in Fly.io).

## Consequences
- The live application will always be up-to-date with the `main` branch.
- Deployment becomes a "hands-off" process once code is merged.
- We rely on Fly.io for production availability.
