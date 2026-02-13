# FastClaw

Kubernetes-native platform for managing and orchestrating [OpenClaw](https://github.com/1panel-dev/openclaw) (Claude Code) bot instances. Provides a RESTful API to create, deploy, and manage AI agent bots in a multi-tenant environment.

## Features

- **Bot Lifecycle** - Create, start, stop, restart, upgrade, and delete bot instances
- **Kubernetes Native** - Each bot runs as an isolated Pod with its own Service
- **Multi-tenant** - App-level isolation with per-user bot ownership
- **Skills** - Dynamically manage skills for running bots
- **IM Channels** - Connect bots to Telegram, Slack, Discord, Teams, LINE, Feishu, etc.
- **Device Pairing** - Approve and manage device access with auto-approval
- **Model Providers** - Configure multiple AI providers (Anthropic, OpenAI, MiniMax, etc.)
- **Proxy** - HTTP and WebSocket proxy to bot instances, with subdomain routing

## Architecture

```
┌──────────┐     ┌──────────┐     ┌──────────┐
│  Client  │────▶│ FastClaw │────▶│   K8s    │
└──────────┘     └─────┬────┘     └────┬─────┘
                       │               │
                 ┌─────┴─────┐    ┌────┴─────┐
                 │ PostgreSQL│    │ OpenClaw  │
                 └───────────┘    │   Pods   │
                                  └────┬─────┘
                            ┌──────────┼──────────┐
                            │          │          │
                       Gateway    IM Channels  Devices
```

## Prerequisites

- Go 1.24+
- PostgreSQL 14+
- Kubernetes cluster (1.28+) - locally via [OrbStack](https://orbstack.dev/) or Docker Desktop
- `kubectl` configured with a valid kubeconfig

> FastClaw does **not** need to run inside the K8s cluster. It only needs a kubeconfig that can reach the cluster API.

## Quick Start

### 1. Prepare K8s

```bash
kubectl create namespace fastclaw

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

### 2. Prepare PostgreSQL

```bash
docker run -d --name fastclaw-pg \
  -e POSTGRES_DB=fastclaw \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:16
```

### 3. Build and configure

```bash
git clone https://github.com/fastclaw-ai/fastclaw.git
cd fastclaw
go build -o fastclaw .
cp config.example.toml config.toml
```

Minimal local dev config:

```toml
[server]
port = 18080

[api]
admin_token = "my-admin-token"

[db]
host = "localhost"
port = 5432
user = "postgres"
password = "postgres"
database = "fastclaw"
sslmode = "disable"
timezone = "Asia/Shanghai"

[kubernetes]
kubeconfig = ""        # leave empty to use ~/.kube/config
namespace = "fastclaw"
local_dev = true       # use ClusterIP for direct pod access from host

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

### 4. Start the server

```bash
./fastclaw server

# Verify
curl http://localhost:18080/health
```

### 5. Create an App (get API token)

Each App has its own API token used to authenticate all bot operations.

```bash
curl -X POST http://localhost:18080/bot/api/v1/admin/apps \
  -H "Authorization: Bearer my-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app"}'
```

Save the `api_token` from the response.

### 6. Create and start a Bot

```bash
export API_TOKEN="<api_token from step 5>"

# Create
curl -X POST http://localhost:18080/bot/api/v1/bots \
  -H "Authorization: Bearer $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-001",
    "name": "my-bot",
    "slug": "my-bot",
    "config": {
      "model": "claude-sonnet-4-20250514",
      "api_key": "sk-ant-xxx"
    }
  }'

export BOT_ID="<id from response>"

# Start (creates K8s Deployment + Service)
curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/start \
  -H "Authorization: Bearer $API_TOKEN"

# Check status
curl http://localhost:18080/bot/api/v1/bots/$BOT_ID/status \
  -H "Authorization: Bearer $API_TOKEN"

# Get connection info
curl http://localhost:18080/bot/api/v1/bots/$BOT_ID/connect \
  -H "Authorization: Bearer $API_TOKEN"

# Access via proxy
curl http://localhost:18080/proxy/$BOT_ID/
```

### 7. Manage the Bot

```bash
# Stop
curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/stop \
  -H "Authorization: Bearer $API_TOKEN"

# Restart
curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/restart \
  -H "Authorization: Bearer $API_TOKEN"

# Delete (also cleans up K8s resources)
curl -X DELETE http://localhost:18080/bot/api/v1/bots/$BOT_ID \
  -H "Authorization: Bearer $API_TOKEN"
```

## API Reference

All routes are prefixed with `/bot/api/v1` and require `Authorization: Bearer <token>`.

### Health Check

```
GET /health
```

### Admin - Apps

| Method | Endpoint                                 | Description         |
| ------ | ---------------------------------------- | ------------------- |
| POST   | `/bot/api/v1/admin/apps`                 | Create app          |
| GET    | `/bot/api/v1/admin/apps`                 | List apps           |
| GET    | `/bot/api/v1/admin/apps/:id`             | Get app             |
| PUT    | `/bot/api/v1/admin/apps/:id`             | Update app          |
| DELETE | `/bot/api/v1/admin/apps/:id`             | Delete app          |
| POST   | `/bot/api/v1/admin/apps/:id/reset-token` | Reset app API token |

### Admin - Bot Upgrade

| Method | Endpoint                             | Description          |
| ------ | ------------------------------------ | -------------------- |
| POST   | `/bot/api/v1/admin/bots/upgrade`     | Upgrade all bots     |
| POST   | `/bot/api/v1/admin/bots/:id/upgrade` | Upgrade specific bot |

### Bots

| Method | Endpoint                           | Description         |
| ------ | ---------------------------------- | ------------------- |
| POST   | `/bot/api/v1/bots`                 | Create bot          |
| GET    | `/bot/api/v1/bots?user_id=xxx`     | List bots           |
| GET    | `/bot/api/v1/bots/:id`             | Get bot             |
| PUT    | `/bot/api/v1/bots/:id`             | Update bot          |
| DELETE | `/bot/api/v1/bots/:id`             | Delete bot          |
| POST   | `/bot/api/v1/bots/:id/start`       | Start bot           |
| POST   | `/bot/api/v1/bots/:id/stop`        | Stop bot            |
| POST   | `/bot/api/v1/bots/:id/restart`     | Restart bot         |
| GET    | `/bot/api/v1/bots/:id/status`      | Get bot status      |
| GET    | `/bot/api/v1/bots/:id/connect`     | Get connection info |
| POST   | `/bot/api/v1/bots/:id/reset-token` | Reset bot token     |

### Skills

| Method | Endpoint                            | Description   |
| ------ | ----------------------------------- | ------------- |
| GET    | `/bot/api/v1/bots/:id/skills`       | List skills   |
| PUT    | `/bot/api/v1/bots/:id/skills/:name` | Upsert skill  |
| DELETE | `/bot/api/v1/bots/:id/skills/:name` | Delete skill  |

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

### Devices

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

Subdomain routing: `{bot-id}.{bot_domain_suffix}/*` routes to the corresponding bot automatically.

## Deployment

### Docker

```bash
docker build -t fastclaw:latest .
docker run -p 18080:18080 -v ./config.toml:/app/config.toml fastclaw:latest
```

### Kubernetes

See [deploy/](deploy/) directory for examples.

## License

Apache License 2.0 - see [LICENSE](LICENSE).
