# Cloud Music Parsing

English documentation. Chinese version: [README.md](./README.md).

Cloud Music Parsing is a self-hosted music parsing system.
It currently focuses on Netease Cloud Music and is designed to be extendable to more providers (such as QQ Music) in future versions.

## Features

- Netease song parsing with multiple quality levels
- Netease song search
- Netease playlist fetch and per-track parsing
- First-run installation wizard (DB test + admin initialization)
- Admin console (cookie management, system settings, proxy, Redis, SMTP test, dashboard)
- Parse records and audit logs
- Switchable cache backend (memory / Redis)

## Tech Stack

- Frontend: Vue 3, TypeScript, Vite, Naive UI, Pinia
- Backend: Go, Gin, GORM
- Database: SQLite (default) / MySQL (optional)
- Cache: Memory (default) / Redis (optional)

## Requirements

- Node.js `18+`
- npm
- Go `1.23+`

## Local Development

### 1) Install dependencies

```powershell
cd e:\个人服务\音乐解析\codex
npm install
cd .\backend
go mod tidy
```

### 2) Prepare backend config

```powershell
cd e:\个人服务\音乐解析\codex
New-Item -ItemType Directory -Path .\data -Force | Out-Null
```

Note: `.env.example` is for reference only and is not used by the startup flow.  
If `data/.env` is missing, it will be generated automatically after installation.

### 3) Quick start (recommended)

```powershell
cd e:\个人服务\音乐解析\codex
.\启动开发.ps1
```

### 4) Manual start (optional)

Terminal A:

```powershell
cd e:\个人服务\音乐解析\codex\backend
go run ./cmd/server
```

Terminal B:

```powershell
cd e:\个人服务\音乐解析\codex
npm run dev
```

### 5) URLs

- Frontend: `http://127.0.0.1:8099`
- Backend health check: `http://127.0.0.1:8098/api/health`

## Docker Deployment

The image now bundles both frontend static assets and backend service.
After startup, UI and API are served on the same port.

```powershell
cd e:\个人服务\音乐解析\codex
docker compose up -d
```

- App URL: `http://127.0.0.1:8099`
- Health check: `http://127.0.0.1:8099/api/health`
- Before first install, the container runs as `root`; after installation it auto-restarts and drops to the `app` user

## Project Name

- Chinese: 云音解析
- English: Cloud Music Parsing

## Disclaimer

- This project is for technical learning and research only.
- Please comply with applicable laws and platform terms of service.
- Please support officially licensed music content.
