#!/usr/bin/env bash
# 常驻启动 slave-master（连服务器 Gateway；不自动起子 Slave）。
# 请在「终端.app / iTerm」里跑，不要依赖 Cursor Agent 后台。
#
# 用法:
#   cd build/slave && ./start-master.sh
#   ./start-master.sh status|stop
#
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

PID_FILE="$ROOT/master.pid"
LOG_FILE="$ROOT/logs/master.log"
CFG="$ROOT/master.config.yaml"
TOKEN_FILE="$ROOT/.gateway.token"
GATEWAY_HTTP="${GATEWAY_HTTP:-http://192.168.2.2:8080}"
PAIR_CODE="${GATEWAY_PAIR_CODE:-ABCD-EFGH}"

mkdir -p "$ROOT/logs"

cmd="${1:-start}"

is_alive() {
  local pid="$1"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

refresh_token() {
  local tok
  tok="$(curl -sS -X POST "$GATEWAY_HTTP/v1/auth/pair" \
    -H 'Content-Type: application/json' \
    -d "{\"pairCode\":\"$PAIR_CODE\"}" \
    | python3 -c 'import sys,json; print(json.load(sys.stdin)["token"])')"
  printf '%s\n' "$tok" > "$TOKEN_FILE"
  chmod 600 "$TOKEN_FILE"
  echo "token refreshed → $TOKEN_FILE"
}

case "$cmd" in
  status)
    if [[ -f "$PID_FILE" ]] && is_alive "$(cat "$PID_FILE")"; then
      echo "master running pid=$(cat "$PID_FILE")"
    else
      echo "master not running"
    fi
    exit 0
    ;;
  stop)
    if [[ -f "$PID_FILE" ]]; then
      pid="$(cat "$PID_FILE")"
      if is_alive "$pid"; then
        echo "stopping master pid=$pid"
        kill "$pid" 2>/dev/null || true
        sleep 1
        kill -9 "$pid" 2>/dev/null || true
      fi
      rm -f "$PID_FILE"
    fi
    echo "stopped"
    exit 0
    ;;
  start) ;;
  *)
    echo "usage: $0 [start|status|stop]" >&2
    exit 2
    ;;
esac

if [[ ! -f "$CFG" ]]; then
  echo "missing $CFG" >&2
  exit 1
fi
if [[ ! -f "$ROOT/dist/master/cli.js" ]]; then
  echo "missing dist/; run: python run.py build --server --target linux-amd64 ./build" >&2
  exit 1
fi
if [[ -z "${CURSOR_API_KEY:-}" ]]; then
  echo "error: export CURSOR_API_KEY first (in this Terminal session)" >&2
  exit 1
fi

if [[ -f "$PID_FILE" ]] && is_alive "$(cat "$PID_FILE")"; then
  echo "master already running pid=$(cat "$PID_FILE")"
  echo "stop first: $0 stop"
  exit 1
fi

refresh_token
export GATEWAY_TOKEN="$(tr -d '\n' < "$TOKEN_FILE")"

# 脱离当前终端；Cursor 关掉也不影响
nohup node dist/master/cli.js serve --config master.config.yaml \
  >>"$LOG_FILE" 2>&1 &
echo $! > "$PID_FILE"
disown "$!" 2>/dev/null || true

sleep 1
if is_alive "$(cat "$PID_FILE")"; then
  echo "master started pid=$(cat "$PID_FILE")"
  echo "  config : $CFG"
  echo "  gateway: $GATEWAY_HTTP (pair $PAIR_CODE)"
  echo "  log    : $LOG_FILE"
  echo "  children: 不自动起 — App 舰队页 Start"
  echo "  stop   : $0 stop"
else
  echo "master failed to stay up; see $LOG_FILE" >&2
  exit 1
fi
