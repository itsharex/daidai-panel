#!/bin/sh

DATA_DIR=${DATA_DIR:-/app/Dumb-Panel}
SERVER_PID_FILE="${DATA_DIR}/run/daidai-server.pid"

mkdir -p "${DATA_DIR}/scripts" "${DATA_DIR}/logs" "${DATA_DIR}/backups" "${DATA_DIR}/run"
mkdir -p "${DATA_DIR}/deps/nodejs" "${DATA_DIR}/deps/python"
mkdir -p /tmp
chmod 1777 /tmp

export NODE_PATH="${DATA_DIR}/deps/nodejs/node_modules"
export PATH="${DATA_DIR}/deps/nodejs/node_modules/.bin:${DATA_DIR}/deps/python/venv/bin:${PATH}"

if [ -d "${DATA_DIR}/deps/python/venv" ]; then
  export PYTHONPATH="${DATA_DIR}/deps/python/venv/lib/python3.$(python3 -c 'import sys;print(f"{sys.version_info.minor}")')/site-packages"
fi

PANEL_PORT=${PANEL_PORT:-5700}

NGINX_CONF_PATH=${NGINX_DEFAULT_CONF:-}
if [ -z "${NGINX_CONF_PATH}" ]; then
  for candidate in /etc/nginx/http.d/default.conf /etc/nginx/conf.d/default.conf /etc/nginx/sites-enabled/default; do
    if [ -f "${candidate}" ]; then
      NGINX_CONF_PATH="${candidate}"
      break
    fi
  done
fi

if [ -n "${NGINX_CONF_PATH}" ] && [ -f "${NGINX_CONF_PATH}" ]; then
  sed -i "s/listen [0-9]*/listen ${PANEL_PORT}/" "${NGINX_CONF_PATH}"
fi

cat > /app/config.yaml <<YAML
server:
  port: 5701
  mode: release

database:
  path: ${DATA_DIR}/daidai.db

jwt:
  secret: ""
  access_token_expire: 480h
  refresh_token_expire: 1440h

data:
  dir: ${DATA_DIR}
  scripts_dir: ${DATA_DIR}/scripts
  log_dir: ${DATA_DIR}/logs

cors:
  origins:
    - http://localhost:5173
    - http://localhost:${PANEL_PORT}
YAML

if [ $# -gt 0 ]; then
  exec "$@"
fi

nginx

shutdown() {
    kill "$SERVER_PID" 2>/dev/null
    rm -f "$SERVER_PID_FILE"
    exit 0
}
trap shutdown SIGTERM SIGINT

while true; do
    /app/daidai-server &
    SERVER_PID=$!
    echo "$SERVER_PID" > "$SERVER_PID_FILE"
    wait $SERVER_PID
    EXIT_CODE=$?
    rm -f "$SERVER_PID_FILE"
    [ $EXIT_CODE -eq 0 ] && exit 0
    sleep 2
done
