# Cloud Music Parsing

English documentation. Chinese version: [README.md](./README.md).

Cloud Music Parsing is a self-hosted music parsing system focused on Netease Cloud Music, with an integrated frontend + backend deployment flow.

## Features

- Netease parsing with multiple quality levels
- Song search, playlist parsing, lyric/cover download
- First-run installation wizard (DB test + initial admin setup)
- Multi-user system (user / admin / super admin)
- User groups (default group, super-admin group, daily/concurrency quota, unlimited mode)
- Site settings (including "parse requires login")
- Captcha support (Geetest v4 bind, Cloudflare Turnstile; disabled by default)
- Cookie management, proxy, Redis, SMTP, dashboard, audit logs

## Tech Stack

- Frontend: Vue 3, TypeScript, Vite, Naive UI, Pinia
- Backend: Go, Gin, GORM
- Database: SQLite (default) / MySQL (optional)
- Cache: Memory (default) / Redis (optional)

## Requirements

- Node.js `18+`
- npm
- Go `1.23+`
- Docker / Docker Compose (optional for container deployment)

## Local Development

### 1) Install dependencies

```powershell
npm install
cd backend
go mod tidy
cd ..
```

### 2) Prepare data directory

```powershell
New-Item -ItemType Directory -Path .\data -Force | Out-Null
```

Note: `data/.env` is generated automatically after installation.

### 3) Start

Option A (recommended):

```powershell
.\启动开发.ps1
```

Option B (manual):

Terminal A:

```powershell
cd backend
go run ./cmd/server
```

Terminal B:

```powershell
npm run dev
```

### 4) Default URLs

- Frontend (dev): `http://127.0.0.1:8099`
- Backend health (dev): `http://127.0.0.1:8098/api/health`

## Docker Deployment

From project root:

```powershell
docker build -t cloudmusic .
docker compose up -d
```

- App URL: `http://127.0.0.1:8099`
- Health check: `http://127.0.0.1:8099/api/health`
- Data volume: `./data -> /app/data`
- Image and container name: `cloudmusic`

Notes:

- Container timezone is set to `Asia/Shanghai` (UTC+8).
- Before installation completes, startup follows the built-in init flow; after installation it restarts and runs as the `app` user.

## Installation Defaults

After installation, the system ensures:

- Default group: `默认组` (`默认用户组`)
- Super admin group: `超级管理员组` (`超级管理员用户组`, unlimited mode enabled by default)
- The super admin account is automatically assigned to the super admin group

The project enforces Beijing time (UTC+8) for time/quota calculations.

## Disclaimer

- This project is for technical learning and research only.
- Please comply with local laws and platform terms of service.
- Please support officially licensed music content.
