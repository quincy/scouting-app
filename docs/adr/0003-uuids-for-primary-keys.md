# 0003-uuids-for-primary-keys

Use random UUIDs for all primary keys in the database tables.

### Context
In distributed databases like CockroachDB, data is automatically split into ordered ranges based on primary keys and distributed across cluster nodes. If sequential auto-incrementing integers (like `SERIAL` or `INT`) are used for primary keys, all new writes will target the exact same range and node at any given time. This creates a severe "write hotspot," defeating the horizontal scaling benefit of CockroachDB's distributed architecture.

### Decision
To ensure optimal performance and prevent write hotspots:
1. All primary key columns across all database tables (Users, Roles, Permissions, Events) will use the `UUID` data type.
2. The default values for these primary keys will be generated automatically on the database side using the `gen_random_uuid()` function.
