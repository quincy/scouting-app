# 0008-runtime-permission-mapping

Permission-to-role mappings are editable at runtime via an admin UI, replacing the seeded-data-only approach from ADR-0006. Role names are derived from Scoutbook positions and seeded in migrations, but which permissions each role holds is managed through the application.

## Context

The original RBAC design (ADR-0006) treated both role definitions and permission mappings as seeded data — read-only from the application's perspective. This worked for a fixed set of roles with known, stable permissions. However, the set of roles is now driven by Scoutbook positions (30+ strings), many of which have no permissions today but may need them in the future. Requiring a database migration to add `event:view` to the `Scribe` role, for example, is too heavy. An admin should be able to update permission mappings without a deployment.

## Considered Options

- **Seeded-only (rejected):** Keep the original approach. Adding permissions to roles requires a SQL migration. Simple but rigid — every troop-level customization needs a deployment.

- **Runtime admin UI (chosen):** Permission ↔ role mappings are stored in `role_permissions` and editable through an admin page. Role names remain seeded (they come from Scoutbook), but their permissions are runtime data.

- **Fully dynamic roles (rejected):** Allow admins to create arbitrary new role names. Over-engineered — roles are Scoutbook positions, not freeform labels.

## Consequences

- A new permission (e.g. `event:delete`) still requires adding the permission string to the seed migration. Only the *mapping* between permissions and roles is runtime-editable.
- The admin page needs to handle role-permission assignment for all roles, including position-based ones. This is the same interface whether a role currently has zero permissions or many.
- Role assignments to users remain automated (sync-driven for position roles, registration for status roles, admin-manual for privileged roles). This ADR does not change who gets which role — only how permissions are associated with roles.
