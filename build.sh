#!/bin/bash
set -e

VERSION="${1:-latest}"
IMAGE="linzixuanzz/daidai-panel"

echo "========================================="
echo " 呆呆面板 本地构建推送工具"
echo " 版本: ${VERSION}"
echo "========================================="

BUILDER_NAME="multiarch"
if ! docker buildx inspect "$BUILDER_NAME" > /dev/null 2>&1; then
    echo "[1/4] 创建多架构 builder..."
    docker buildx create --name "$BUILDER_NAME" --driver docker-container --platform linux/amd64,linux/arm64
fi
docker buildx use "$BUILDER_NAME"

echo "[2/4] 启动 builder..."
docker buildx inspect --bootstrap > /dev/null 2>&1 || {
    echo "ERROR: builder 启动失败，请检查网络（需要拉取 moby/buildkit 镜像）"
    exit 1
}

echo "[3/4] 登录 Docker Hub..."
docker login 2>/dev/null || {
    echo "ERROR: Docker Hub 登录失败，请先执行 docker login"
    exit 1
}

echo "[4/4] 构建并推送 (amd64 + arm64)..."
TAGS="-t ${IMAGE}:${VERSION}"
if [ "$VERSION" != "latest" ]; then
    TAGS="${TAGS} -t ${IMAGE}:latest"
fi

docker buildx build \
    --platform linux/amd64,linux/arm64 \
    --build-arg VERSION="${VERSION}" \
    ${TAGS} \
    --push \
    .

echo ""
echo "========================================="
echo " 构建完成！已推送:"
echo "   ${IMAGE}:${VERSION}"
if [ "$VERSION" != "latest" ]; then
    echo "   ${IMAGE}:latest"
fi
echo "========================================="
