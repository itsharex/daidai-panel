#!/system/bin/sh
##########################################################################
# 呆呆面板 Magisk 模块 - 快捷操作脚本
#
# 点击管理器卡片上的「运行」按钮触发。
##########################################################################

MODDIR=${0%/*}
PERSIST_DIR=/data/adb/daidai-panel
SERVICE_LOG="$PERSIST_DIR/service.log"
RURIMA=$MODDIR/system/bin/rurima

rootfs=/data/daidai
[ ! -d "$rootfs" ] && rootfs=/data/local/daidai

SERVER_LOG="$rootfs/app/Dumb-Panel/daidai.log"
TAIL_LINES=60

if ! command -v ui_print >/dev/null 2>&1; then
  ui_print() { echo "$1"; }
fi

ui_print "========================================="
ui_print " 呆呆面板 - 运行状态"
ui_print "========================================="

# ---- 进程状态（容器内） -------------------------------------------------
PID=""
if [ -x "$RURIMA" ] && [ -d "$rootfs" ]; then
  PID=$("$RURIMA" ruri -p -N -S -A "$rootfs" /bin/ash -c "pgrep -f /usr/local/bin/daidai-server | head -n1" 2>/dev/null)
fi

if [ -n "$PID" ]; then
  ui_print "- 状态: 运行中"
  ui_print "- PID : $PID (容器内)"
else
  ui_print "- 状态: 未运行"
fi

# ---- 端口监听（宿主侧 0.0.0.0:5700） -----------------------------------
PORT_INFO=$(netstat -ltn 2>/dev/null | grep ':5700' | head -n2)
if [ -n "$PORT_INFO" ]; then
  ui_print "- 监听端口:"
  echo "$PORT_INFO" | while IFS= read -r line; do
    ui_print "    $line"
  done
else
  ui_print "- 监听端口: 未检测到 (5700 未监听)"
fi

ui_print "- 访问地址: http://127.0.0.1:5700"
ui_print "- rootfs  : $rootfs"
ui_print "- 数据目录: $rootfs/app/Dumb-Panel"

# ---- 容器运行时自检 ----------------------------------------------------
if [ -x "$RURIMA" ] && [ -d "$rootfs" ]; then
  ui_print " "
  ui_print "--- 容器运行时 ---"
  "$RURIMA" ruri -p -N -S -A "$rootfs" /bin/ash -c '
    for c in python3 node npm git curl bash; do
      p=$(command -v $c 2>/dev/null)
      if [ -n "$p" ]; then
        v=$($c --version 2>&1 | head -n1)
        echo "$c: $p | $v"
      else
        echo "$c: 缺失"
      fi
    done
  ' 2>/dev/null | while IFS= read -r line; do
    ui_print "$line"
  done
fi

# ---- service.log --------------------------------------------------------
ui_print " "
ui_print "--- service.log (最近 ${TAIL_LINES} 行) ---"
if [ -f "$SERVICE_LOG" ]; then
  tail -n "$TAIL_LINES" "$SERVICE_LOG" 2>/dev/null | while IFS= read -r line; do
    ui_print "$line"
  done
else
  ui_print "(暂无 $SERVICE_LOG)"
fi

# ---- daidai.log (容器内后端日志) ----------------------------------------
ui_print " "
ui_print "--- daidai.log (最近 ${TAIL_LINES} 行) ---"
if [ -f "$SERVER_LOG" ]; then
  tail -n "$TAIL_LINES" "$SERVER_LOG" 2>/dev/null | while IFS= read -r line; do
    ui_print "$line"
  done
else
  ui_print "(暂无 $SERVER_LOG)"
fi

ui_print " "
ui_print "========================================="
ui_print " 常用命令 (adb shell / Termux):"
ui_print "   进入容器:"
ui_print "     su -c \"$RURIMA ruri -p -N -S -A $rootfs /bin/bash\""
ui_print "   重启面板:"
ui_print "     su -c \"pkill -f daidai-server; sh $MODDIR/service.sh &\""
ui_print "========================================="
