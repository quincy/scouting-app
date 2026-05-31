# 0006-rbac-seeded-data

Role and permission definitions are seeded data (inserted in an idempotent migration), not dynamic runtime records. The application does not expose admin CRUD for roles or permissions in the MVP.

### Context

The RBAC system stores roles and permissions in relational tables with many-to-many join tables. One approach is to treat them as ordinary application data — build admin screens to create/edit/delete roles and reassign permissions at runtime. Another is to treat them as seed data — defined in code, applied via migration, and read-only from the application's perspective.

For the MVP the set of roles (Admin, Scoutmaster, Asst Scoutmaster, Scout, Parent) and their permission mappings are well-known and stable. Building admin CRUD for something that won't change weekly adds complexity without payoff.

### Considered Options

- **Seeded data (chosen):** Roles/permissions live in idempotent `INSERT ... ON CONFLICT DO NOTHING` migrations. Application code references them by name or by constant UUIDs baked into the seed. No runtime mutation.
- **Dynamic CRUD:** Full admin interface to manage roles, permissions, and their mappings. Flexible for custom roles per troop, but adds features, screens, validation, and audit concerns that the MVP doesn't need.
- **Configuration-file driven:** Roles/permissions read from a YAML file on startup. Moves the definition out of the DB but requires a restart to apply changes — worst of both worlds for a multi-replica deployment.

### Consequences

- Adding a new role or permission requires a migration (not just a click in an admin panel).
- A future admin feature would need to split seeded rows from user-created rows, or simply stop seeding and let the admin manage everything from that point forward.
- Application code can safely assume roles like `admin` always exist, simplifying authorization middleware.
