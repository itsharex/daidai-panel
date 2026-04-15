#!/system/bin/sh
##########################################################################
# 呆呆面板 Magisk 模块卸载脚本
#
# 此脚本会在 Magisk / KernelSU / APatch 移除本模块时被调用。
# 默认会把模块产生的全部数据一并清理，做到「卸载即还原」：
#
#   - 停止仍在运行的 daidai-server 进程
#   - 删除持久化数据目录 /data/adb/daidai-panel/
#     （数据库、脚本、日志、备份等都会被清掉）
#
# 如果你希望保留数据以便重装后继续使用，请在卸载前创建保留标记：
#   su -c "touch /data/adb/daidai-panel/.keep_on_uninstall"
# 存在该标记时，本脚本不会删除数据目录。
##########################################################################

PANEL_DIR=/data/adb/daidai-panel
KEEP_FLAG="$PANEL_DIR/.keep_on_uninstall"
LOG_TAG="daidai-panel-uninstall"

log() {
  # Magisk 卸载阶段 ui_print 不一定可用，这里同时写系统日志
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >&2
  log -t "$LOG_TAG" "$1" 2>/dev/null
}

log "卸载脚本开始执行"

# 1. 停止面板进程（模块目录和数据目录都尝试一下，尽量不残留）
pkill -f "/data/adb/modules/daidai-panel/daidai-server" 2>/dev/null
pkill -f "daidai-server" 2>/dev/null
sleep 1
pkill -9 -f "daidai-server" 2>/dev/null

# 2. 根据是否存在保留标记决定处理方式
if [ -f "$KEEP_FLAG" ]; then
  log "检测到保留标记 $KEEP_FLAG，跳过数据目录清理"
  log "如需彻底删除：su -c \"rm -rf $PANEL_DIR\""
else
  if [ -d "$PANEL_DIR" ]; then
    log "清理持久化数据目录: $PANEL_DIR"
    rm -rf "$PANEL_DIR"
  fi
fi

# 3. 清理历史版本可能写入的其它路径（旧版本模块遗留）
rm -f /system/etc/init.d/99daidai 2>/dev/null
rm -f /data/adb/service.d/daidai-panel.sh 2>/dev/null
rm -f /data/local/tmp/daidai-panel.* 2>/dev/null

log "卸载完成；重启后模块本体目录会被 Magisk 自动清除"
