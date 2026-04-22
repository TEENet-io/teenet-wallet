#!/usr/bin/env bash
# Local dev orchestrator: brings up the mock TEENet service + the wallet
# with matching ports and WebAuthn origin, so Quick Start is one command.
#
#   ./scripts/dev.sh up      bring both services up (clones/builds as needed)
#   ./scripts/dev.sh down    stop both services
#   ./scripts/dev.sh status  show what's running
#   ./scripts/dev.sh logs    tail both log files
#
# Env overrides:
#   WALLET_PORT       wallet HTTP port (default 28080)
#   MOCK_PORT         mock-server port (default 18089)
#   APP_INSTANCE_ID   one of the mock server's app IDs (default mock-app-id-03)
#   SDK_DIR           path to teenet-sdk checkout (default ../teenet-sdk)
#   SDK_REPO          git URL used when SDK_DIR doesn't exist
#                     (default https://github.com/TEENet-io/teenet-sdk.git)
#   AUTO_PORT         set to 1 to auto-pick the next free port when the
#                     desired one is busy (default: die with a suggestion)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WALLET_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

: "${WALLET_PORT:=28080}"
: "${MOCK_PORT:=18089}"
: "${APP_INSTANCE_ID:=mock-app-id-03}"
: "${SDK_DIR:=$(cd "$WALLET_DIR/.." && pwd)/teenet-sdk}"
: "${SDK_REPO:=https://github.com/TEENet-io/teenet-sdk.git}"

DEV_DIR="$WALLET_DIR/.dev"
PID_MOCK="$DEV_DIR/mock.pid"
PID_WALLET="$DEV_DIR/wallet.pid"
LOG_MOCK="$DEV_DIR/mock.log"
LOG_WALLET="$DEV_DIR/wallet.log"
PORT_MOCK_FILE="$DEV_DIR/mock.port"
PORT_WALLET_FILE="$DEV_DIR/wallet.port"
DATA_DIR="$DEV_DIR/data"

# --- output helpers ---------------------------------------------------------

if [ -t 1 ]; then
  c_grn=$'\e[32m'; c_red=$'\e[31m'; c_yel=$'\e[33m'; c_rst=$'\e[0m'
else
  c_grn=; c_red=; c_yel=; c_rst=
fi
info() { printf '%s==>%s %s\n' "$c_grn" "$c_rst" "$*"; }
warn() { printf '%s==>%s %s\n' "$c_yel" "$c_rst" "$*" >&2; }
die()  { printf '%s==>%s %s\n' "$c_red" "$c_rst" "$*" >&2; exit 1; }

# --- helpers ----------------------------------------------------------------

is_running() {
  local pf=$1
  [ -f "$pf" ] || return 1
  if kill -0 "$(cat "$pf")" 2>/dev/null; then
    return 0
  fi
  # Stale pid file — pid file exists but process is gone.
  rm -f "$pf"
  return 1
}

port_busy() {
  # Returns 0 if something is listening on $1.
  if command -v ss >/dev/null 2>&1; then
    ss -ltn "sport = :$1" 2>/dev/null | grep -q ":$1\b"
  else
    lsof -iTCP:"$1" -sTCP:LISTEN -t >/dev/null 2>&1
  fi
}

find_free_port() {
  # Walks up from $1 looking for a free port, scans up to 20 ports.
  local p=$1
  for _ in $(seq 0 20); do
    if ! port_busy "$p"; then echo "$p"; return 0; fi
    p=$((p + 1))
  done
  return 1
}

resolve_port() {
  # In AUTO_PORT=1 mode, auto-picks the next free port when the desired one is
  # busy; otherwise dies. Reassigns the named variable in place.
  local name=$1
  local port=${!name}
  port_busy "$port" || return 0
  if [ "${AUTO_PORT:-0}" = "1" ]; then
    local new
    new=$(find_free_port $((port + 1))) \
      || die "$name=$port busy and no free port in the next 20"
    warn "$name=$port busy, auto-picked $new"
    printf -v "$name" '%s' "$new"
  else
    die "port $port already in use ($name); set $name=<other> or AUTO_PORT=1"
  fi
}

wait_healthy() {
  local url=$1 name=$2
  for _ in $(seq 1 30); do
    if curl -sf "$url" >/dev/null 2>&1; then info "$name healthy"; return 0; fi
    sleep 1
  done
  die "$name did not become healthy within 30s (check $DEV_DIR/*.log)"
}

# --- prereq checks ----------------------------------------------------------

check_prereqs() {
  command -v go >/dev/null || die "go not found (need 1.25+)"
  command -v git >/dev/null || die "git not found"
  command -v curl >/dev/null || die "curl not found"
  [ -f /usr/include/sqlite3.h ] || die \
"sqlite3 development headers missing — install one of:
  Debian/Ubuntu:  sudo apt-get install libsqlite3-dev
  RHEL/Fedora:    sudo dnf install sqlite-devel
  Alpine:         apk add sqlite-dev gcc musl-dev
  macOS:          xcode-select --install"
  command -v node >/dev/null || die "node not found (required for 'make frontend')"
  command -v npm  >/dev/null || die "npm not found (required for 'make frontend')"
}

# --- build steps ------------------------------------------------------------

ensure_sdk() {
  if [ ! -d "$SDK_DIR" ]; then
    info "Cloning teenet-sdk → $SDK_DIR"
    git clone --depth 1 "$SDK_REPO" "$SDK_DIR"
  fi
}

ensure_mock() {
  if [ ! -x "$SDK_DIR/mock-server/mock-server" ] \
    || [ "$SDK_DIR/mock-server/main.go" -nt "$SDK_DIR/mock-server/mock-server" ]; then
    info "Building mock-server"
    (cd "$SDK_DIR/mock-server" && make build)
  fi
}

ensure_wallet() {
  (cd "$WALLET_DIR" && git submodule update --init --recursive)
  if [ ! -f "$WALLET_DIR/frontend/index.html" ]; then
    info "Building frontend"
    (cd "$WALLET_DIR" && make frontend)
  fi
  if [ ! -x "$WALLET_DIR/teenet-wallet" ] \
    || [ "$WALLET_DIR/main.go" -nt "$WALLET_DIR/teenet-wallet" ]; then
    info "Building wallet"
    (cd "$WALLET_DIR" && make build)
  fi
}

# --- subcommands ------------------------------------------------------------

cmd_up() {
  mkdir -p "$DEV_DIR" "$DATA_DIR"

  check_prereqs
  ensure_sdk
  ensure_mock
  ensure_wallet

  # Resolve both ports up front: PASSKEY_RP_ORIGIN passed to mock-server must
  # match whatever port wallet actually lands on.
  local mock_needs_start=0 wallet_needs_start=0
  if is_running "$PID_MOCK"; then
    warn "mock-server already running (pid $(cat "$PID_MOCK"))"
  else
    resolve_port MOCK_PORT
    mock_needs_start=1
  fi
  if is_running "$PID_WALLET"; then
    warn "wallet already running (pid $(cat "$PID_WALLET"))"
  else
    resolve_port WALLET_PORT
    wallet_needs_start=1
  fi

  if [ "$mock_needs_start" = 1 ]; then
    info "Starting mock-server on :$MOCK_PORT"
    (
      cd "$SDK_DIR/mock-server"
      PASSKEY_RP_ID=localhost \
      PASSKEY_RP_ORIGIN="http://localhost:$WALLET_PORT" \
      MOCK_SERVER_PORT="$MOCK_PORT" \
        nohup ./mock-server >"$LOG_MOCK" 2>&1 &
      echo $! >"$PID_MOCK"
    )
    echo "$MOCK_PORT" >"$PORT_MOCK_FILE"
  else
    MOCK_PORT=$(cat "$PORT_MOCK_FILE" 2>/dev/null || echo "$MOCK_PORT")
  fi

  if [ "$wallet_needs_start" = 1 ]; then
    info "Starting wallet on :$WALLET_PORT"
    (
      cd "$WALLET_DIR"
      PORT="$WALLET_PORT" \
      APP_INSTANCE_ID="$APP_INSTANCE_ID" \
      DATA_DIR="$DATA_DIR" \
      SERVICE_URL="http://127.0.0.1:$MOCK_PORT" \
        nohup ./teenet-wallet >"$LOG_WALLET" 2>&1 &
      echo $! >"$PID_WALLET"
    )
    echo "$WALLET_PORT" >"$PORT_WALLET_FILE"
  else
    WALLET_PORT=$(cat "$PORT_WALLET_FILE" 2>/dev/null || echo "$WALLET_PORT")
  fi

  wait_healthy "http://127.0.0.1:$MOCK_PORT/api/health"   "mock-server"
  wait_healthy "http://127.0.0.1:$WALLET_PORT/api/health" "wallet"

  cat <<EOF

${c_grn}Services are up.${c_rst}

  Mock    http://127.0.0.1:$MOCK_PORT   (log: $LOG_MOCK)
  Wallet  http://localhost:$WALLET_PORT (log: $LOG_WALLET)
  Data    $DATA_DIR

Next steps (WebAuthn requires localhost or HTTPS):

  1. If running remotely:
     ssh -L $WALLET_PORT:localhost:$WALLET_PORT <user@server>

  2. Open http://localhost:$WALLET_PORT in a browser.

  3. Register: any email, code 999999, then Passkey.

  4. Settings → generate an API key (starts with ocw_, shown once).

  5. When done: ./scripts/dev.sh down
EOF
}

cmd_down() {
  local stopped=0
  for pair in "$PID_WALLET:$PORT_WALLET_FILE" "$PID_MOCK:$PORT_MOCK_FILE"; do
    IFS=: read -r pf port_file <<<"$pair"
    [ -f "$pf" ] || { rm -f "$port_file"; continue; }
    pid=$(cat "$pf")
    if kill -0 "$pid" 2>/dev/null; then
      info "Stopping $(basename "$pf" .pid) (pid $pid)"
      kill "$pid" 2>/dev/null || true
      stopped=1
    fi
    rm -f "$pf" "$port_file"
  done
  [ "$stopped" -eq 1 ] || info "Nothing to stop"
}

cmd_status() {
  printf '%-10s %-10s %-10s %s\n' "SERVICE" "STATE" "PORT" "PID"
  for quad in "mock:$PID_MOCK:$PORT_MOCK_FILE:$MOCK_PORT" "wallet:$PID_WALLET:$PORT_WALLET_FILE:$WALLET_PORT"; do
    IFS=: read -r name pf port_file fallback <<<"$quad"
    # Port shown is whatever the service was started on (from port file), falling
    # back to the current env default when the service has never run.
    local port; port=$(cat "$port_file" 2>/dev/null || echo "$fallback")
    if is_running "$pf"; then
      printf '%-10s %s%-10s%s %-10s %s\n' "$name" "$c_grn" "RUNNING" "$c_rst" "$port" "$(cat "$pf")"
    else
      printf '%-10s %s%-10s%s %-10s %s\n' "$name" "$c_yel" "STOPPED" "$c_rst" "$port" "-"
    fi
  done
}

cmd_logs() {
  if [ ! -f "$LOG_MOCK" ] && [ ! -f "$LOG_WALLET" ]; then
    die "no log files yet (try: ./scripts/dev.sh up)"
  fi
  # -F follows across file rotation; -n100 shows recent context
  exec tail -F -n 100 "$LOG_MOCK" "$LOG_WALLET"
}

usage() {
  cat <<EOF
usage: $(basename "$0") {up|down|status|logs}

  up      bring mock-server + wallet up (clones/builds as needed)
  down    stop both services
  status  show which services are running
  logs    tail mock + wallet logs
EOF
  exit 2
}

case "${1:-up}" in
  up|start)    cmd_up ;;
  down|stop)   cmd_down ;;
  status)      cmd_status ;;
  logs|log)    cmd_logs ;;
  -h|--help|help) usage ;;
  *) warn "unknown subcommand: $1"; usage ;;
esac
