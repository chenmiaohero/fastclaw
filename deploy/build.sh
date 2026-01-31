#!/bin/bash

# Clawork 镜像构建和推送脚本
# 使用方法: ./build.sh [镜像仓库地址] [版本号]

set -e

# 配置
REGISTRY=${1:-"your-registry.com/clawork"}
VERSION=${2:-"latest"}
IMAGE_NAME="${REGISTRY}:${VERSION}"

echo "=========================================="
echo "Building Clawork Docker Image"
echo "Image: ${IMAGE_NAME}"
echo "=========================================="

# 进入项目根目录
cd "$(dirname "$0")/.."

# 构建镜像
echo ">> Building image..."
docker build -t ${IMAGE_NAME} .

# 推送镜像
echo ">> Pushing image..."
docker push ${IMAGE_NAME}

echo "=========================================="
echo "Build complete!"
echo "Image: ${IMAGE_NAME}"
echo "=========================================="

# 输出部署命令
echo ""
echo "To deploy, update clawork.yaml with the new image and run:"
echo "  kubectl set image deployment/clawork clawork=${IMAGE_NAME} -n workany"
echo ""
echo "Or apply the full configuration:"
echo "  kubectl apply -f deploy/"
