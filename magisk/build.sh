# 呆呆面板 Magisk 模块构建脚本
# 用于在本地构建 Magisk 模块 zip 包

set -e

# 配置
MODULE_NAME="daidai-panel"
VERSION="${1:-2.0.4}"
BUILD_DIR="./magisk-build"
ARCH="${2:-arm64}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查环境
check_env() {
    log_info "检查构建环境..."

    if ! command -v zip &> /dev/null; then
        log_error "需要安装 zip 命令"
        exit 1
    fi

    if ! command -v go &> /dev/null; then
        log_error "需要安装 Go 环境"
        exit 1
    fi

    log_info "环境检查完成"
}

# 下载前端构建文件
prepare_frontend() {
    log_info "准备前端文件..."

    # 如果 web/dist 不存在，尝试构建
    if [ ! -d "web/dist" ]; then
        log_warn "前端构建目录不存在，尝试构建..."
        if [ -d "web" ]; then
            cd web
            npm install
            npm run build
            cd ..
        else
            log_error "web 目录不存在"
            exit 1
        fi
    fi

    log_info "前端文件准备完成"
}

# 编译 Android ARM 后端
build_backend() {
    log_info "编译 Android $ARCH 后端..."

    cd server

    # 设置 Go 环境
    export CGO_ENABLED=0
    export GOOS=android
    export GOARCH=$ARCH

    # 编译主程序
    go build -ldflags="-s -w -X daidai-panel/handler.Version=${VERSION}" -o ../$BUILD_DIR/daidai-server .

    # 编译命令行工具
    go build -ldflags="-s -w" -o ../$BUILD_DIR/ddp ./cmd/ddp

    cd ..

    log_info "后端编译完成"
}

# 准备模块目录
prepare_module() {
    log_info "准备模块目录..."

    rm -rf $BUILD_DIR
    mkdir -p $BUILD_DIR

    # 复制模块文件
    cp magisk/module.json $BUILD_DIR/
    cp magisk/system.prop $BUILD_DIR/
    cp magisk/service.sh $BUILD_DIR/
    cp magisk/uninstall.sh $BUILD_DIR/
    cp -r magisk/META-INF $BUILD_DIR/

    # 复制二进制文件
    cp $BUILD_DIR/daidai-server $BUILD_DIR/
    cp $BUILD_DIR/ddp $BUILD_DIR/ 2>/dev/null || true

    # 复制前端文件
    mkdir -p $BUILD_DIR/web
    cp -r web/dist/* $BUILD_DIR/web/

    # 复制 README
    cp magisk/README.md $BUILD_DIR/

    log_info "模块目录准备完成"
}

# 打包 ZIP
package_module() {
    log_info "打包模块..."

    cd $BUILD_DIR

    # 创建 zip 包
    zip -r "../${MODULE_NAME}-magisk-${VERSION}-${ARCH}.zip" .

    cd ..

    log_info "打包完成: ${MODULE_NAME}-magisk-${VERSION}-${ARCH}.zip"
}

# 主函数
main() {
    log_info "开始构建呆呆面板 Magisk 模块..."
    log_info "版本: $VERSION"
    log_info "架构: $ARCH"

    check_env
    prepare_frontend
    build_backend
    prepare_module
    package_module

    log_info "构建完成!"
}

main "$@"
