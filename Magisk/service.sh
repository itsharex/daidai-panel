#!/system/bin/sh
##########################################################################
# 呆呆面板 Magisk 模块 - late_start service
#
# 进入 Alpine 容器启动 daidai-server，单端口 5700（API + 前端静态资源
# 都由 daidai-server 自己托管，无需 nginx）。
##########################################################################

export PATH=/data/adb/ap/bin:/data/adb/ksu/bin:/data/adb/magisk:$PATH

# rootfs 位置探测
rootfs=/data/daidai
if [ ! -d "$rootfs" ]; then
  rootfs=/data/local/daidai
fi

# 模块目录探测
MODDIR=${MODDIR:-/data/adb/modules/daidai-panel}
[ ! -d "$MODDIR" ] && MODDIR=/data/adb/magisk/modules/daidai-panel
[ ! -d "$MODDIR" ] && MODDIR=/sbin/.magisk/modules/daidai-panel
[ ! -d "$MODDIR" ] && MODDIR=$(dirname "$0")
RURIMA=$MODDIR/system/bin/rurima

PERSIST_DIR=/data/adb/daidai-panel
LOG_FILE="$PERSIST_DIR/service.log"

mkdir -p "$PERSIST_DIR"

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE" 2>/dev/null
}

# 日志滚动
if [ -f "$LOG_FILE" ]; then
  size=$(stat -c%s "$LOG_FILE" 2>/dev/null || echo 0)
  [ "${size:-0}" -gt 2097152 ] && mv -f "$LOG_FILE" "$LOG_FILE.old" 2>/dev/null
fi

log "========================================="
log "呆呆面板模块启动 (MODDIR=$MODDIR, rootfs=$rootfs)"
log "========================================="

echo "noSuspend" > /sys/power/wake_lock 2>/dev/null
dumpsys deviceidle disable 2>/dev/null || true

# 等网络就绪（尽量，失败也不阻塞）
for i in 1 2 3 4 5; do
  if busybox nslookup m.baidu.com >/dev/null 2>&1; then
    log "网络已就绪"
    break
  fi
  sleep 5
done

if [ ! -f "$RURIMA" ]; then
  log "!! 找不到 rurima 二进制: $RURIMA"
  exit 1
fi

chmod +x "$RURIMA" 2>/dev/null

if [ ! -d "$rootfs" ]; then
  log "!! 找不到 rootfs: $rootfs，模块可能未完成安装，请重装"
  exit 1
fi

# KernelSU 下 /data 可能以 ro 挂载，确保可写
if [ -d "/data/adb/ksu" ]; then
  mount -o remount,rw /data 2>/dev/null
fi

# 把最新的前端和 daidai-server 同步进容器
mkdir -p $rootfs/app/web $rootfs/app/Dumb-Panel $rootfs/usr/local/bin
cp -rf $MODDIR/web/* $rootfs/app/web/ 2>/dev/null
cp -f  $MODDIR/system/bin/daidai-server $rootfs/usr/local/bin/daidai-server 2>/dev/null
chmod 755 $rootfs/usr/local/bin/daidai-server 2>/dev/null

if [ -f $MODDIR/system/bin/ddp ]; then
  cp -f  $MODDIR/system/bin/ddp $rootfs/usr/local/bin/ddp 2>/dev/null
  chmod 755 $rootfs/usr/local/bin/ddp 2>/dev/null
fi

cp -f $MODDIR/module.prop $rootfs/app/module.prop 2>/dev/null

log "进入 Alpine 容器启动 daidai-server..."

"$RURIMA" ruri -p -N -S -A $rootfs /bin/ash << 'CONTAINER_EOF'
export DAIDAI_DIR=/app/Dumb-Panel
export LANG=C.UTF-8
export HOME=/root
export SHELL=/bin/bash
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/app
export NODE_PATH=/usr/local/lib/node_modules

mkdir -p $DAIDAI_DIR/scripts $DAIDAI_DIR/logs $DAIDAI_DIR/deps/nodejs $DAIDAI_DIR/deps/python $DAIDAI_DIR/backups
chmod 777 $DAIDAI_DIR

# Python 虚拟环境（第一次进入时创建）
if [ ! -d "$DAIDAI_DIR/deps/python/venv" ]; then
  python3 -m venv $DAIDAI_DIR/deps/python/venv 2>/dev/null || true
fi

# 每次启动都用当前 config 模板覆盖（用户自定义请编辑容器外 /data/adb/daidai-panel/config.yaml 再用 ddp 同步）
cat > $DAIDAI_DIR/config.yaml << 'YAML'
server:
  port: 5700
  mode: release
  web_dir: /app/web

database:
  path: /app/Dumb-Panel/daidai.db

jwt:
  secret: ""
  access_token_expire: 480h
  refresh_token_expire: 1440h

data:
  dir: /app/Dumb-Panel
  scripts_dir: /app/Dumb-Panel/scripts
  log_dir: /app/Dumb-Panel/logs

cors:
  origins:
    - http://localhost:5700
    - http://127.0.0.1:5700
YAML

# 避免重复拉起
if pgrep -f /usr/local/bin/daidai-server >/dev/null 2>&1; then
  echo "daidai-server 已在运行" >> $DAIDAI_DIR/service.log
  exit 0
fi

cd $DAIDAI_DIR
nohup /usr/local/bin/daidai-server > $DAIDAI_DIR/daidai.log 2>&1 &
echo "daidai-server 已拉起 PID=$!" >> $DAIDAI_DIR/service.log
exit 0
CONTAINER_EOF

sleep 2

# 容器内启动后简单验证
if "$RURIMA" ruri -p -N -S -A $rootfs /bin/ash -c "pgrep -f /usr/local/bin/daidai-server >/dev/null 2>&1"; then
  log "面板启动成功，访问 http://127.0.0.1:5700"
else
  log "!! 面板启动失败，查看 $rootfs/app/Dumb-Panel/daidai.log"
fi
