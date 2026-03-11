# TicketPulse

TicketPulse is a Zendesk and Slack integration platform that provides SLA alerting, tag-based notifications, and configurable daily ticket summaries. It features a web dashboard with Google OAuth authentication for managing users, alert configurations, and Zendesk/Slack settings.

## Features

- **SLA Monitoring** -- Polls Zendesk for tickets approaching or breaching SLA targets and sends alerts to Slack.
- **Tag-Based Alerts** -- Configure per-user tag alerts that notify specific Slack channels when matching tickets are created or updated.
- **Daily Summaries** -- Scheduled daily ticket summaries delivered via Slack DM, with configurable work hours, timezone, and tag/ticket filters.
- **Web Dashboard** -- View alert history and statistics with a browser-based UI.
- **Admin Panel** -- Manage users, tag alert rules, and Zendesk/Slack configuration from the web UI.

## Architecture

```
main.go                   Application entry point and HTTP routing
├── handlers/             HTTP handlers, auth middleware (Google OAuth)
├── services/
│   ├── zendesk.go        Zendesk API client, ticket polling, SLA processing
│   ├── slack.go          Slack client, Socket Mode, alert delivery
│   ├── scheduler.go      Daily summary scheduler
│   ├── summary.go        Zendesk search and daily summary generation
│   ├── polling.go        Testable polling service
│   ├── dashboard.go      Dashboard statistics queries
│   ├── config_cache.go   TTL-backed configuration cache
│   ├── comment_fetcher.go Rate-limited comment fetching
│   └── ticket_context.go Ticket enrichment (requester, org)
├── models/               Data models and database access (users, alerts, config)
├── db/                   Database interface, SQLite initialization, migrations
├── middlewares/          SSE server, notification middleware
├── logging/              Area-based debug logging
├── templates/            HTML templates (dashboard, profile, admin)
└── static/               CSS, JavaScript, images, vendor libraries
```

## Prerequisites

- **Go 1.23+**
- **Google OAuth credentials** (Client ID and Client Secret) for user authentication
- **Zendesk account** with API access (subdomain, email, API key)
- **Slack app** with Bot Token and App Token (Socket Mode enabled)

## Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/TylerConlee/TicketPulse.git
   cd TicketPulse
   ```

2. **Create a `.env` file** in the project root:

   ```env
   GOOGLE_CLIENT_ID=your-google-client-id
   GOOGLE_CLIENT_SECRET=your-google-client-secret
   SESSION_KEY=a-random-base64-session-key
   DB_FILEPATH=ticketpulse.db
   DEBUG_AREAS=all
   ```

   `DEBUG_AREAS` controls verbose logging. Options: `zendesk`, `sla`, `cache`, `slack`, `polling`, `tags`, `scheduler`, `db`, or `all`. Leave empty to disable debug logging.

3. **Run the application:**

   ```bash
   go run .
   ```

   The server starts on `http://localhost:8080`. Log in with Google OAuth, then configure Zendesk and Slack credentials via the Admin > Configuration page.

4. **Build a binary:**

   ```bash
   go build -o ticketpulse .
   ```

## Docker

```bash
docker build -t ticketpulse .
docker run -p 8080:8080 \
  -e GOOGLE_CLIENT_ID=... \
  -e GOOGLE_CLIENT_SECRET=... \
  -e SESSION_KEY=... \
  -e DB_FILEPATH=/data/ticketpulse.db \
  -v ticketpulse-data:/data \
  ticketpulse
```

## Running Tests

```bash
go test -race ./...
```

Tests use an in-memory SQLite database and mock HTTP servers for Zendesk API calls. See `testutil/` for shared test helpers.

## Configuration

Zendesk and Slack settings are stored in the database and managed through the web UI at `/admin/configuration`. Required configuration keys:

| Key | Description |
|-----|-------------|
| `zendesk_subdomain` | Your Zendesk subdomain (e.g. `mycompany`) |
| `zendesk_email` | Zendesk agent email for API authentication |
| `zendesk_api_key` | Zendesk API token |
| `slack_bot_token` | Slack Bot User OAuth Token (`xoxb-...`) |
| `slack_app_token` | Slack App-Level Token for Socket Mode (`xapp-...`) |

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_CLIENT_ID` | Yes | Google OAuth 2.0 Client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | Google OAuth 2.0 Client Secret |
| `SESSION_KEY` | Yes | Secret key for session cookie encryption |
| `DB_FILEPATH` | Yes | Path to the SQLite database file |
| `DEBUG_AREAS` | No | Comma-separated debug logging areas |
