# Clawork 线上部署指南

## 架构概览

```
                                 ┌─────────────────────────────────────────────────────┐
                                 │                 Kubernetes Cluster                  │
                                 │                                                     │
    用户浏览器                    │   ┌──────────────┐      ┌─────────────────────┐    │
        │                        │   │   workany    │      │      clawork        │    │
        │                        │   │  (前端服务)   │─────►│   (Bot管理API)      │    │
        │                        │   └──────────────┘      └──────────┬──────────┘    │
        │                        │          ▲                         │               │
        │                        │          │                         │ 管理          │
        ├── workany.ai ──────────┼──────────┘                         ▼               │
        │                        │                      ┌─────────────────────────┐   │
        │                        │                      │    OpenClaw Pods        │   │
        │                        │                      │  ┌─────┐ ┌─────┐        │   │
        └── xxx.workany.bot ─────┼──► Ingress ─────────►│  │bot-1│ │bot-2│ ...    │   │
                                 │        │             │  └─────┘ └─────┘        │   │
                                 │        │             └─────────────────────────┘   │
                                 │        │                         ▲                 │
                                 │        └─────────────────────────┘                 │
                                 │              (通过 clawork 代理)                    │
                                 │                                                     │
                                 │   ┌──────────────┐      ┌─────────────────────┐    │
                                 │   │  PostgreSQL  │      │   Shared PVC        │    │
                                 │   │   (数据库)    │      │  (Bot数据存储)       │    │
                                 │   └──────────────┘      └─────────────────────┘    │
                                 └─────────────────────────────────────────────────────┘
```

## 部署步骤

### 1. 准备工作

```bash
# 创建命名空间
kubectl create namespace workany

# 创建镜像拉取密钥 (如果使用私有仓库)
kubectl create secret docker-registry regcred \
  --docker-server=your-registry.com \
  --docker-username=your-username \
  --docker-password=your-password \
  -n workany
```

### 2. 部署配置文件

按顺序应用以下配置文件：

```bash
cd deploy/

# 1. 存储
kubectl apply -f pvc.yaml

# 2. 数据库 (或使用云数据库服务)
kubectl apply -f postgres.yaml

# 3. RBAC 权限
kubectl apply -f rbac.yaml

# 4. ConfigMap
kubectl apply -f configmap.yaml

# 5. Clawork 服务
kubectl apply -f clawork.yaml

# 6. Ingress (需要先安装 ingress-nginx 和 cert-manager)
kubectl apply -f ingress.yaml
```

### 3. 验证部署

```bash
# 检查 Pod 状态
kubectl get pods -n workany

# 检查服务
kubectl get svc -n workany

# 检查日志
kubectl logs -f deployment/clawork -n workany

# 测试 API
curl https://workany.ai/bot/api/v1/bots
```

## 域名配置

### DNS 记录

```
workany.ai      A     -> 你的 Ingress IP
*.workany.bot   A     -> 你的 Ingress IP
```

### SSL 证书

使用 cert-manager 自动管理 Let's Encrypt 证书：

```bash
# 安装 cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.0/cert-manager.yaml

# 等待 cert-manager 就绪
kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=300s
```

## 配置说明

### 环境变量

| 变量 | 说明 | 默认值 |
|-----|------|-------|
| K8S_NAMESPACE | 部署 Bot 的命名空间 | workany |
| DATABASE_DSN | 数据库连接串 | - |
| SERVER_PORT | 服务端口 | 18080 |

### Bot 资源限制

在 `configmap.yaml` 中配置每个 Bot Pod 的资源限制：

```yaml
openclaw.cpu_limit: "500m"      # CPU 上限
openclaw.memory_limit: "512Mi"  # 内存上限
openclaw.cpu_request: "100m"    # CPU 请求
openclaw.memory_request: "128Mi" # 内存请求
```

## API 接口

### Bot 管理

| 方法 | 路径 | 说明 |
|-----|------|------|
| POST | /bot/api/v1/bots | 创建 Bot |
| GET | /bot/api/v1/bots | 列出所有 Bot |
| GET | /bot/api/v1/bots/:id | 获取 Bot 详情 |
| PUT | /bot/api/v1/bots/:id | 更新 Bot 配置 |
| DELETE | /bot/api/v1/bots/:id | 删除 Bot |
| POST | /bot/api/v1/bots/:id/start | 启动 Bot |
| POST | /bot/api/v1/bots/:id/stop | 停止 Bot |
| POST | /bot/api/v1/bots/:id/restart | 重启 Bot |

### IM 渠道管理

| 方法 | 路径 | 说明 |
|-----|------|------|
| POST | /bot/api/v1/bots/:id/channels | 添加 IM 渠道 |
| GET | /bot/api/v1/bots/:id/channels | 列出 IM 渠道 |
| DELETE | /bot/api/v1/bots/:id/channels/:channel | 删除 IM 渠道 |

## 使用示例

### 创建 Bot

```bash
curl -X POST https://workany.ai/bot/api/v1/bots \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-assistant",
    "user_id": "user-123",
    "config": {
      "api_key": "your-minimax-api-key",
      "base_url": "https://api.minimaxi.com/anthropic",
      "model": "MiniMax-M2.1"
    }
  }'
```

### 启动 Bot

```bash
curl -X POST https://workany.ai/bot/api/v1/bots/{bot_id}/start
```

### 添加 Telegram 渠道

```bash
curl -X POST https://workany.ai/bot/api/v1/bots/{bot_id}/channels \
  -H "Content-Type: application/json" \
  -d '{
    "channel": "telegram",
    "token": "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ"
  }'
```

### 访问 Bot WebUI

打开浏览器访问: `https://{bot_id}.workany.bot`

## 监控和日志

### 查看 Bot 日志

```bash
# 列出所有 Bot Pod
kubectl get pods -n workany -l app=openclaw

# 查看特定 Bot 日志
kubectl logs -f <pod-name> -n workany
```

### 健康检查

```bash
# Clawork 健康检查
curl https://workany.ai/health

# Bot 健康检查 (通过 WebSocket)
# 在 Bot WebUI 查看 Health 状态
```

## 故障排除

### Bot 启动失败

1. 检查 PVC 是否正常挂载
2. 检查 Bot Pod 日志
3. 确认 API Key 是否正确

### IM 连接失败

1. 检查 Token 是否正确
2. 查看 Bot Pod 日志中的 channel 相关错误
3. 确认网络是否能访问 IM 服务

### WebUI 无法访问

1. 检查 Ingress 配置
2. 确认 DNS 解析正确
3. 检查 SSL 证书状态
