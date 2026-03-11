# 云音解析（Cloud Music Parsing）

中文默认文档。英文文档见 [README_EN.md](./README_EN.md)。

云音解析是一个可自部署的音乐解析系统，当前聚焦网易云音乐场景，提供前后台一体化部署。

## 功能概览

- 网易云音乐解析（多档音质）
- 歌曲搜索、歌单读取、歌词与封面下载
- 首次安装向导（数据库测试 + 初始化管理员）
- 多用户系统（普通用户 / 管理员 / 超级管理员）
- 用户组管理（默认组、超级管理员组、日次数/并发配额、无限解析开关）
- 站点设置（可配置“解析需登录”）
- 验证码（极验 4.0 bind、Cloudflare Turnstile，默认不启用）
- Cookie 管理、代理、Redis、SMTP、统计与审计日志

## 技术栈

- 前端：Vue 3、TypeScript、Vite、Naive UI、Pinia
- 后端：Go、Gin、GORM
- 数据库：SQLite（默认）/ MySQL（可选）
- 缓存：Memory（默认）/ Redis（可选）

## 环境要求

- Node.js `18+`
- npm
- Go `1.23+`
- Docker / Docker Compose（容器部署可选）

## 本地开发

### 1) 安装依赖

```powershell
npm install
cd backend
go mod tidy
cd ..
```

### 2) 准备数据目录

```powershell
New-Item -ItemType Directory -Path .\data -Force | Out-Null
```

说明：`data/.env` 在完成安装向导后自动生成。

### 3) 启动项目

方式 A（推荐）：

```powershell
.\启动开发.ps1
```

方式 B（手动）：

终端 A：

```powershell
cd backend
go run ./cmd/server
```

终端 B：

```powershell
npm run dev
```

### 4) 默认访问地址

- 前端（开发）：`http://127.0.0.1:8099`
- 后端健康检查（开发）：`http://127.0.0.1:8098/api/health`

## Docker 部署

项目根目录执行：

```powershell
docker build -t cloudmusic .
docker compose up -d
```

- 访问地址：`http://127.0.0.1:8099`
- 健康检查：`http://127.0.0.1:8099/api/health`
- 数据目录映射：`./data -> /app/data`
- 镜像名和容器名：`cloudmusic`

说明：

- 容器内默认时区为 `Asia/Shanghai`（UTC+8）。
- 首次安装前容器以内置流程运行；安装完成后自动重启并降权为 `app` 用户运行。

## 首次安装与初始化规则

- 完成安装后系统会自动创建：
  - 默认组：`默认组`（描述：`默认用户组`）
  - 超级管理员组：`超级管理员组`（描述：`超级管理员用户组`，默认开启无限解析）
- 初始化创建的超级管理员会自动加入超级管理员组。
- 项目强制使用北京时间（UTC+8）进行时间与配额统计。

## 项目名称

- 中文名：云音解析
- 英文名：Cloud Music Parsing

## 免责声明

- 本项目仅用于技术学习与研究。
- 请遵守所在地法律法规及平台服务条款。
- 请支持正版音乐版权内容。
