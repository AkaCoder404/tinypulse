<h1><img src="tinypulse.png" width="36" height="36" align="center" style="vertical-align: middle; margin-right: 8px;"> TinyPulse</h1>

A tiny, single-binary, self-hosted uptime monitor for HTTP/HTTPS and TCP endpoints. Drop it on any VPS, NAS, or Raspberry Pi and start monitoring in seconds.

[![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
![Binary Size](https://img.shields.io/badge/binary-%3C%2012%20MB-success)
![Memory](https://img.shields.io/badge/memory-%3C%2030%20MB-success)

## Features

Why use TinyPulse over other uptime monitors? 

- **Tiny Footprint:** ~12 MB binary, < 30 MB RAM at rest.
- **Zero Dependencies:** Single static Go binary with embedded Tailwind CSS frontend. No Docker, no Node.js, no PostgreSQL.
- **Multiple Monitor Types:** Support for HTTP/HTTPS monitors and raw TCP port connections (with more coming soon).
- **Efficient Data Layer:** Pure-Go SQLite with WAL mode, chunked pruning, and decoupled asynchronous writers. Every check result is stored locally in a single `.db` file.
- **Uptime History:** 30-day uptime % and a visual ping hit/miss chart per endpoint.
- **Concurrent Monitoring:** One lightweight, hardened goroutine per endpoint.
- **Alert Notifications:** Built-in support for Telegram and Pushover alerts with per-endpoint linking.
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

| Flag        | Env Var              | Default       | Description                                     |
| ----------- | -------------------- | ------------- | ----------------------------------------------- |
| `-addr`     | `TINYPULSE_ADDR`     | `:8080`       | HTTP listen address                             |
| `-db`       | `TINYPULSE_DB`       | `./uptime.db` | SQLite database path                            |
| `-password` | `TINYPULSE_PASSWORD` | *(empty)*     | Enables Basic Auth (Username is always `admin`) |

---

## Notifications

TinyPulse supports multiple notification channels to alert you when an endpoint goes down or recovers. You can configure these in the UI and link them to specific endpoints. Currently supported: *Telegram, Pushover*

*More notification providers (Slack, Discord, Webhooks) will be added in future releases.*

---

## Contributing

Contributions are welcome! Please open an issue or pull request. Feel free to request features as well.

## License

MIT
