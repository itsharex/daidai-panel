#!/system/bin/sh
##########################################################################
# 呆呆面板 Magisk 模块 - late_start service
#
# 该脚本在系统 late_start service 阶段被 Magisk 执行，此时
# /data 已挂载，网络子系统通常也已准备好，适合启动面板后端。
##########################################################################

MODDIR=${0%/*}
PANEL_DIR=/data/adb/daidai-panel
LOG_FILE="$PANEL_DIR/service.log"

# ------------------------------------------------------------------
# 组装运行时 PATH
#   1. 模块自身（daidai-server / ddp）
#   2. Termux 常见路径（如果用户装了 Termux，脚本就能直接用 python/node/git 等）
#   3. 常见静态运行时目录，方便高级用户自行摆放 busybox / python-static
#   4. 系统默认 PATH
# ------------------------------------------------------------------
MODULE_PATHS="$MODDIR"

TERMUX_PATHS=""
for p in \
  /data/data/com.termux/files/usr/bin \
  /data/data/com.termux/files/usr/local/bin \
  /data/user/0/com.termux/files/usr/bin; do
  if [ -d "$p" ]; then
    TERMUX_PATHS="${TERMUX_PATHS:+$TERMUX_PATHS:}$p"
  fi
done

EXTRA_PATHS=""
for p in /data/local/tmp/bin /sbin /system/bin /system/xbin /vendor/bin; do
  [ -d "$p" ] && EXTRA_PATHS="${EXTRA_PATHS:+$EXTRA_PATHS:}$p"
done

# 用户可放自己的运行时二进制到 $PANEL_DIR/bin
if [ -d "$PANEL_DIR/bin" ]; then
  MODULE_PATHS="$PANEL_DIR/bin:$MODULE_PATHS"
fi

export PATH="$MODULE_PATHS${TERMUX_PATHS:+:$TERMUX_PATHS}${EXTRA_PATHS:+:$EXTRA_PATHS}:$PATH"
export HOME="$PANEL_DIR"

# 让 Termux 的动态库也能加载（python/node 可能依赖）
if [ -n "$TERMUX_PATHS" ]; then
  for ldp in \
    /data/data/com.termux/files/usr/lib \
    /data/user/0/com.termux/files/usr/lib; do
    if [ -d "$ldp" ]; then
      export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$ldp"
    fi
  done
fi

mkdir -p "$PANEL_DIR"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE" 2>/dev/null
}

# 简单滚动：日志超过 2MB 后切成 .old
if [ -f "$LOG_FILE" ]; then
  size=$(stat -c%s "$LOG_FILE" 2>/dev/null || echo 0)
  if [ "${size:-0}" -gt 2097152 ]; then
    mv -f "$LOG_FILE" "$LOG_FILE.old" 2>/dev/null
  fi
fi

log "========================================="
log "呆呆面板模块启动 (MODDIR=$MODDIR)"
log "========================================="
log "PATH=$PATH"

# 探测常用脚本运行时，只做记录，缺失不阻塞启动
for cmd in sh bash python3 python node npm pnpm yarn ts-node go git curl wget; do
  p=$(command -v "$cmd" 2>/dev/null)
  if [ -n "$p" ]; then
    log "runtime: $cmd -> $p"
  else
    log "runtime: $cmd -> (缺失)"
  fi
done

# 等待 /data 真正可写（极少数场景下 service 阶段早于 encryption unlock）
for i in 1 2 3 4 5 6 7 8 9 10; do
  if [ -w /data/adb ]; then
    break
  fi
  log "等待 /data/adb 就绪... ($i/10)"
  sleep 3
done

# 等待网络（尽量，失败也不阻塞）
for i in 1 2 3 4 5 6; do
  ip=$(ip route get 1.1.1.1 2>/dev/null | awk '/src/ {print $NF}' | head -n1)
  if [ -n "$ip" ]; then
    log "网络已就绪 (src=$ip)"
    break
  fi
  sleep 5
done

if [ ! -x "$MODDIR/daidai-server" ]; then
  log "!! 找不到或无法执行 $MODDIR/daidai-server"
  exit 1
fi

# 避免重复启动
if pgrep -f "$MODDIR/daidai-server" >/dev/null 2>&1; then
  log "面板进程已在运行，跳过启动"
  exit 0
fi

# 以 $PANEL_DIR 为工作目录，配合 config.yaml 中的相对路径
cd "$PANEL_DIR" || {
  log "无法进入数据目录 $PANEL_DIR"
  exit 1
}

# 如果用户把 config.yaml 删除了，补一份，避免启动失败
if [ ! -f "$PANEL_DIR/config.yaml" ] && [ -f "$MODDIR/config.yaml" ]; then
  cp -f "$MODDIR/config.yaml" "$PANEL_DIR/config.yaml"
fi

log "启动 daidai-server..."
nohup "$MODDIR/daidai-server" >> "$PANEL_DIR/server.log" 2>&1 &
PID=$!

sleep 2
if kill -0 "$PID" 2>/dev/null; then
  log "面板启动成功 (PID=$PID)，访问 http://127.0.0.1:5700"
else
  log "!! 面板启动失败，请查看 $PANEL_DIR/server.log"
fi
