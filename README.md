# 云音解析（Cloud Music Parsing）

中文默认文档。英文文档见 [README_EN.md](./README_EN.md)。

云音解析（Cloud Music Parsing）是一个可自部署的音乐解析系统，当前聚焦网易云音乐解析，后续可扩展到 QQ 音乐等更多平台。

## 功能概览

- 网易云音乐歌曲解析（支持多档音质）
- 网易云音乐歌曲搜索
- 网易云音乐歌单读取与逐首解析
- 首次安装向导（数据库连接测试 + 管理员初始化）
- 管理后台（Cookie 管理、系统设置、代理设置、Redis 设置、SMTP 测试、统计看板）
- 解析记录与审计日志
- 缓存后端可切换（内存 / Redis）

## 技术栈

- 前端：Vue 3、TypeScript、Vite、Naive UI、Pinia
- 后端：Go、Gin、GORM
- 数据库：SQLite（默认）/ MySQL（可选）
- 缓存：Memory（默认）/ Redis（可选）

## 开发环境要求

- Node.js `18+`
- npm
- Go `1.23+`

## 本地开发启动

### 1) 安装依赖

```powershell
cd e:\个人服务\音乐解析\codex
npm install
cd .\backend
go mod tidy
```

### 2) 准备后端配置

```powershell
cd e:\个人服务\音乐解析\codex
New-Item -ItemType Directory -Path .\data -Force | Out-Null
```

说明：`.env.example` 仅用于展示示例配置，不参与启动流程。若不存在 `data/.env`，系统会在安装完成后自动生成。

### 3) 一键启动（推荐）

```powershell
cd e:\个人服务\音乐解析\codex
.\启动开发.ps1
```

### 4) 手动启动（可选）

终端 A：

```powershell
cd e:\个人服务\音乐解析\codex\backend
go run ./cmd/server
```

终端 B：

```powershell
cd e:\个人服务\音乐解析\codex
npm run dev
```

### 5) 访问地址

- 前端：`http://127.0.0.1:8099`
- 后端健康检查：`http://127.0.0.1:8098/api/health`

## Docker 部署

镜像内已包含前端构建产物与后端服务，启动后通过同一端口提供页面与 API。

```powershell
cd e:\个人服务\音乐解析\codex
docker compose up -d
```

- 访问地址：`http://127.0.0.1:8099`
- 健康检查：`http://127.0.0.1:8099/api/health`
- 首次安装前容器会以 `root` 运行；安装完成后服务自动重启并降权为 `app` 用户

## 首次安装流程

1. 打开 `http://127.0.0.1:8099`
2. 按安装向导完成数据库测试与管理员账号创建
3. 安装完成后跳转登录页
4. 登录后进入管理后台

## 项目名称

- 中文名：云音解析
- 英文名：Cloud Music Parsing

## 免责声明

- 本项目仅用于技术学习与研究。
- 请遵守你所在地区的法律法规及平台服务条款。
- 请支持正版音乐版权内容。
