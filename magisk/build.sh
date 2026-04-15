#!/usr/bin/env bash
##########################################################################
# 呆呆面板 Magisk 模块打包脚本
#
# 用法:
#   bash Magisk/build.sh            # 默认打包 arm64
#   bash Magisk/build.sh 2.0.6      # 指定版本号
#   bash Magisk/build.sh 2.0.6 all  # 同时打包 arm64 + amd64
#
# 产物: dist/daidai-panel-magisk-<版本>.zip
##########################################################################

set -euo pipefail

VERSION="${1:-2.0.6}"
TARGETS="${2:-arm64}"     # arm64 / amd64 / all

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

MODDIR="$ROOT/Magisk"
DIST="$ROOT/dist"
STAGING="$DIST/magisk-staging"
OUTZIP="$DIST/daidai-panel-magisk-v${VERSION}.zip"

info()  { printf "\033[1;32m[INFO]\033[0m %s\n" "$*"; }
warn()  { printf "\033[1;33m[WARN]\033[0m %s\n" "$*"; }
error() { printf "\033[1;31m[ERR ]\033[0m %s\n" "$*" >&2; }

command -v go   >/dev/null || { error "缺少 go"; exit 1; }
command -v npm  >/dev/null || { error "缺少 npm"; exit 1; }
command -v zip  >/dev/null || { error "缺少 zip"; exit 1; }

# 1. 前端构建
if [ ! -d "$ROOT/web/dist" ]; then
  info "前端 dist 不存在，开始构建..."
  (cd "$ROOT/web" && npm ci && npm run build)
else
  info "已存在 web/dist，跳过前端构建（如需强制重建请先删除 web/dist）"
fi

# 2. 后端交叉编译（Android root 环境下，GOOS=linux + static 二进制即可）
rm -rf "$STAGING"
mkdir -p "$STAGING/bin" "$STAGING/web" "$DIST"

build_backend() {
  local go_arch="$1"
  local suffix="$2"
  info "编译后端: GOOS=linux GOARCH=${go_arch}"
  (cd "$ROOT/server" && \
    CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" \
    go build -trimpath \
      -ldflags="-s -w -X daidai-panel/handler.Version=${VERSION}" \
      -o "$STAGING/bin/daidai-server-${suffix}" .)
  (cd "$ROOT/server" && \
    CGO_ENABLED=0 GOOS=linux GOARCH="${go_arch}" \
    go build -trimpath \
      -ldflags="-s -w -X daidai-panel/handler.Version=${VERSION}" \
      -o "$STAGING/bin/ddp-${suffix}" ./cmd/ddp)
}

case "$TARGETS" in
  arm64) build_backend arm64 arm64 ;;
  amd64) build_backend amd64 amd64 ;;
  all)
    build_backend arm64 arm64
    build_backend amd64 amd64
    ;;
  *) error "未知架构: $TARGETS （支持: arm64 / amd64 / all）"; exit 1 ;;
esac

# 3. 拷贝模块文件
info "打包模块文件..."
cp -f  "$MODDIR/module.prop"    "$STAGING/"
cp -f  "$MODDIR/customize.sh"   "$STAGING/"
cp -f  "$MODDIR/service.sh"     "$STAGING/"
cp -f  "$MODDIR/uninstall.sh"   "$STAGING/"
cp -f  "$MODDIR/action.sh"      "$STAGING/"
cp -rf "$MODDIR/scripts"        "$STAGING/"
cp -f  "$MODDIR/README.md"      "$STAGING/"
cp -rf "$MODDIR/META-INF"       "$STAGING/"

# 同步版本号到 module.prop
sed -i.bak "s/^version=.*/version=v${VERSION}/" "$STAGING/module.prop"
rm -f "$STAGING/module.prop.bak"

# 前端
cp -rf "$ROOT/web/dist/"* "$STAGING/web/"

# 为方便首次启动缺失 config 时回填，带一份默认 config
cp -f "$ROOT/server/config.yaml" "$STAGING/config.yaml" 2>/dev/null || true

# 4. 打包 ZIP
rm -f "$OUTZIP"
info "生成 ZIP: $OUTZIP"
(cd "$STAGING" && zip -r9 "$OUTZIP" . -x "*.DS_Store")

info "完成: $OUTZIP"
info "用法: 在 Magisk / KernelSU / APatch 管理器中选择此 ZIP 安装即可。"
