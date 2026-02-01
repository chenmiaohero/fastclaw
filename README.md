# Clawork

Clawork is a Kubernetes-native platform for managing and orchestrating OpenClaw (Claude Code) bot instances. It provides a RESTful API to create, deploy, and manage AI agent bots in a multi-tenant environment.

## Features

- **Bot Lifecycle Management**: Create, start, stop, restart, and delete bot instances
- **Kubernetes Native**: Deploys each bot as an isolated Pod with its own Service
- **Multi-tenant Support**: Manage multiple bots per user with namespace isolation
- **Skills Management**: Dynamically add, update, and delete skills for running bots
- **WebSocket Proxy**: Real-time communication with bot instances
- **Shared Storage**: NAS-backed persistent storage for bot data
- **RESTful API**: Clean API design with Echo framework

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   API Client    │────▶│  Clawork API    │────▶│   Kubernetes    │
│   (Frontend)    │     │    Server       │     │    Cluster      │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                               │                        │
                               ▼                        ▼
                        ┌─────────────┐         ┌─────────────┐
                        │  PostgreSQL │         │  OpenClaw   │
                        │   Database  │         │    Pods     │
                        └─────────────┘         └─────────────┘
```

## Requirements

- Go 1.24+
- Kubernetes cluster (1.28+)
- PostgreSQL 14+
- Shared storage (NFS/NAS) for PVC

## Quick Start

### Build from source

```bash
git clone https://github.com/workany-ai/clawork.git
cd clawork
go build -o clawork .
```

### Configuration

Copy the example config and modify it:

```bash
cp config.example.toml config.toml
```

Edit `config.toml`:

```toml
[server]
port = 8080

[db]
host = "localhost"
port = 5432
user = "aigc"
password = "your-password"
database = "clawork"

[kubernetes]
kubeconfig = ""  # Empty for in-cluster config
namespace = "clawork"

[storage]
pvc_name = "clawork-shared-data"
base_path = "/openclaw-data"

[openclaw]
image = "1panel/openclaw:latest"
gateway_port = 18789
cpu_limit = "500m"
memory_limit = "512Mi"
```

### Run

```bash
./clawork server
```

## API Reference

### Health Check

```
GET /health
```

### Bot Management

| Method | Endpoint                   | Description       |
| ------ | -------------------------- | ----------------- |
| POST   | `/api/v1/bots`             | Create a new bot  |
| GET    | `/api/v1/bots?user_id=xxx` | List bots by user |
| GET    | `/api/v1/bots/:id`         | Get bot details   |
| PUT    | `/api/v1/bots/:id`         | Update bot        |
| DELETE | `/api/v1/bots/:id`         | Delete bot        |

### Bot Lifecycle

| Method | Endpoint                   | Description         |
| ------ | -------------------------- | ------------------- |
| POST   | `/api/v1/bots/:id/start`   | Start bot           |
| POST   | `/api/v1/bots/:id/stop`    | Stop bot            |
| POST   | `/api/v1/bots/:id/restart` | Restart bot         |
| GET    | `/api/v1/bots/:id/status`  | Get bot status      |
| GET    | `/api/v1/bots/:id/connect` | Get connection info |

### Skills Management

| Method | Endpoint                        | Description         |
| ------ | ------------------------------- | ------------------- |
| GET    | `/api/v1/bots/:id/skills`       | List skills         |
| PUT    | `/api/v1/bots/:id/skills/:name` | Update/create skill |
| DELETE | `/api/v1/bots/:id/skills/:name` | Delete skill        |

### Proxy

| Method | Endpoint           | Description           |
| ------ | ------------------ | --------------------- |
| ANY    | `/proxy/:bot_id/*` | Proxy requests to bot |
| WS     | `/proxy/:bot_id/*` | WebSocket proxy       |

### Example: Create and Start a Bot

```bash
# Create a bot
curl -X POST http://localhost:8080/api/v1/bots \
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
curl -X POST http://localhost:8080/api/v1/bots/{bot_id}/start

# Get bot status
curl http://localhost:8080/api/v1/bots/{bot_id}/status
```

## Project Structure

```
clawork/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command
│   └── server.go          # Server command
├── handler/               # HTTP handlers
│   ├── api/v1/           # API v1 handlers
│   │   ├── bot_*.go      # Bot endpoints
│   │   └── skill_*.go    # Skill endpoints
│   └── proxy/            # Proxy handlers
│       └── proxy.go      # WebSocket/HTTP proxy
├── model/                 # Data models
│   └── bot.go            # Bot model
├── service/              # Business logic
│   └── k8s/              # Kubernetes service
│       ├── client.go     # K8s client init
│       ├── deployment.go # Deployment management
│       ├── service.go    # Service management
│       ├── exec.go       # Pod exec
│       ├── botconfig.go  # Bot config writing
│       ├── approve.go    # Device auto-approve
│       └── channel.go    # WebSocket channel
├── util/                 # Utilities
│   ├── config.go        # Config management
│   ├── db.go            # Database connection
│   └── response.go      # HTTP response helper
├── deploy/              # Deployment files
├── config.toml          # Configuration
└── main.go              # Entry point
```

## Deployment

See [deploy/](deploy/) directory for Kubernetes deployment examples.

```bash
# Build Docker image
./deploy/build.sh

# Deploy to Kubernetes
./deploy/deploy.sh
```

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
