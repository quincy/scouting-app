# 0004-currency-as-integer-cents

Store event costs as integer cents and project them as readable decimals using generated columns.

### Context
Using floating-point numbers or standard decimals for currency can introduce subtle rounding errors during database aggregations or server-side math. Furthermore, most payment gateways and APIs (such as Stripe) exclusively use integer values representing the smallest currency unit (e.g., cents) to prevent precision errors. Storing cents as an integer is the safest approach, but it reduces readability for database administrators or manual queries.

### Decision
To achieve absolute computational precision and keep the schema readable:
1. The primary database column for event costs will store cents as an integer: `cost_cents INT NOT NULL DEFAULT 0`.
2. To enhance readability, a database-level generated column will project the cost as a decimal format automatically: `cost_decimal DECIMAL(10, 2) GENERATED ALWAYS AS (cost_cents::DECIMAL / 100.0) STORED`.
