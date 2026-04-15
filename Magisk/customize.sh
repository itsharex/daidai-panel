#!/system/bin/sh
##########################################################################
# 呆呆面板 Magisk 模块安装脚本
#
# 变量说明 (由 Magisk 安装框架注入):
#   MODPATH   模块安装到 /data/adb/modules_update/<id> 的路径
#   TMPDIR    临时目录
#   ZIPFILE   当前安装的 zip 包路径
#   ARCH      设备主架构 (arm/arm64/x86/x64)
#   API       Android API Level
##########################################################################

SKIPUNZIP=0

PANEL_DIR=/data/adb/daidai-panel

ui_print " "
ui_print "======================================"
ui_print " 呆呆面板 (Daidai Panel) Magisk 模块"
ui_print "======================================"
ui_print "- 设备架构: $ARCH"
ui_print "- Android API: $API"

# 架构检查：当前仅发布 ARM64 版本的后端二进制
if [ "$ARCH" != "arm64" ] && [ "$ARCH" != "x64" ]; then
  abort "! 当前仅支持 arm64 / x86_64，设备架构 $ARCH 暂不支持"
fi

if [ "$API" -lt 26 ]; then
  abort "! 要求 Android 8.0 (API 26) 及以上，当前 API=$API"
fi

# 依据架构挑选对应二进制，并统一重命名为 daidai-server / ddp
ui_print "- 正在部署模块文件..."
if [ "$ARCH" = "arm64" ]; then
  BIN_SUFFIX="arm64"
else
  BIN_SUFFIX="amd64"
fi

if [ ! -f "$MODPATH/bin/daidai-server-${BIN_SUFFIX}" ]; then
  abort "! 模块包缺少 bin/daidai-server-${BIN_SUFFIX}，无法安装"
fi

mv -f "$MODPATH/bin/daidai-server-${BIN_SUFFIX}" "$MODPATH/daidai-server"
if [ -f "$MODPATH/bin/ddp-${BIN_SUFFIX}" ]; then
  mv -f "$MODPATH/bin/ddp-${BIN_SUFFIX}" "$MODPATH/ddp"
fi
rm -rf "$MODPATH/bin"

# 准备持久化数据目录（不会随模块卸载清除）
ui_print "- 正在准备数据目录 $PANEL_DIR ..."
mkdir -p "$PANEL_DIR"
mkdir -p "$PANEL_DIR/data"
mkdir -p "$PANEL_DIR/data/scripts"
mkdir -p "$PANEL_DIR/data/logs"
mkdir -p "$PANEL_DIR/data/backups"
mkdir -p "$PANEL_DIR/data/deps"

# 首次安装时生成默认 config.yaml
# Magisk 环境没有 nginx，daidai-server 需要同时托管前端 → 单端口 5700，
# 并通过 web_dir 指向模块内置的前端静态文件目录。
if [ ! -f "$PANEL_DIR/config.yaml" ]; then
  ui_print "- 生成默认 config.yaml ..."
  cat > "$PANEL_DIR/config.yaml" <<EOF
server:
  port: 5700
  mode: release
  web_dir: /data/adb/modules/daidai-panel/web

database:
  path: ./data/daidai.db

jwt:
  secret: ""
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
else
  ui_print "- 检测到已有 config.yaml，自动迁移到单端口 5700 + web_dir ..."
  # 若来自 v2.0.6 首发版本的 5701 配置，升级到单端口方案
  sed -i -e 's/^  port: 5701/  port: 5700/' "$PANEL_DIR/config.yaml" 2>/dev/null
  # 若未设置 web_dir，注入一条；若已设置则覆盖为当前模块路径（保证 updater 升级后路径正确）
  if grep -q '^\s*web_dir:' "$PANEL_DIR/config.yaml"; then
    sed -i -E "s|^(\s*web_dir:).*|\1 /data/adb/modules/daidai-panel/web|" "$PANEL_DIR/config.yaml"
  else
    sed -i -E "s|^(\s*mode:.*)$|\1\n  web_dir: /data/adb/modules/daidai-panel/web|" "$PANEL_DIR/config.yaml"
  fi
fi

# 权限设置：
#   二进制 -> 可执行
#   web    -> 只读即可
ui_print "- 修正文件权限..."
set_perm_recursive "$MODPATH"        0 0 0755 0644
set_perm           "$MODPATH/daidai-server" 0 0 0755
[ -f "$MODPATH/ddp" ] && set_perm "$MODPATH/ddp" 0 0 0755
set_perm           "$MODPATH/service.sh"    0 0 0755
set_perm           "$MODPATH/uninstall.sh"  0 0 0755
[ -f "$MODPATH/action.sh" ] && set_perm "$MODPATH/action.sh" 0 0 0755
if [ -d "$MODPATH/scripts" ]; then
  set_perm_recursive "$MODPATH/scripts" 0 0 0755 0755
fi

# 数据目录权限（允许面板进程读写）
chown -R 0:0 "$PANEL_DIR" 2>/dev/null
chmod -R 0700 "$PANEL_DIR" 2>/dev/null

ui_print " "
ui_print "- 安装完成！"
ui_print "- 重启手机后，面板会自动启动。"
ui_print "- 访问地址: http://127.0.0.1:5700"
ui_print "- 数据目录: $PANEL_DIR"
ui_print "- 日志文件: $PANEL_DIR/service.log"
ui_print " "
