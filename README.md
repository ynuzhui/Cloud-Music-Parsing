<p align="center">
  <img src="./src/assets/logo-yunyin-full.svg" alt="云音解析" width="560">
</p>

<p align="center">
  可自部署的网易云音乐解析系统，前后端一体化部署
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue.svg" alt="AGPL-3.0 License"></a>
  <img src="https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white" alt="Go 1.23">
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vuedotjs&logoColor=white" alt="Vue 3">
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white" alt="Docker Ready">
</p>

<p align="center">
  <a href="./README_EN.md">English</a> | 中文
</p>

---

## Important

### 严肃警告

请务必遵守 GNU Affero General Public License (AGPL-3.0) 许可协议。

在您的修改、演绎、分发或派生项目中，必须同样采用 AGPL-3.0 许可协议，并在适当的位置包含本项目的许可和版权信息。

若您用于售卖或其他盈利用途，必须提供本项目的源代码及原项目链接。另外由于本项目涉及第三方，售卖后可能遭受法律或诉讼风险。如若发现违反许可协议，作者保留追究法律责任的权利。

禁止在二开项目中修改程序原版权信息（您可以添加二开作者信息）。

感谢您的尊重与理解。


## 功能

- 🎵 网易云音乐解析（多档音质）
- 🔍 歌曲搜索、歌单解析、歌词与封面下载
- 🧙 首次安装向导（数据库连接测试 + 管理员初始化）
- 👥 多用户体系（用户 / 管理员 / 超级管理员）与用户组配额管理
- 🛡️ 验证码支持（极验 4.0、Cloudflare Turnstile）
- ⚙️ Cookie 管理、代理、Redis 缓存、SMTP 邮件、审计日志

## 技术栈

| 层 | 技术 |
|:---|:-----|
| 前端 | Vue 3 · TypeScript · Vite · Naive UI · Pinia |
| 后端 | Go 1.23 · Gin · GORM |
| 数据库 | SQLite（默认）/ MySQL |
| 缓存 | 内存（默认）/ Redis |

## 快速开始

### Docker 部署（推荐）

```bash
docker build -t cloudmusic .
docker compose up -d
```

访问 `http://127.0.0.1:8099`，按向导完成安装。

- 数据持久化：`./data → /app/data`
- 内置 HEALTHCHECK，每 30 秒探测 `/api/health`
- 安装完成后自动降权为 `app` 用户运行

### 本地开发

```bash
# 安装依赖
npm install
cd backend && go mod tidy && cd ..

# 启动（推荐）
./启动开发.ps1

# 或手动启动：终端 A 运行后端，终端 B 运行前端
cd backend && go run ./cmd/server   # 终端 A
npm run dev                          # 终端 B
```

| 服务 | 地址 |
|:-----|:-----|
| 前端（开发） | `http://127.0.0.1:8099` |
| 后端健康检查 | `http://127.0.0.1:8098/api/health` |

## 环境变量

| 变量 | 默认值 | 说明 |
|:-----|:-------|:-----|
| `APP_PORT` | `8099` | 监听端口 |
| `TZ` | `Asia/Shanghai` | 时区 |
| `GIN_MODE` | `release` | Gin 模式 |
| `RATE_LIMIT` | `30` | 单 IP 单接口每窗口最大请求数 |
| `RATE_WINDOW_SEC` | `60` | 限流窗口（秒） |

## 安全与性能

| 特性 | 说明 |
|:-----|:-----|
| 优雅关机 | SIGINT / SIGTERM 信号处理，请求排空 + 连接释放 |
| HTTP 超时 | 读取头 10s · 读取 30s · 写入 60s · 空闲 120s |
| JWT 续签 | Token 过期前自动刷新，并发请求排队等待 |
| 审计脱敏 | 请求体截断 4KB，密码/Token/Cookie 等字段自动遮蔽 |
| 定时清理 | 审计日志与解析记录 90 天清理，过期验证码 7 天清理 |
| CDN 兼容 | API 响应 `no-store`，构建产物长期缓存，`index.html` 禁缓存 |
| 限流 | IP + 路径维度内存限流，环境变量可调 |
| 数据库 | 全模型索引优化 |
| 前端 | 路由懒加载 · 组件按需引入 |

## 初始化规则

安装完成后系统自动创建：

- **默认组**（`默认用户组`）— 新注册用户自动加入
- **超级管理员组**（`超级管理员用户组`）— 无限解析，超级管理员自动归属

时间与配额统计强制使用北京时间（UTC+8）。

## 许可证

[GNU Affero General Public License v3.0 (AGPL-3.0)](./LICENSE)

## 免责声明

本项目仅用于技术学习与研究，请遵守当地法律法规及平台服务条款，支持正版音乐。
