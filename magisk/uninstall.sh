# 呆呆面板 Magisk 模块卸载脚本
# 会在模块被删除时执行

MODDIR=${0%/*}
PANEL_DIR="/data/adb/daidai-panel"
LOG_FILE="$PANEL_DIR/uninstall.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE" 2>/dev/null
}

log "========================================="
log "呆呆面板模块卸载中..."
log "========================================="

# 停止服务
log "停止呆呆面板服务..."
pkill -f "daidai-server" 2>/dev/null
sleep 1

# 清理启动脚本中的自启动配置
log "清理自启动配置..."

# 移除可能的 init.d 脚本
rm -f /system/etc/init.d/99daidai 2>/dev/null
rm -f /data/adb/service.d/daidai-panel.sh 2>/dev/null

# 询问用户是否保留数据
# 注意: Magisk 卸载时无法交互，这里提供选项
# 保留数据目录，用户可以之后手动删除
log "数据目录保留在: $PANEL_DIR"
log "如需完全删除，请手动执行: rm -rf $PANEL_DIR"

log "卸载完成!"
