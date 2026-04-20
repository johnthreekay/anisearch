# AniSearch

> **Note:** AniSearch is feature-frozen. It was the prototype that led to [Ryokan](https://github.com/johnthreekay/ryokan), a self-hosted anime PVR written in Rust that supersedes this project. Ryokan covers everything AniSearch does and a lot more. AniList-aware metadata, multi-source classification, SeaDex integration, Jellyfin post-processing, a Sonarr/Radarr-compatible API, and a proper scoring engine. If you're starting fresh, use Ryokan. If you want to keep using Sonarr and just want better manual search, try AniSearch.

A self-hosted anime torrent search and download manager that integrates with your existing *arr stack. Search Nyaa, grab torrents to qBittorrent, and trigger Sonarr rescans from one interface.
## Features

- **Nyaa search** with scoring based on preferred release groups, resolution, batch detection
- **One-click grab** to qBittorrent (magnet or .torrent)
- **Sonarr rescan** button
- **Download watcher** that auto-triggers Sonarr rescan when downloads complete
- **Session-based login** with bcrypt-hashed passwords (safe to expose via Cloudflare Tunnel)
- **API key** support for programmatic access

## Quick Start

```bash
docker compose up -d
```

On first launch, visit `http://localhost:8978` and you'll be prompted to create a username and password. Your API key will be displayed after setup — save it for programmatic access.

## Configuration

Config lives in `/config/config.json` (mounted volume). You can also override via environment variables:

| Env Var | Description |
|---|---|
| `ANISEARCH_PORT` | Listen port (default: 8978) |
| `ANISEARCH_APIKEY` | Override API key |
| `ANISEARCH_USERNAME` | Override login username |
| `ANISEARCH_PASSWORD_HASH` | Override bcrypt password hash |
| `QBIT_URL` | qBittorrent WebUI URL |
| `QBIT_USER` | qBittorrent username |
| `QBIT_PASS` | qBittorrent password |
| `SONARR_URL` | Sonarr URL |
| `SONARR_APIKEY` | Sonarr API key |

### Pre-hashing a password

If you want to set the password via env var or config.json directly:

```bash
# Using htpasswd
htpasswd -nbBC 12 "" 'your-password-here' | cut -d: -f2

# Or using Python
python3 -c "import bcrypt; print(bcrypt.hashpw(b'your-password-here', bcrypt.gensalt(12)).decode())"
```

Then set `ANISEARCH_PASSWORD_HASH` to that value, or put it in `config.json` as `"passwordHash"`.

## Auth

- **Web UI**: Session cookie-based login (30-day sessions)
- **API**: Pass `X-Api-Key` header or `?apikey=` query param
- Both methods work for all endpoints

## Docker Compose (anime stack)

```yaml
services:
  anisearch:
    image: ghcr.io/johnthreekay/anisearch:latest
    container_name: anisearch
    restart: unless-stopped
    ports:
      - "8978:8978"
    volumes:
      - ./config:/config
    environment:
      - TZ=America/Chicago
      # Override config.json values via env vars:
      # - ANISEARCH_APIKEY=your-api-key-here
      # - QBIT_URL=http://gluetun:8080      # if qbit runs via gluetun
      # - QBIT_USER=admin
      # - QBIT_PASS=adminadmin
      # - SONARR_URL=http://sonarr:8989
      # - SONARR_APIKEY=your-sonarr-api-key
    networks:
      - anime_net
```

## Building

After any changes, run `go mod tidy` to update dependencies, then rebuild:

```bash
docker compose build --no-cache
docker compose up -d
```
