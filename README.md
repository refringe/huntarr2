<p align="center">
  <img src="https://raw.githubusercontent.com/refringe/huntarr2/main/.github/images/banner.png" alt="Huntarr2" />
</p>

[![Build](https://github.com/refringe/huntarr2/actions/workflows/tests.yml/badge.svg)](https://github.com/refringe/huntarr2/actions/workflows/tests.yml)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8.svg)](https://go.dev/)
[![Licence](https://img.shields.io/badge/licence-AGPL--3.0-blue.svg)](LICENSE)

Huntarr2 tells your \*arr apps to search for missing items and quality upgrades so you don't have to do it manually.

It connects to Sonarr, Radarr, Lidarr, and Whisparr, finds monitored items with no file and items that haven't reached their quality cutoff, then triggers searches for them on a schedule. That's it. There is a web UI for configuration and viewing what it's doing.

## How it works

1. You add your \*arr instances (URL + API key) through the web UI.
2. A scheduler runs on a configurable tick interval (default 30 seconds).
3. Each tick, it checks your \*arr libraries for monitored items with no file and items below their quality cutoff.
4. It tells the \*arr app to search for those items (missing first downloads and quality upgrades).
5. Per-item cooldowns prevent the same item from being searched repeatedly.
6. The scheduler adapts its pace to avoid hammering the \*arr APIs.

Huntarr2 does not download anything itself. It just tells your existing \*arr apps to look for items you are missing and better versions of what you already have.

*And no, it doesn't leak your API keys. ;)*

## Running it

Huntarr2 is meant to run as a Docker container alongside your \*arr stack.

```bash
docker compose up -d
```

The web UI will be at `http://localhost:9706`.

On first start, an encryption key is generated automatically and saved to `/config/encryption.key` inside the container. This key encrypts your \*arr API keys in the database. Keep the `/config` volume intact or you will lose access to stored API keys.

### Docker Compose

```yaml
services:
  huntarr2:
    image: ghcr.io/refringe/huntarr2:latest
    ports:
      - '9706:9706'
    volumes:
      - huntarr2_config:/config
    environment:
      PUID: 1000
      PGID: 1000
      TZ: Etc/UTC
    restart: unless-stopped

volumes:
  huntarr2_config:
```

### Unraid / Portainer / Synology / TrueNAS

Set the environment variables through your platform's UI. The container image has sensible defaults for everything. You just need to map `/config` to persistent storage and expose port `9706`.

## Environment variables

| Variable | Default | What it does |
|---|---|---|
| `PORT` | `9706` | HTTP listen port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `DATABASE_PATH` | `/config/huntarr2.db` | Path to the SQLite database file |
| `SCHEDULER_TICK_SECS` | `30` | Seconds between scheduler ticks (minimum 5) |
| `ENCRYPTION_KEY` | auto-generated | 32-byte key as 64-char hex or base64. Auto-generated and persisted in `/config/encryption.key` if not set. Generate your own with `openssl rand -hex 32` |
| `AUTH_USERNAME` | *(empty)* | HTTP Basic Auth username. Set both username and password to enable |
| `AUTH_PASSWORD` | *(empty)* | HTTP Basic Auth password |
| `PUID` | `1000` | UID to run as inside the container |
| `PGID` | `1000` | GID to run as inside the container |
| `TZ` | `UTC` | Container timezone |

## Authentication

Optional. Set both `AUTH_USERNAME` and `AUTH_PASSWORD` to require HTTP Basic Authentication on all routes except the health check (`/api/health`). Leave both empty to run without authentication.

If you are exposing Huntarr2 to the internet, put it behind a reverse proxy with HTTPS.

### Reverse proxy examples

**nginx:**

```nginx
server {
    listen 443 ssl;
    server_name huntarr.example.com;

    location / {
        proxy_pass http://127.0.0.1:9706;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Caddy:**

```
huntarr.example.com {
    reverse_proxy 127.0.0.1:9706
}
```

## Building from source

Requires Go 1.26 or later.

```bash
make build       # produces bin/huntarr2
make test        # runs tests with the race detector
make lint        # runs golangci-lint
make fmt-check   # checks gofmt formatting
```

For local development with Docker:

```bash
docker compose down && make docker-build && make docker-up
```

## Contributing

See [CONTRIBUTING.md](.github/CONTRIBUTING.md).

## Licence

[AGPL-3.0](LICENSE)
