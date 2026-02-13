# FastClaw

FastClaw is a Kubernetes-native platform for managing and orchestrating OpenClaw (Claude Code) bot instances. It provides a RESTful API to create, deploy, and manage AI agent bots in a multi-tenant environment.

## Features

- **Bot Lifecycle Management**: Create, start, stop, restart, upgrade, and delete bot instances
- **Kubernetes Native**: Deploys each bot as an isolated Pod with its own Service
- **Multi-tenant Support**: App-level isolation with per-user bot ownership
- **Skills Management**: Dynamically add, update, and delete skills for running bots
- **IM Channel Integration**: Connect bots to Telegram, Slack, Discord, Teams, LINE, Feishu, and more
- **Device Pairing**: Approve and manage device access with auto-approval support
- **Model Providers**: Configure multiple AI model providers (Anthropic, OpenAI, MiniMax, etc.)
- **WebSocket Proxy**: Real-time communication with bot instances via HTTP and WebSocket
- **Subdomain Routing**: Access bots via `{bot-id}.{domain}` subdomain
- **Shared Storage**: NAS-backed persistent storage for bot data
- **RESTful API**: Clean API design with Echo framework

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   API Client    │────▶│  FastClaw API    │────▶│   Kubernetes    │
│   (Frontend)    │     │    Server        │     │    Cluster      │
└─────────────────┘     └────────┬────────┘     └────────┬────────┘
                                 │                       │
                    ┌────────────┼────────────┐          │
                    ▼            ▼            ▼          ▼
             ┌───────────┐ ┌─────────┐ ┌──────────┐ ┌──────────┐
             │ PostgreSQL│ │  Auth   │ │ Config   │ │ OpenClaw │
             │ Database  │ │Middleware│ │  Sync   │ │   Pods   │
             └───────────┘ └─────────┘ └──────────┘ └────┬─────┘
                                                         │
                                              ┌──────────┼──────────┐
                                              ▼          ▼          ▼
                                        ┌─────────┐┌─────────┐┌─────────┐
                                        │ Gateway ││   IM    ││ Device  │
                                        │   WS    ││Channels ││ Pairing │
                                        └─────────┘└─────────┘└─────────┘
```

## Requirements

- Go 1.24+
- Kubernetes cluster (1.28+)
- PostgreSQL 14+
- Shared storage (NFS/NAS) for PVC

## Quick Start

本指南以 macOS 本地开发环境为例，带你从零跑通完整流程：启动服务 → 创建 App → 创建 Bot → 启动 Bot → 访问 Bot。

### 前置条件

| 依赖 | 说明 |
|------|------|
| Go 1.24+ | 编译 FastClaw |
| PostgreSQL 14+ | 存储 App 和 Bot 数据 |
| K8s 集群 | Bot 运行环境，本地推荐 [OrbStack](https://orbstack.dev/) 或 Docker Desktop |
| kubectl | 确认 `~/.kube/config` 能连上集群 |

### Step 1: 准备 K8s 环境

FastClaw 本身不需要部署到 K8s 集群里，只需要能连上集群的 kubeconfig。

```bash
# 确认 K8s 集群可用
kubectl cluster-info

# 创建命名空间
kubectl create namespace fastclaw

# 创建共享存储 PVC（本地开发用 hostPath，生产环境用 NFS/NAS）
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: fastclaw-shared-data
  namespace: fastclaw
spec:
  accessModes: [ ReadWriteOnce ]
  resources:
    requests:
      storage: 10Gi
EOF
```

### Step 2: 准备数据库

```bash
# 创建数据库（如果还没有）
createdb fastclaw

# 或者用 Docker 跑一个 PostgreSQL
docker run -d --name fastclaw-pg \
  -e POSTGRES_DB=fastclaw \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:16
```

### Step 3: 编译和配置

```bash
git clone https://github.com/fastclaw-ai/fastclaw.git
cd fastclaw
go build -o fastclaw .
```

复制配置文件并根据实际环境修改：

```bash
cp config.example.toml config.toml
```

本地开发最小配置：

```toml
[server]
port = 18080

[api]
admin_token = "my-admin-token"           # 自定义管理员 token

[db]
host = "localhost"
port = 5432
user = "postgres"
password = "postgres"
database = "fastclaw"
sslmode = "disable"
timezone = "Asia/Shanghai"

[kubernetes]
kubeconfig = ""                           # 留空，自动读取 ~/.kube/config
namespace = "fastclaw"
local_dev = true                          # 本地开发设为 true，通过 ClusterIP 直连 Pod

[storage]
pvc_name = "fastclaw-shared-data"
base_path = "/fastclaw-data"

[domain]
api_domain = "fastclaw.ai"
bot_domain_suffix = "fastclaw.ai"
bot_domain_template = "https://{bot_id}.fastclaw.ai"

[openclaw]
image = "1panel/openclaw:latest"
gateway_port = 18789
cpu_limit = "2000m"
memory_limit = "4Gi"
cpu_request = "500m"
memory_request = "1Gi"
```

### Step 4: 启动服务

```bash
./fastclaw server
```

验证服务是否正常：

```bash
curl http://localhost:18080/health
# {"status":"ok"}
```

### Step 5: 创建 App（获取 API Token）

App 是多租户的顶层概念，每个 App 拥有独立的 API Token，用于调用所有 Bot 相关接口。

```bash
curl -X POST http://localhost:18080/bot/api/v1/admin/apps \
  -H "Authorization: Bearer my-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app"}'
```

返回示例：

```json
{
  "code": 0,
  "data": {
    "id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "name": "my-app",
    "api_token": "a1b2c3d4e5f6...",
    "status": "active",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

记下 `api_token`，后续所有请求都用它做认证。

### Step 6: 创建 Bot

```bash
export API_TOKEN="a1b2c3d4e5f6..."  # 替换为上一步拿到的 api_token

curl -X POST http://localhost:18080/bot/api/v1/bots \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "name": "my-first-bot",
    "slug": "my-bot",
    "config": {
      "model": "claude-sonnet-4-20250514",
      "api_key": "sk-ant-xxx"
    }
  }'
```

返回示例：

```json
{
  "code": 0,
  "data": {
    "id": "bot-uuid-xxxx",
    "name": "my-first-bot",
    "slug": "my-bot",
    "status": "created",
    "access_token": "xxxxxxxxxx",
    "access_url": "https://my-bot.fastclaw.ai?token=xxxxxxxxxx"
  }
}
```

### Step 7: 启动 Bot

```bash
export BOT_ID="bot-uuid-xxxx"  # 替换为上一步返回的 bot id

curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/start \
  -H "Authorization: Bearer $API_TOKEN"
```

FastClaw 会在 K8s 集群中为 Bot 创建一个 Deployment 和 Service。

### Step 8: 查看状态和连接

```bash
# 查看 Bot 状态
curl http://localhost:18080/bot/api/v1/bots/$BOT_ID/status \
  -H "Authorization: Bearer $API_TOKEN"

# 获取连接信息（endpoint、WebSocket URL 等）
curl http://localhost:18080/bot/api/v1/bots/$BOT_ID/connect \
  -H "Authorization: Bearer $API_TOKEN"
```

也可以通过 proxy 直接访问 Bot：

```bash
# HTTP proxy
curl http://localhost:18080/proxy/$BOT_ID/

# 或通过子域名访问（需配置 DNS）
# curl https://my-bot.fastclaw.ai/
```

### 后续操作

```bash
# 停止 Bot
curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/stop \
  -H "Authorization: Bearer $API_TOKEN"

# 重启 Bot
curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/restart \
  -H "Authorization: Bearer $API_TOKEN"

# 删除 Bot（会同时清理 K8s 资源）
curl -X DELETE http://localhost:18080/bot/api/v1/bots/$BOT_ID \
  -H "Authorization: Bearer $API_TOKEN"
```

## API Reference

All API routes are prefixed with `/bot/api/v1`. Requests require a Bearer token (`Authorization: Bearer <token>`) unless noted otherwise.

### Health Check

```
GET /health
```

### App Management (Admin)

Requires admin token authentication.

| Method | Endpoint                                 | Description         |
| ------ | ---------------------------------------- | ------------------- |
| POST   | `/bot/api/v1/admin/apps`                 | Create app          |
| GET    | `/bot/api/v1/admin/apps`                 | List apps           |
| GET    | `/bot/api/v1/admin/apps/:id`             | Get app details     |
| PUT    | `/bot/api/v1/admin/apps/:id`             | Update app          |
| DELETE | `/bot/api/v1/admin/apps/:id`             | Delete app          |
| POST   | `/bot/api/v1/admin/apps/:id/reset-token` | Reset app API token |

### Bot Management

| Method | Endpoint                       | Description       |
| ------ | ------------------------------ | ----------------- |
| POST   | `/bot/api/v1/bots`             | Create a new bot  |
| GET    | `/bot/api/v1/bots?user_id=xxx` | List bots by user |
| GET    | `/bot/api/v1/bots/:id`         | Get bot details   |
| PUT    | `/bot/api/v1/bots/:id`         | Update bot        |
| DELETE | `/bot/api/v1/bots/:id`         | Delete bot        |

### Bot Lifecycle

| Method | Endpoint                           | Description         |
| ------ | ---------------------------------- | ------------------- |
| POST   | `/bot/api/v1/bots/:id/start`       | Start bot           |
| POST   | `/bot/api/v1/bots/:id/stop`        | Stop bot            |
| POST   | `/bot/api/v1/bots/:id/restart`     | Restart bot         |
| GET    | `/bot/api/v1/bots/:id/status`      | Get bot status      |
| GET    | `/bot/api/v1/bots/:id/connect`     | Get connection info |
| POST   | `/bot/api/v1/bots/:id/reset-token` | Reset bot token     |

### Bot Upgrade (Admin)

| Method | Endpoint                             | Description          |
| ------ | ------------------------------------ | -------------------- |
| POST   | `/bot/api/v1/admin/bots/upgrade`     | Upgrade all bots     |
| POST   | `/bot/api/v1/admin/bots/:id/upgrade` | Upgrade specific bot |

### Skills Management

| Method | Endpoint                            | Description         |
| ------ | ----------------------------------- | ------------------- |
| GET    | `/bot/api/v1/bots/:id/skills`       | List skills         |
| PUT    | `/bot/api/v1/bots/:id/skills/:name` | Update/create skill |
| DELETE | `/bot/api/v1/bots/:id/skills/:name` | Delete skill        |

### IM Channels

| Method | Endpoint                                                 | Description           |
| ------ | -------------------------------------------------------- | --------------------- |
| POST   | `/bot/api/v1/bots/:id/channels`                          | Add channel           |
| GET    | `/bot/api/v1/bots/:id/channels`                          | List channels         |
| DELETE | `/bot/api/v1/bots/:id/channels/:channel`                 | Remove channel        |
| GET    | `/bot/api/v1/bots/:id/channels/:channel/pairing`         | List pairing requests |
| POST   | `/bot/api/v1/bots/:id/channels/:channel/pairing/approve` | Approve pairing       |
| POST   | `/bot/api/v1/bots/:id/channels/:channel/pairing/revoke`  | Revoke pairing        |
| GET    | `/bot/api/v1/bots/:id/channels/:channel/pairing/users`   | Get paired users      |

### Device Pairing

| Method | Endpoint                                           | Description    |
| ------ | -------------------------------------------------- | -------------- |
| GET    | `/bot/api/v1/bots/:id/devices`                     | List devices   |
| POST   | `/bot/api/v1/bots/:id/devices/:request_id/approve` | Approve device |
| DELETE | `/bot/api/v1/bots/:id/devices/:device_id`          | Revoke device  |

### Model Providers

| Method | Endpoint                                       | Description     |
| ------ | ---------------------------------------------- | --------------- |
| GET    | `/bot/api/v1/bots/:id/config/models`           | List providers  |
| POST   | `/bot/api/v1/bots/:id/config/models`           | Add provider    |
| GET    | `/bot/api/v1/bots/:id/config/models/:provider` | Get provider    |
| PUT    | `/bot/api/v1/bots/:id/config/models/:provider` | Update provider |
| DELETE | `/bot/api/v1/bots/:id/config/models/:provider` | Delete provider |

### Agent Defaults

| Method | Endpoint                               | Description        |
| ------ | -------------------------------------- | ------------------ |
| GET    | `/bot/api/v1/bots/:id/config/defaults` | Get agent defaults |
| PUT    | `/bot/api/v1/bots/:id/config/defaults` | Set agent defaults |

### Proxy

| Method | Endpoint           | Description           |
| ------ | ------------------ | --------------------- |
| ANY    | `/proxy/:bot_id/*` | Proxy requests to bot |
| WS     | `/proxy/:bot_id/*` | WebSocket proxy       |

Subdomain routing is also supported: `{bot-id}.{bot_domain_suffix}/*` automatically routes to the corresponding bot.

### Example: Create and Start a Bot

```bash
# Create a bot
curl -X POST http://localhost:18080/bot/api/v1/bots \
  -H "Authorization: Bearer <your-app-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "name": "my-bot",
    "config": {
      "model": "claude-sonnet-4-20250514",
      "api_key": "sk-xxx"
    }
  }'

# Start the bot
curl -X POST http://localhost:18080/bot/api/v1/bots/{bot_id}/start \
  -H "Authorization: Bearer <your-app-token>"

# Get bot status
curl http://localhost:18080/bot/api/v1/bots/{bot_id}/status \
  -H "Authorization: Bearer <your-app-token>"
```

## Project Structure

```
fastclaw/
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go            # Root command & config init
│   └── server.go          # Server command & route setup
├── handler/               # HTTP handlers
│   ├── api/v1/           # API v1 handlers
│   │   ├── app.go        # App management (admin)
│   │   ├── bot_create.go # Create bot
│   │   ├── bot_list.go   # List bots
│   │   ├── bot_get.go    # Get bot details
│   │   ├── bot_update.go # Update bot
│   │   ├── bot_delete.go # Delete bot
│   │   ├── bot_start.go  # Start bot
│   │   ├── bot_stop.go   # Stop bot
│   │   ├── bot_restart.go# Restart bot
│   │   ├── bot_status.go # Bot status
│   │   ├── bot_connect.go# Connection info
│   │   ├── bot_reset_token.go # Reset token
│   │   ├── bot_upgrade.go# Bot upgrade
│   │   ├── bot_channel.go# IM channels
│   │   ├── bot_devices.go# Device pairing
│   │   ├── bot_config_models.go   # Model providers
│   │   ├── bot_config_defaults.go # Agent defaults
│   │   ├── config_utils.go        # Config helpers
│   │   ├── skill_list.go  # List skills
│   │   ├── skill_update.go# Update skill
│   │   └── skill_delete.go# Delete skill
│   └── proxy/            # Proxy handlers
│       └── proxy.go      # HTTP/WebSocket proxy
├── middleware/            # Middleware
│   └── auth.go           # Bearer auth, bot owner auth, admin auth
├── model/                # Data models (GORM)
│   ├── app.go           # App model
│   └── bot.go           # Bot model
├── service/             # Business logic
│   └── k8s/             # Kubernetes service
│       ├── client.go     # K8s client init
│       ├── deployment.go # Deployment management
│       ├── service.go    # Service management
│       ├── exec.go       # Pod exec
│       ├── botconfig.go  # Bot config writing
│       ├── config_sync.go# Config synchronization
│       ├── approve.go    # Device auto-approve
│       ├── channel.go    # IM channel operations
│       └── gateway.go    # Gateway WebSocket
├── util/                # Utilities
│   ├── config.go       # Config management (Viper)
│   ├── db.go           # Database connection (GORM)
│   └── response.go     # HTTP response helpers
├── config.example.toml  # Configuration template
├── Dockerfile           # Docker build
└── main.go             # Entry point
```

## Deployment

### Docker

```bash
# Build Docker image
docker build -t fastclaw:latest .

# Run
docker run -p 18080:18080 -v ./config.toml:/app/config.toml fastclaw:latest
```

### Kubernetes

See [deploy/](deploy/) directory for Kubernetes deployment examples.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
