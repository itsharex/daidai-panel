# 呆呆面板 Magisk 模块启动脚本
# 会在 Magisk 开机时自动执行

MODDIR=${0%/*}
PANEL_DIR="/data/adb/daidai-panel"
LOG_FILE="$PANEL_DIR/service.log"

# 日志函数
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE" 2>/dev/null
}

log "========================================="
log "呆呆面板模块启动中..."
log "MODDIR: $MODDIR"
log "========================================="

# 创建面板数据目录
mkdir -p "$PANEL_DIR/data" "$PANEL_DIR/scripts" "$PANEL_DIR/logs" "$PANEL_DIR/backups" "$PANEL_DIR/web"

# 检查二进制文件
if [ ! -f "$MODDIR/daidai-server" ]; then
    log "错误: 找不到 daidai-server 二进制文件!"
    exit 1
fi

# 设置权限
chmod 755 "$MODDIR/daidai-server"
chmod 755 "$MODDIR/ddp" 2>/dev/null
chmod -R 755 "$MODDIR/web/" 2>/dev/null

# 复制前端文件(如果目录为空)
if [ ! -f "$PANEL_DIR/web/index.html" ] && [ -d "$MODDIR/web" ]; then
    log "复制前端文件..."
    cp -rf "$MODDIR/web/"* "$PANEL_DIR/web/" 2>/dev/null
fi

# 创建默认配置文件
if [ ! -f "$PANEL_DIR/config.yaml" ]; then
    log "创建默认配置文件..."
    cat > "$PANEL_DIR/config.yaml" << 'EOF'
server:
  port: 5700
  mode: release

database:
  path: ./data/daidai.db

jwt:
  secret: "daidai-panel-android-$(date +%s)"
  access_token_expire: 480h
  refresh_token_expire: 1440h

data:
  dir: ./data
  scripts_dir: ./data/scripts
  log_dir: ./data/logs

cors:
  origins:
    - http://localhost:5700
    - http://127.0.0.1:5700
EOF
    chmod 644 "$PANEL_DIR/config.yaml"
fi

# 设置目录权限
chown -R 0:0 "$PANEL_DIR" 2>/dev/null
chmod -R 755 "$PANEL_DIR" 2>/dev/null

# 停止旧进程
pkill -f "daidai-server" 2>/dev/null
sleep 1

# 启动服务
cd "$PANEL_DIR"
log "启动呆呆面板服务..."
nohup "$MODDIR/daidai-server" > "$PANEL_DIR/server.log" 2>&1 &

sleep 2

# 检查服务是否启动成功
if pgrep -f "daidai-server" > /dev/null; then
    log "呆呆面板服务启动成功!"
    log "访问地址: http://localhost:5700"
else
    log "警告: 服务可能未正常启动，请检查 $PANEL_DIR/server.log"
fi

log "模块启动脚本执行完成"
