<p align="center">
  <img src="frontend/public/logo.png" alt="AI Gateway Logo" width="120" />
</p>

<h1 align="center">AI Gateway</h1>

<p align="center">
  <strong>兼容 OpenAI API 协议的智能模型网关</strong>
  <br />
  统一入口 · 异常恢复 · 流式缓冲 · Web 管理后台
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
用户客户端 → AI Gateway → Nginx → FreeLLM 模型节点池
```

> AI Gateway 只负责 AI 请求处理、SSE 流处理、异常恢复和后台管理。Nginx 负责负载均衡与节点切换，不属于本项目范围。

---

## ✨ 功能特性

- **OpenAI 兼容** — 完全兼容 `POST /v1/chat/completions`，支持 `stream=true/false`
- **SSE 流式处理** — 实时读取并转发 SSE 流，缓存已输出内容
- **异常自动恢复** — 支持 429、500、502、503、504 等异常，自动重试与恢复
- **两种恢复模式** — 模式 A（接续生成）/ 模式 B（完整重生成），支持自定义提示词模板
- **流式缓冲模式** — 可选择将上游内容完全缓存后一次性输出，失败时客户端不会看到部分内容
- **Web 管理后台** — 配置管理、提示词管理、API Key 管理、日志查看、统计图表
- **API Key 认证** — 支持多 Key 管理、启用/禁用、调用记录追踪
- **模型 Key 轮询** — 多个模型 Key 自动轮询，避免单 Key 限流
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
| `data/model_keys.json` | 上游模型节点 Key |
| `data/prompts.json` | 恢复提示词模板 |
| `data/api_usage.json` | API 调用记录 |

### 核心配置项

```json
{
  "listen_port": "3301",
  "nginx_upstream_url": "http://127.0.0.1:8080",
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
| `/admin/login` | POST | 管理员登录 |
| `/admin/register` | POST | 注册管理员 |
| `/admin/config` | GET/PUT | 配置管理 |
| `/admin/prompts` | GET/POST | 提示词管理 |
| `/admin/prompts/:id` | PUT/DELETE | 提示词编辑/删除 |
| `/admin/logs` | GET/DELETE | 日志查看/清空 |
| `/admin/apikeys` | GET/POST | API Key 管理 |
| `/admin/modelkeys` | GET/POST | 模型 Key 管理 |
| `/admin/stats` | GET | 系统统计 |

---

## 🐳 Docker 部署详解

### 构建镜像

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
├── cmd/
│   └── main.go              # 入口文件
├── internal/
│   ├── auth/
│   │   └── jwt.go            # JWT 认证
│   ├── config/
│   │   └── config.go         # 配置管理
│   ├── gateway/
│   │   ├── proxy.go          # 请求代理
│   │   ├── sse.go            # SSE 流处理
│   │   ├── recovery.go       # 异常恢复
│   │   └── location.go       # IP 定位
│   ├── logger/
│   │   └── logger.go         # 日志
│   └── storage/
│       ├── file.go           # 文件存储
│       ├── apikeys.go        # API Key 管理
│       └── modelkeys.go      # 模型 Key 管理
├── frontend/
│   ├── src/
│   │   ├── views/            # 前端页面
│   │   └── router/           # 路由
│   └── package.json
├── data/                     # 配置数据目录
├── embed.go                  # 前端资源嵌入
├── main.go                   # 主程序
├── Dockerfile
├── docker-compose.yml
└── README.md
```

---

## 🔒 安全建议

1. **生产环境务必修改 JWT 密钥**：通过环境变量 `JWT_SECRET` 设置
2. **设置 GIN_MODE=release**：关闭 debug 模式
3. **定期更换 API Key**：通过后台管理页面管理
4. **配置日志保留天数**：根据需求调整 `log_keep_days`
5. **使用 HTTPS**：建议在前置 Nginx 上配置 SSL 证书

---

## 📄 许可证

本项目基于 MIT 许可证开源。

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！