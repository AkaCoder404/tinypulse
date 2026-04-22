<h1><img src="tinypulse.png" width="36" height="36" align="center" style="vertical-align: middle; margin-right: 8px;"> TinyPulse</h1>

A tiny, single-binary, self-hosted uptime monitor for HTTP/HTTPS and TCP endpoints. Drop it on any VPS, NAS, or Raspberry Pi and start monitoring in seconds.

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Binary Size](https://img.shields.io/badge/binary-%3C%2012%20MB-success)
![Memory](https://img.shields.io/badge/memory-%3C%2030%20MB-success)

## Features

Why use TinyPulse?

- **Tiny Footprint:** ~12 MB binary, < 30 MB RAM at rest.
- **Near Zero Dependencies:** Single static Go binary with embedded Tailwind CSS frontend. No Docker, no Node.js, no PostgreSQL.
- **Multiple Monitor Types:** Support for HTTP/HTTPS monitors and raw TCP port connections (with more coming soon).
- **Efficient Data Layer:** Pure-Go SQLite with WAL mode, chunked pruning, and decoupled asynchronous writers. Every check result is stored locally in a single `.db` file.
- **High Performance:** An in-memory cache and background aggregation engine guarantee instant dashboard load times, even with years of historical data.
- **Massive Concurrency:** One lightweight, hardened goroutine per endpoint. Effortlessly monitor 1,000+ endpoints concurrently on a single CPU core.
- **Uptime History:** 24 hour uptime % and a visual ping hit/miss chart per endpoint. 90 day auto cleanup.
- **Alert Notifications:** Built-in support for Telegram, Pushover (and more) alerts with per-endpoint linking.
- **Customizable Thresholds:** Configure exactly how many consecutive failures trigger an alert per endpoint.
- **Built-in Security:** Secure the dashboard and API instantly with HTTP Basic Auth (`TINYPULSE_PASSWORD`).
- **REST API:** Fully featured `/api` endpoints for programmatic access and automation.

## Quick Start

### Download a release

Grab the latest binary for your platform from the [Releases](https://github.com/AkaCoder404/tinypulse/releases) page.

```bash
chmod +x tinypulse
TINYPULSE_PASSWORD=supersecret ./tinypulse
```
Then open [http://localhost:8080](http://localhost:8080) and log in with username `admin` and your password.

> Please see the [Deployment and Updates Guide](docs/deployment_and_updates.md) for an example on how to set up TinyPulse on a server. Also contains information how to update from a previous version.

### Build from source

```bash
git clone https://github.com/AkaCoder404/tinypulse.git
cd tinypulse
make build
./tinypulse
```

**Requirements:** Go 1.21+

---

## Configuration

TinyPulse can be configured via flags or environment variables.

| Flag            | Env Var              | Default          | Description                                     |
| --------------- | -------------------- | ---------------- | ----------------------------------------------- |
| `-addr`         | `TINYPULSE_ADDR`     | `:8080`          | HTTP listen address                             |
| `-db`           | `TINYPULSE_DB`       | `./tinypulse.db` | SQLite database path                            |
| `-password`     | `TINYPULSE_PASSWORD` | *(empty)*        | Enables Basic Auth (Username is always `admin`) |
| `-config`       | `TINYPULSE_CONFIG`   | *(empty)*        | Path to a YAML configuration file               |
| `-dry-run`      | *(none)*             | `false`          | Parse config, preview DB changes, and exit      |
| `-eject-config` | *(none)*             | `false`          | Unlock config-managed items back to UI control  |
| `-export-config`| *(none)*             | *(empty)*        | Export current database to a YAML config file   |

### Configuration as Code (YAML)

TinyPulse supports declarative provisioning via a YAML configuration file. This allows you to manage your endpoints and notifiers using GitOps or CI/CD pipelines alongside your existing UI-created items. 

When you define an item in the YAML file:
1. It is automatically created or updated in the database on startup.
2. It becomes **Read-Only** in the web dashboard (a purple `Config` badge will appear).
3. If you remove it from the YAML file, it is safely deleted from the database on the next startup.

*Any items you create manually through the web UI are completely safe and will not be touched by the YAML sync.*

#### Usage

Create a `config.yml` (see [example_config.yml](example_config.yml) for a full example):

```yaml
endpoints:
  my_website:
    name: "My Website"
    type: "http"
    url: "https://example.com"
    interval_seconds: 60
    notifiers:
      - telegram_ops

notifiers:
  telegram_ops:
    name: "Ops Team Telegram"
    type: "telegram"
    config:
      bot_token: "${TELEGRAM_BOT_TOKEN}" # Supports environment variables!
      chat_id: "-1001234567890"
```

Start TinyPulse with the config flag:
```bash
TELEGRAM_BOT_TOKEN="123:abc" ./tinypulse -config config.yml
```

#### Dry Run

You can safely preview what changes the YAML file will apply to your database without actually writing anything to the disk. This is perfect for CI/CD validation.

```bash
./tinypulse -config config.yml -dry-run
```

#### Exporting Configuration

If you have already set up monitors and notifiers using the Web UI and want to transition to a Configuration-as-Code workflow, you can export your current database state to a YAML file:

```bash
./tinypulse -export-config my_new_config.yml
```

This will generate a ready-to-use YAML file containing all your existing endpoints and notifiers. You can then use this file to start TinyPulse in the future.

#### Ejecting Configuration

If you start the server *without* a `-config` flag, but the database still contains items created by a previous config file, those items will remain running but will be locked in the UI (to prevent accidental data loss).

If you want to permanently stop using the YAML file and manage those specific items via the Web UI again, run the eject command:

```bash
./tinypulse -eject-config
```
This safely transitions all config-managed items back to UI control without losing their uptime history.

---

## Notifications

TinyPulse supports multiple notification channels to alert you when an endpoint goes down or recovers. You can configure these in the UI and link them to specific endpoints. Currently supported: *Telegram, Pushover, Redis*

*More notification providers (Slack, Discord, Webhooks) will be added in future releases.*

## REST API

TinyPulse includes a full JSON REST API, which powers the dashboard and can be used for automation. If you set a `TINYPULSE_PASSWORD`, you must provide it via HTTP Basic Auth (Username: `admin`).

**Endpoints (`/api/endpoints`)**
- `GET /api/endpoints` - List all endpoints with their current 30-day uptime statistics.
- `POST /api/endpoints` - Create a new endpoint.
- `GET /api/endpoints/{id}` - Get a specific endpoint.
- `PUT /api/endpoints/{id}` - Update a specific endpoint.
- `DELETE /api/endpoints/{id}` - Delete an endpoint and all its history.
- `POST /api/endpoints/{id}/pause` - Toggle pause/resume for monitoring.
- `GET /api/endpoints/{id}/history?limit=60` - Get lightweight, recent ping data for the visual timeline.
- `GET /api/endpoints/{id}/checks?limit=100` - Get full raw check history (status codes, response times).

**Notifiers (`/api/notifiers`)**
- `GET /api/notifiers` - List all notifiers.
- `POST /api/notifiers` - Create a new notifier.
- `GET /api/notifiers/{id}` - Get a specific notifier.
- `PUT /api/notifiers/{id}` - Update a specific notifier.
- `DELETE /api/notifiers/{id}` - Delete a notifier.
- `POST /api/notifiers/{id}/test` - Trigger a test alert to verify credentials.

---

## Performance

> TODO: Requires more comprehensive testing in different environment.

You can take a look at the ![performance](docs/performance.md) docs for some testing.

## Contributing

Contributions are welcome! Please open an issue or pull request. Feel free to request features as well.

## Acknowledgements

Heavy expired by Uptime-Kuma!

## License

MIT
