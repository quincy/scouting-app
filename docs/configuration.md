# Configuration

All configuration values follow a three-tier hierarchy:

1. **Environment variable** — overrides everything, useful for local dev
2. **Database (`app_config` table)** — set via the admin settings page
3. **Hardcoded default** — sensible default when nothing else is set

## Unit Configuration

| Setting            | Env Var              | DB Key               | Default            |
|--------------------|----------------------|----------------------|--------------------|
| Unit Type          | `UNIT_TYPE`          | `UNIT_TYPE`          | `Troop`            |
| Unit Number        | `UNIT_NUMBER`        | `UNIT_NUMBER`        | (empty)            |
| Scoutbook Org GUID | `SCOUTBOOK_ORG_GUID` | `SCOUTBOOK_ORG_GUID` | (empty)            |
| Default Timezone   | `DEFAULT_TIMEZONE`   | `DEFAULT_TIMEZONE`   | `America/New_York` |

## Email (SMTP)

| Setting           | Env Var     | DB Key      | Default     |
|-------------------|-------------|-------------|-------------|
| SMTP Host         | `SMTP_HOST` | `SMTP_HOST` | `localhost` |
| SMTP Port         | `SMTP_PORT` | `SMTP_PORT` | `1025`      |
| SMTP Username     | `SMTP_USER` | `SMTP_USER` | (empty)     |
| SMTP Password     | `SMTP_PASS` | `SMTP_PASS` | (empty)     |
| SMTP From Address | `SMTP_FROM` | `SMTP_FROM` | (empty)     |

Email settings can be managed:

- Locally via `.env` file or environment variables
- In production via the admin settings page (`/admin/settings`)

### MailHog (local development)

A MailHog service is included in `docker-compose.yml` for local email testing:

- **SMTP**: `localhost:1025`
- **Web UI**: `http://localhost:8025`

## Server Configuration

These values are only configurable via environment variables:

| Setting                | Env Var                  | Default                    |
|------------------------|--------------------------|----------------------------|
| Address                | `ADDR`                   | `:8080`                    |
| Database URL           | `DATABASE_URL`           | (required)                 |
| Session Secret         | `SESSION_SECRET`         | (required)                 |
| Scoutbook API Base URL | `SCOUTBOOK_API_BASE_URL` | `https://api.scouting.org` |
| Scoutbook Token        | `SCOUTBOOK_TOKEN`        | (empty)                    |

## Environment File

You can use a `.env` file to load local configuration:

```bash
go run . -env=.env
```

The `.env` file format is `KEY=VALUE` (one per line). Values set in the environment take precedence over the file.
