<p align="center">
  <img src="./src/assets/logo-yunyin-full.svg" alt="Cloud Music Parsing" width="560">
</p>

<p align="center">
  Self-hosted Netease Cloud Music parsing system with integrated frontend & backend
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue.svg" alt="AGPL-3.0 License"></a>
  <img src="https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white" alt="Go 1.23">
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white" alt="Vue 3">
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white" alt="Docker Ready">
</p>

<p align="center">
  English | <a href="./README.md">中文</a>
</p>

---

## Important

### Serious Warning

Please strictly comply with the GNU Affero General Public License (AGPL-3.0).

In any modified, derived, redistributed, or commercialized version of this project, you must also adopt AGPL-3.0 and retain appropriate license and copyright notices from this project.

If you use this project for sales or other commercial purposes, you must provide the complete source code of your project and a link to this original project. This project may involve third-party components; commercial use may carry legal or litigation risks. If any license violation is found, the author reserves the right to pursue legal responsibility.

Modifying the original copyright notice in secondary development is prohibited (you may add your own secondary-author information).

Thank you for your respect and understanding.


## Features

- 🎵 Netease Cloud Music parsing (multiple quality levels)
- 🔍 Song search, playlist parsing, lyric & cover download
- 🧙 First-run installation wizard (DB test + admin setup)
- 👥 Multi-user system (user / admin / super admin) with group-based quotas
- 🛡️ Captcha support (Geetest v4, Cloudflare Turnstile)
- ⚙️ Cookie management, proxy, Redis cache, SMTP mail, audit logs

## Tech Stack

| Layer | Technologies |
|:------|:-------------|
| Frontend | Vue 3 · TypeScript · Vite · Naive UI · Pinia |
| Backend | Go 1.23 · Gin · GORM |
| Database | SQLite (default) / MySQL |
| Cache | In-memory (default) / Redis |

## Quick Start

### Docker (Recommended)

```bash
docker build -t cloudmusic .
docker compose up -d
```

Visit `http://127.0.0.1:8099` and follow the setup wizard.

- Data persistence: `./data → /app/data`
- Built-in HEALTHCHECK probes `/api/health` every 30s
- After installation, the process drops privileges to the `app` user

### Local Development

```bash
# Install dependencies
npm install
cd backend && go mod tidy && cd ..

# Start (recommended)
./启动开发.ps1

# Or manually: Terminal A for backend, Terminal B for frontend
cd backend && go run ./cmd/server   # Terminal A
npm run dev                          # Terminal B
```

| Service | URL |
|:--------|:----|
| Frontend (dev) | `http://127.0.0.1:8099` |
| Backend health | `http://127.0.0.1:8098/api/health` |

## Environment Variables

| Variable | Default | Description |
|:---------|:--------|:------------|
| `APP_PORT` | `8099` | Listen port |
| `TZ` | `Asia/Shanghai` | Timezone |
| `GIN_MODE` | `release` | Gin mode |
| `RATE_LIMIT` | `30` | Max requests per IP per endpoint per window |
| `RATE_WINDOW_SEC` | `60` | Rate limit window (seconds) |

## Security & Performance

| Feature | Details |
|:--------|:--------|
| Graceful shutdown | SIGINT / SIGTERM handling, request draining + connection cleanup |
| HTTP timeouts | Read header 10s · Read 30s · Write 60s · Idle 120s |
| JWT refresh | Silent token refresh before expiry with concurrent request queuing |
| Audit masking | Request body truncated to 4KB, password/token/cookie fields auto-masked |
| Scheduled cleanup | Audit logs & parse records purged after 90 days, expired codes after 7 days |
| CDN-friendly | API responses `no-store`, hashed assets cached long-term, `index.html` never cached |
| Rate limiting | IP + path based in-memory limiter, configurable via env |
| Database | All model fields properly indexed |
| Frontend | Route lazy loading · tree-shaken component imports |

## Installation Defaults

After setup, the system automatically creates:

- **Default group** — new users are assigned here on registration
- **Super admin group** — unlimited parsing, super admin auto-assigned

All time and quota calculations use Beijing time (UTC+8).

## License

[GNU Affero General Public License v3.0 (AGPL-3.0)](./LICENSE)

## Disclaimer

This project is for technical learning and research only. Please comply with local laws and platform terms of service. Support officially licensed music.
