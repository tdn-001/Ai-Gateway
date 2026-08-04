<p align="center">
  <img src="frontend/public/logo.png" alt="AI Gateway Logo" width="120" />
</p>

<h1 align="center">AI Gateway</h1>

<p align="center">
  <strong>兼容 OpenAI API 协议的智能模型网关</strong>
  <br />
  统一入口 · 多节点智能路由 · 异常恢复 · 流式缓冲 · Web 管理后台
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go" />
  <img src="https://img.shields.io/badge/Vue-3.5-4FC08D?logo=vue.js" />
  <img src="https://img.shields.io/badge/License-MIT-green" />
</p>

---

## 📖 项目简介

AI Gateway 是一个兼容 OpenAI API 协议的智能模型网关，作为用户访问模型服务的统一入口。它的核心价值在于自动处理请求过程中的 429、500、502 等异常错误，让应用始终只收到成功的模型返回。

**请求链路：**

```
用户客户端 → AI Gateway → 模型节点（每个节点独立 URL，自动轮询 Key）
```

> AI Gateway 直接管理多个模型节点，支持节点内多 Key 自动轮询。无需依赖 Nginx 做负载均衡与节点切换，所有路由逻辑由网关内置完成。

---

## ✨ 功能特性

- **OpenAI 兼容** — 完全兼容 `POST /v1/chat/completions`，支持 `stream=true/false`
- **SSE 流式处理** — 实时读取并转发 SSE 流，缓存已输出内容
- **异常自动恢复** — 支持 429、500、502、503、504 等异常，自动重试与恢复
- **两种恢复模式** — 模式 A（接续生成）/ 模式 B（完整重生成），支持自定义提示词模板
- **流式缓冲模式** — 可选择将上游内容完全缓存后一次性输出，失败时客户端不会看到部分内容
- **Web 管理后台** — 配置管理、提示词管理、API Key 管理、节点管理、日志查看、统计图表
- **API Key 认证** — 支持多 Key 管理、启用/禁用、调用记录追踪
- **多节点管理** — 支持多个模型节点，每个节点独立 URL、启用/禁用控制
- **节点内 Key 轮询** — 每个节点支持多个 API Key，严格顺序轮询，连续失败 3 次自动换 Key
- **纯内存存储** — 不依赖 Redis、MySQL，运行状态全内存，配置本地文件持久化
- **Docker 部署** — 单容器运行，前端资源嵌入 Go 二进制

---

## 🚀 快速开始

### 前置要求

- Go 1.23+（本地编译）
- Node.js 20+（构建前端，仅在本地开发时需要）
- Docker & Docker Compose（容器部署）

### 方式一：Docker Compose 部署（推荐）

```bash
# 克隆仓库
git clone https://github.com/tdn-001/Ai-Gateway.git
cd Ai-Gateway

# 创建数据目录
mkdir -p data

# 构建并启动
docker compose up -d --build

# 查看日志
docker compose logs -f

# 停止
docker compose down
```

访问 `http://localhost:3301` 进入后台管理，首次访问会提示注册管理员账号。

### 方式二：本地编译运行

**Linux / macOS：**

```bash
# 克隆仓库
git clone https://github.com/tdn-001/Ai-Gateway.git
cd Ai-Gateway

# 构建前端
cd frontend
npm install
npm run build
cd ..

# 编译并运行
go build -o ai-gateway .
./ai-gateway
```

**Windows：**

```bash
# 克隆仓库
git clone https://github.com/tdn-001/Ai-Gateway.git
cd Ai-Gateway

# 构建前端
cd frontend
npm install
npm run build
cd ..

# 编译并运行
go build -o ai-gateway.exe .
.\ai-gateway.exe
```

### 方式三：直接运行（开发模式）

```bash
# 先构建前端
cd frontend && npm install && npm run build && cd ..

# 启动
go run .
```

---

## ⚙️ 配置说明

首次启动会自动生成默认配置，所有配置文件位于 `data/` 目录下：

| 文件 | 说明 |
|------|------|
| `data/config.json` | 网关核心配置（端口、上游地址、超时、恢复策略等） |
| `data/admin.json` | 管理员账号（bcrypt 加密） |
| `data/api_keys.json` | 客户端 API Key |
| `data/model_keys.json` | 模型节点配置（节点 URL、Key 列表、启用状态） |
| `data/prompts.json` | 恢复提示词模板 |
| `data/api_usage.jsonl` | API 调用记录（JSON Lines 格式） |

### 核心配置项

```json
{
  "listen_port": "3301",
  "nginx_upstream_url": "http://127.0.0.1:8080",  // 迁移兼容字段，新部署请在模型节点中配置 URL
  "client_timeout": 300,
  "upstream_timeout": 300,
  "sse_recovery_enable": true,
  "default_recovery_mode": "B",
  "max_retry_times": 5,
  "session_expire_minute": 30,
  "log_keep_days": 5,
  "buffer_mode": false
}
```

> **注意**：上游地址现在在 `data/model_keys.json` 中按节点独立配置，`nginx_upstream_url` 仅用于旧格式自动迁移时的默认值。

---

## 🔧 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `GIN_MODE` | Gin 运行模式（release/debug） | `debug` |
| `JWT_SECRET` | JWT 签名密钥 | `ai-gateway-secret-key-2024` |

---

## 📡 API 接口

### OpenAI 兼容接口

```bash
POST /v1/chat/completions
Content-Type: application/json
Authorization: Bearer <your-api-key>

{
  "model": "gpt-3.5-turbo",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": true
}
```

### 管理后台接口

所有管理接口需携带 JWT Token（通过登录获取）：

| 路径 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/api/check-admin` | GET | 检查管理员是否已注册 |
| `/admin/login` | POST | 管理员登录 |
| `/admin/register` | POST | 注册管理员 |
| `/admin/password` | POST | 修改管理员密码 |
| `/admin/config` | GET/PUT | 配置管理 |
| `/admin/prompts` | GET/POST | 提示词管理 |
| `/admin/prompts/:id` | PUT/DELETE | 提示词编辑/删除 |
| `/admin/logs` | GET/DELETE | 日志查看/清空 |
| `/admin/upstream-logs` | GET | 上游请求日志 |
| `/admin/apikeys` | GET/POST | API Key 管理 |
| `/admin/apikeys/:key` | DELETE | 删除指定 API Key |
| `/admin/apikeys/:key/toggle` | PUT | 启用/禁用 API Key |
| `/admin/apikeys/:key/usage` | GET | API Key 调用记录（分页） |
| `/admin/modelnodes` | GET/POST | 模型节点列表/创建 |
| `/admin/modelnodes/:id` | PUT/DELETE | 模型节点更新/删除 |
| `/admin/modelnodes/:id/toggle` | PUT | 启用/禁用模型节点 |
| `/admin/modelnodes/:id/keys` | POST | 添加节点 Key |
| `/admin/modelnodes/:id/keys/:keyId` | DELETE | 删除节点 Key |
| `/admin/modelnodes/:id/keys/:keyId/toggle` | PUT | 启用/禁用节点 Key |
| `/admin/stats` | GET | 系统统计 |
| `/admin/stats/active-ips` | GET | 活跃 IP 统计 |
| `/admin/stats/trend` | GET | 请求趋势数据 |
| `/admin/location/:ip` | GET | IP 归属地查询 |

---

## 🐳 Docker 部署详解

### 拉取预构建镜像（推荐）

```bash
# 从 GitHub Container Registry 拉取最新镜像
docker pull ghcr.io/tdn-001/ai-gateway:latest
```

### 本地构建镜像

```bash
docker build -t ai-gateway .
```

### 运行容器

```bash
docker run -d \
  --name ai-gateway \
  -p 3301:3301 \
  -v /path/to/data:/app/data \
  -e TZ=Asia/Shanghai \
  -e GIN_MODE=release \
  -e JWT_SECRET=your-secret-key \
  ai-gateway
```

### docker-compose.yml

```yaml
services:
  ai-gateway:
    build: .
    container_name: ai-gateway
    ports:
      - "3301:3301"
    volumes:
      - /home/obj/ai_gateway/data:/app/data
    environment:
      - TZ=Asia/Shanghai
      - GIN_MODE=release
      - JWT_SECRET=${JWT_SECRET:-ai-gateway-secret-key-2024}
    restart: unless-stopped
```

---

## 🗂️ 项目结构

```
ai-gateway/
├── internal/
│   ├── auth/
│   │   └── jwt.go            # JWT 认证
│   ├── config/
│   │   └── config.go         # 配置管理
│   ├── gateway/
│   │   ├── proxy.go          # 请求代理（多节点路由 + Key 轮询）
│   │   ├── sse.go            # SSE 流处理
│   │   ├── recovery.go       # 异常恢复（同 Key 3 次失败换 Key）
│   │   └── location.go       # IP 定位
│   ├── logger/
│   │   └── logger.go         # 日志
│   └── storage/
│       ├── file.go           # 文件存储 + 异步日志写入
│       ├── apikeys.go        # API Key 管理 + api_usage.jsonl
│       └── modelkeys.go      # 模型节点管理（多节点、节点多 Key）
├── frontend/
│   ├── src/
│   │   ├── views/            # 前端页面
│   │   ├── components/       # 前端组件
│   │   ├── assets/           # 静态资源
│   │   └── router/           # 路由
│   ├── public/               # 公共资源（logo、图标）
│   ├── package.json
│   └── vite.config.ts
├── data/                     # 配置数据目录（自动迁移旧格式）
├── embed.go                  # 前端资源嵌入
├── main.go                   # 主程序入口
├── go.mod
├── .gitignore
├── .dockerignore
├── Dockerfile
├── docker-compose.yml
└── README.md
```

---

## 🔒 安全建议

1. **生产环境务必修改 JWT 密钥**：通过环境变量 `JWT_SECRET` 设置
2. **设置 GIN_MODE=release**：关闭 debug 模式
3. **定期更换 API Key**：通过后台管理页面管理客户端 API Key
4. **保护上游模型 Key**：通过后台「模型节点」管理上游 API Key，启用/禁用控制
5. **配置日志保留天数**：根据需求调整 `log_keep_days`
6. **使用 HTTPS**：建议在前置 Nginx 上配置 SSL 证书

---

## 📄 许可证

本项目基于 MIT 许可证开源。

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！
