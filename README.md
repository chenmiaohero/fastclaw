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
┌──────────┐       ┌──────────────────────────┐       ┌──────────────────────┐
│  Client  │──────▶│        FastClaw          │──────▶│    K8s Cluster       │
│          │       │                          │       │                      │
│ Browser  │       │  ┌─────────┐ ┌────────┐  │       │  ┌────────────────┐  │
│ CLI      │       │  │ Bot API │ │Admin   │  │       │  │  OpenClaw Pod  │  │
│ API      │       │  │ (CRUD,  │ │API     │  │  K8s  │  │  ┌──────────┐ │  │
│          │       │  │ Skills, │ │(Apps,  │  │  API  │  │  │ Gateway  │ │  │
└──────────┘       │  │ Channel,│ │Upgrade)│  │◀─────▶│  │  │  :18789  │ │  │
     │             │  │ Devices,│ └────────┘  │       │  │  ├──────────┤ │  │
     │  Subdomain  │  │ Models) │             │       │  │  │ Channels │ │  │
     │  Routing    │  └─────────┘ ┌────────┐  │       │  │  │ TG/Slack │ │  │
     │             │              │ Proxy  │  │ HTTP/ │  │  │ Discord..│ │  │
     └────────────▶│              │ HTTP & │──┼─WS───▶│  │  └──────────┘ │  │
  {slug}.domain/*  │              │ WS     │  │       │  └────────────────┘  │
                   │              └────────┘  │       │  ┌────────────────┐  │
                   │                     │    │       │  │  Shared PVC    │  │
                   └─────────────────────┼────┘       │  │  /data/{botID} │  │
                                         │            │  └────────────────┘  │
                                   ┌─────┴─────┐      └──────────────────────┘
                                   │PostgreSQL │
                                   │ apps      │
                                   │ bots      │
                                   │ (config   │
                                   │  JSONB)   │
                                   └───────────┘
```

Each bot runs as an isolated K8s Pod (Deployment + ClusterIP Service). FastClaw manages the full lifecycle and proxies all traffic — subdomain requests are rewritten to `/proxy/{slug}/*` internally, no per-bot Ingress needed. Bot config is stored as JSONB in PostgreSQL and synced bidirectionally with the pod's `openclaw.json`.

## Prerequisites

- Go 1.24+ (for building from source)
- Kubernetes cluster (1.28+) - locally via [OrbStack](https://orbstack.dev/) or Docker Desktop
- `kubectl` and optionally `helm` (v3)

> FastClaw does **not** need to run inside the K8s cluster. It only needs a kubeconfig that can reach the cluster API.

## Quick Start

### Option A: Helm Install (recommended)

One command to deploy everything (FastClaw + PostgreSQL + RBAC) into your K8s cluster:

```bash
helm install fastclaw deploy/helm/fastclaw \
  -n fastclaw --create-namespace \
  --set adminToken="my-admin-token"
```

Verify:

```bash
kubectl -n fastclaw get pods
kubectl -n fastclaw port-forward svc/fastclaw 18080:18080
curl http://localhost:18080/health
```

See [Helm values](#helm-chart) for full configuration options.

### Option B: kubectl Apply

```bash
# Create namespace and RBAC
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/rbac.yaml
kubectl apply -f deploy/k8s/pvc.yaml

# Create secrets (edit first!)
cp deploy/k8s/secrets.yaml.example deploy/k8s/secrets.yaml
# edit deploy/k8s/secrets.yaml with your tokens/passwords
kubectl apply -f deploy/k8s/secrets.yaml

# Deploy PostgreSQL and FastClaw
kubectl apply -f deploy/k8s/postgres.yaml
kubectl apply -f deploy/k8s/configmap.yaml
kubectl apply -f deploy/k8s/deployment.yaml

# Port-forward to access locally
kubectl -n fastclaw port-forward svc/fastclaw 18080:18080
```

### Option C: Local Binary

Run FastClaw on your host, connecting to a K8s cluster via kubeconfig.

```bash
git clone https://github.com/fastclaw-ai/fastclaw.git
cd fastclaw
go build -o fastclaw .
cp config.example.toml config.toml
```

Start a PostgreSQL instance:

```bash
docker run -d --name fastclaw-pg \
  -e POSTGRES_DB=fastclaw \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 postgres:16
```

Prepare K8s namespace and storage:

```bash
kubectl create namespace fastclaw
kubectl apply -f deploy/k8s/pvc.yaml
```

Edit `config.toml` - key settings for local dev:

```toml
[kubernetes]
local_dev = true    # use ClusterIP for direct pod access from host

[api]
admin_token = "my-admin-token"
```

Run:

```bash
./fastclaw server
curl http://localhost:18080/health
```

### Create Your First Bot

Once the server is running (via any option above):

```bash
# 1. Create an App (each app gets its own API token)
curl -s -X POST http://localhost:18080/bot/api/v1/admin/apps \
  -H "Authorization: Bearer my-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-app"}'
# Save the api_token from the response

export API_TOKEN="<api_token>"

# 2. Create a Bot
curl -s -X POST http://localhost:18080/bot/api/v1/bots \
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

export BOT_ID="<id>"

# 3. Start the Bot (creates K8s Deployment + Service)
curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/start \
  -H "Authorization: Bearer $API_TOKEN"

# 4. Check status
curl http://localhost:18080/bot/api/v1/bots/$BOT_ID/status \
  -H "Authorization: Bearer $API_TOKEN"

# 5. Access via proxy
curl http://localhost:18080/proxy/$BOT_ID/

# Stop / Restart / Delete
curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/stop -H "Authorization: Bearer $API_TOKEN"
curl -X POST http://localhost:18080/bot/api/v1/bots/$BOT_ID/restart -H "Authorization: Bearer $API_TOKEN"
curl -X DELETE http://localhost:18080/bot/api/v1/bots/$BOT_ID -H "Authorization: Bearer $API_TOKEN"
```

## Deployment

### Helm Chart

```bash
helm install fastclaw deploy/helm/fastclaw \
  -n fastclaw --create-namespace \
  --set adminToken="my-secret-token"
```

Key values (`deploy/helm/fastclaw/values.yaml`):

| Parameter | Default | Description |
|-----------|---------|-------------|
| `adminToken` | `change-me` | Admin API token |
| `server.image.repository` | `fastclaw` | FastClaw image |
| `server.image.tag` | `latest` | Image tag |
| `server.replicas` | `1` | Number of replicas |
| `postgresql.enabled` | `true` | Deploy built-in PostgreSQL |
| `postgresql.auth.password` | `postgres` | DB password |
| `externalDatabase.host` | `""` | External DB host (when `postgresql.enabled=false`) |
| `storage.size` | `10Gi` | Shared PVC size for bot data |
| `openclaw.image` | `1panel/openclaw:latest` | OpenClaw bot image |
| `openclaw.cpuLimit` | `2000m` | Bot CPU limit |
| `openclaw.memoryLimit` | `4Gi` | Bot memory limit |
| `domain.botDomainSuffix` | `fastclaw.ai` | Bot subdomain suffix |
| `ingress.enabled` | `false` | Enable ingress |

Use an external database:

```bash
helm install fastclaw deploy/helm/fastclaw \
  -n fastclaw --create-namespace \
  --set adminToken="my-token" \
  --set postgresql.enabled=false \
  --set externalDatabase.host="db.example.com" \
  --set externalDatabase.password="secret"
```

Upgrade:

```bash
helm upgrade fastclaw deploy/helm/fastclaw -n fastclaw
```

Uninstall:

```bash
helm uninstall fastclaw -n fastclaw
```

### Raw K8s Manifests

All manifests are in `deploy/k8s/`:

| File | Description |
|------|-------------|
| `namespace.yaml` | Namespace |
| `rbac.yaml` | ServiceAccount, Role, RoleBinding |
| `pvc.yaml` | Shared storage for bot data |
| `postgres.yaml` | PostgreSQL Deployment + Service |
| `secrets.yaml.example` | Secret template (copy and edit) |
| `configmap.yaml` | FastClaw config.toml |
| `deployment.yaml` | FastClaw Deployment + Service |

### Docker

```bash
docker build -t fastclaw:latest .
docker run -p 18080:18080 -v ./config.toml:/app/config.toml fastclaw:latest
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

## License

Apache License 2.0 - see [LICENSE](LICENSE).
