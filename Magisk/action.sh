#!/system/bin/sh
##########################################################################
# 呆呆面板 Magisk 模块 - 快捷操作脚本
#
# Magisk v26+ / KernelSU / APatch 会在模块卡片上显示「运行」按钮，
# 点击时会执行本脚本，通过 ui_print 把内容输出到管理器面板上。
#
# 本脚本用于：
#   1. 查看面板运行状态
#   2. 打印最近的 service.log / server.log 日志
#   3. 必要时提供重启 / 停止的提示
##########################################################################

MODDIR=${0%/*}
PANEL_DIR=/data/adb/daidai-panel
SERVICE_LOG="$PANEL_DIR/service.log"
SERVER_LOG="$PANEL_DIR/server.log"
TAIL_LINES=60

# 兼容部分管理器没有 ui_print 的情况
if ! command -v ui_print >/dev/null 2>&1; then
  ui_print() { echo "$1"; }
fi

ui_print "========================================="
ui_print " 呆呆面板 - 运行状态"
ui_print "========================================="

# --- 进程状态 ---------------------------------------------------------
PID=$(pgrep -f "$MODDIR/daidai-server" | head -n1)
if [ -z "$PID" ]; then
  PID=$(pgrep -f "daidai-server" | head -n1)
fi

if [ -n "$PID" ]; then
  ui_print "- 状态: 运行中"
  ui_print "- PID : $PID"
else
  ui_print "- 状态: 未运行"
fi

# --- 端口监听 ---------------------------------------------------------
PORT_INFO=$(netstat -ltn 2>/dev/null | grep -E ':5700|:5701' | head -n2)
if [ -n "$PORT_INFO" ]; then
  ui_print "- 监听端口:"
  echo "$PORT_INFO" | while IFS= read -r line; do
    ui_print "    $line"
  done
else
  ui_print "- 监听端口: 未检测到 (5700/5701 未监听)"
fi

# --- 访问地址 ---------------------------------------------------------
ui_print "- 访问地址: http://127.0.0.1:5700"
ui_print "- 数据目录: $PANEL_DIR"

# --- 内置 ddp 快速检查 ------------------------------------------------
if [ -x "$MODDIR/ddp" ]; then
  ui_print " "
  ui_print "--- ddp status ---"
  "$MODDIR/ddp" status 2>&1 | while IFS= read -r line; do
    ui_print "$line"
  done
fi

# --- 脚本运行时自检 ---------------------------------------------------
if [ -f "$MODDIR/scripts/check-runtimes.sh" ]; then
  ui_print " "
  ui_print "--- 脚本运行时自检 ---"
  sh "$MODDIR/scripts/check-runtimes.sh" 2>&1 | while IFS= read -r line; do
    ui_print "$line"
  done
fi

# --- 服务启动日志 -----------------------------------------------------
ui_print " "
ui_print "--- service.log (最近 ${TAIL_LINES} 行) ---"
if [ -f "$SERVICE_LOG" ]; then
  tail -n "$TAIL_LINES" "$SERVICE_LOG" 2>/dev/null | while IFS= read -r line; do
    ui_print "$line"
  done
else
  ui_print "(暂无 $SERVICE_LOG)"
fi

# --- 后端运行日志 -----------------------------------------------------
ui_print " "
ui_print "--- server.log (最近 ${TAIL_LINES} 行) ---"
if [ -f "$SERVER_LOG" ]; then
  tail -n "$TAIL_LINES" "$SERVER_LOG" 2>/dev/null | while IFS= read -r line; do
    ui_print "$line"
  done
else
  ui_print "(暂无 $SERVER_LOG)"
fi

ui_print " "
ui_print "========================================="
ui_print " 其他常用命令 (adb shell / Termux):"
ui_print "   查看实时日志:"
ui_print "     su -c \"tail -f $SERVICE_LOG\""
ui_print "     su -c \"tail -f $SERVER_LOG\""
ui_print "   重启面板:"
ui_print "     su -c \"pkill -f daidai-server;"
ui_print "             sh $MODDIR/service.sh &\""
ui_print "   忘记密码:"
ui_print "     su -c \"$MODDIR/ddp list-users\""
ui_print "     su -c \"$MODDIR/ddp reset-password admin NewPass123\""
ui_print "========================================="
