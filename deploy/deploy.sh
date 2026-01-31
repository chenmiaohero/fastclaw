#!/bin/bash

# Clawork 一键部署脚本
# 使用方法: ./deploy.sh

set -e

NAMESPACE="workany"

echo "=========================================="
echo "Deploying Clawork to Kubernetes"
echo "Namespace: ${NAMESPACE}"
echo "=========================================="

cd "$(dirname "$0")"

# 检查 kubectl
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl not found"
    exit 1
fi

# 检查集群连接
echo ">> Checking cluster connection..."
kubectl cluster-info

# 1. 创建命名空间
echo ">> Creating namespace..."
kubectl apply -f namespace.yaml

# 2. 创建存储
echo ">> Creating PVC..."
kubectl apply -f pvc.yaml

# 3. 部署数据库 (可选，生产环境建议使用云数据库)
echo ">> Deploying PostgreSQL..."
kubectl apply -f postgres.yaml

# 等待数据库就绪
echo ">> Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres -n ${NAMESPACE} --timeout=120s

# 4. 创建 RBAC
echo ">> Creating RBAC..."
kubectl apply -f rbac.yaml

# 5. 创建 ConfigMap
echo ">> Creating ConfigMap..."
kubectl apply -f configmap.yaml

# 6. 部署 Clawork
echo ">> Deploying Clawork..."
kubectl apply -f clawork.yaml

# 等待 Clawork 就绪
echo ">> Waiting for Clawork to be ready..."
kubectl wait --for=condition=ready pod -l app=clawork -n ${NAMESPACE} --timeout=120s

# 7. 部署 Ingress (可选)
read -p "Deploy Ingress? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ">> Deploying Ingress..."
    kubectl apply -f ingress.yaml
fi

echo "=========================================="
echo "Deployment complete!"
echo "=========================================="

# 显示状态
echo ""
echo ">> Pod Status:"
kubectl get pods -n ${NAMESPACE}

echo ""
echo ">> Service Status:"
kubectl get svc -n ${NAMESPACE}

echo ""
echo ">> Ingress Status:"
kubectl get ingress -n ${NAMESPACE}

echo ""
echo "=========================================="
echo "Next Steps:"
echo "1. Configure DNS records for workany.ai and *.workany.bot"
echo "2. Wait for SSL certificates to be issued"
echo "3. Test API: curl https://workany.ai/bot/api/v1/bots"
echo "=========================================="
