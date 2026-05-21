# 0001-cockroachdb-for-storage

CockroachDB will be used as the primary data store for both production and local development (via Docker).

### Context
We chose CockroachDB primarily because they offer a generous free-tier cloud service that should handle the troop's needs indefinitely.

### Decision
To avoid vendor lock-in and ensure we can migrate if the free tier disappears, we will:
1.  **Protect DB interactions with interfaces**: All database operations must be abstracted behind interfaces defined by the application.
2.  **Decouple Business Logic**: No specific CockroachDB features or SQL dialects should creep into the business logic. The application logic should remain agnostic of the underlying storage implementation.
