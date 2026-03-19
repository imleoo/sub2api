#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
DEV_DIR="$ROOT_DIR/.dev/local-debug"
DATA_DIR="$DEV_DIR/backend-data"
ENV_FILE="$DEV_DIR/dev.env"
LOG_DIR="$DEV_DIR/logs"
PID_DIR="$DEV_DIR/pids"
BACKEND_LOG="$LOG_DIR/backend.log"
FRONTEND_LOG="$LOG_DIR/frontend.log"
BACKEND_PID_FILE="$PID_DIR/backend.pid"
FRONTEND_PID_FILE="$PID_DIR/frontend.pid"
TAIL_PID_FILE="$PID_DIR/tail.pid"

BACKEND_HOST="${BACKEND_HOST:-127.0.0.1}"
BACKEND_PORT="${BACKEND_PORT:-8082}"
FRONTEND_HOST="${FRONTEND_HOST:-127.0.0.1}"
FRONTEND_PORT="${FRONTEND_PORT:-3002}"

DATABASE_HOST="${DATABASE_HOST:-127.0.0.1}"
DATABASE_PORT="${DATABASE_PORT:-5432}"
DATABASE_USER="${DATABASE_USER:-sub2api}"
DATABASE_PASSWORD="${DATABASE_PASSWORD:-sub2api}"
DATABASE_DBNAME="${DATABASE_DBNAME:-sub2api}"
DATABASE_SSLMODE="${DATABASE_SSLMODE:-disable}"

REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"
REDIS_DB="${REDIS_DB:-0}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@sub2api.local}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123}"
JWT_SECRET="${JWT_SECRET:-}"
TOTP_ENCRYPTION_KEY="${TOTP_ENCRYPTION_KEY:-}"

POSTGRES_WAIT_TIMEOUT="${POSTGRES_WAIT_TIMEOUT:-120}"
REDIS_WAIT_TIMEOUT="${REDIS_WAIT_TIMEOUT:-120}"
START_FRONTEND="${START_FRONTEND:-true}"

read_container_env() {
  local container="$1"
  local key="$2"
  docker inspect "$container" --format '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
    | awk -F= -v key="$key" '$1 == key {sub("^[^=]+=", "", $0); print; exit}'
}

detect_docker_credentials() {
  local postgres_container=""
  local redis_container=""

  for candidate in sub2api-postgres-dev sub2api-postgres; do
    if docker inspect "$candidate" >/dev/null 2>&1; then
      postgres_container="$candidate"
      break
    fi
  done

  if [[ -n "$postgres_container" ]]; then
    local value
    value="$(read_container_env "$postgres_container" POSTGRES_USER)"
    [[ -n "$value" ]] && DATABASE_USER="$value"
    value="$(read_container_env "$postgres_container" POSTGRES_PASSWORD)"
    [[ -n "$value" ]] && DATABASE_PASSWORD="$value"
    value="$(read_container_env "$postgres_container" POSTGRES_DB)"
    [[ -n "$value" ]] && DATABASE_DBNAME="$value"
  fi

  for candidate in sub2api-redis-dev sub2api-redis; do
    if docker inspect "$candidate" >/dev/null 2>&1; then
      redis_container="$candidate"
      break
    fi
  done

  if [[ -n "$redis_container" ]]; then
    local value
    value="$(read_container_env "$redis_container" REDIS_PASSWORD)"
    REDIS_PASSWORD="${value:-}"
  fi
}

require_port_free() {
  local port="$1"
  local label="$2"

  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      die "${label} 端口 ${port} 已被占用，请先释放它或通过环境变量改端口。"
    fi
    return 0
  fi

  if command -v nc >/dev/null 2>&1; then
    if nc -z 127.0.0.1 "$port" >/dev/null 2>&1; then
      die "${label} 端口 ${port} 已被占用，请先释放它或通过环境变量改端口。"
    fi
  fi
}

die() {
  echo "error: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

generate_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
    return 0
  fi

  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr -d '-'
    return 0
  fi

  die "Unable to generate a secret. Please install openssl or uuidgen."
}

initialize_env() {
  mkdir -p "$DEV_DIR"
  mkdir -p "$DATA_DIR"

  if [[ -f "$ENV_FILE" ]]; then
    # shellcheck disable=SC1090
    set -a
    source "$ENV_FILE"
    set +a
  fi

  if command -v docker >/dev/null 2>&1; then
    detect_docker_credentials
  fi

  if [[ -z "${JWT_SECRET:-}" ]]; then
    JWT_SECRET="$(generate_secret)"
  fi
  if [[ -z "${TOTP_ENCRYPTION_KEY:-}" ]]; then
    TOTP_ENCRYPTION_KEY="$(generate_secret)"
  fi

  export AUTO_SETUP=true
  export DATA_DIR="$DATA_DIR"
  export SERVER_HOST="$BACKEND_HOST"
  export SERVER_PORT="$BACKEND_PORT"
  export SERVER_MODE=debug
  export RUN_MODE=standard
  export TZ=Asia/Shanghai

  export DATABASE_HOST="$DATABASE_HOST"
  export DATABASE_PORT="$DATABASE_PORT"
  export DATABASE_USER="$DATABASE_USER"
  export DATABASE_PASSWORD="$DATABASE_PASSWORD"
  export DATABASE_DBNAME="$DATABASE_DBNAME"
  export DATABASE_SSLMODE="$DATABASE_SSLMODE"

  export REDIS_HOST="$REDIS_HOST"
  export REDIS_PORT="$REDIS_PORT"
  export REDIS_PASSWORD="$REDIS_PASSWORD"
  export REDIS_DB="$REDIS_DB"

  export ADMIN_EMAIL="$ADMIN_EMAIL"
  export ADMIN_PASSWORD="$ADMIN_PASSWORD"
  export JWT_SECRET="$JWT_SECRET"
  export TOTP_ENCRYPTION_KEY="$TOTP_ENCRYPTION_KEY"

  export VITE_DEV_PROXY_TARGET="http://${BACKEND_HOST}:${BACKEND_PORT}"
  export VITE_DEV_PORT="$FRONTEND_PORT"

  {
    printf 'export AUTO_SETUP=true\n'
    printf 'export DATA_DIR=%q\n' "$DATA_DIR"
    printf 'export SERVER_HOST=%q\n' "$BACKEND_HOST"
    printf 'export SERVER_PORT=%q\n' "$BACKEND_PORT"
    printf 'export SERVER_MODE=debug\n'
    printf 'export RUN_MODE=standard\n'
    printf 'export TZ=Asia/Shanghai\n'
    printf '\n'
    printf 'export DATABASE_HOST=%q\n' "$DATABASE_HOST"
    printf 'export DATABASE_PORT=%q\n' "$DATABASE_PORT"
    printf 'export DATABASE_USER=%q\n' "$DATABASE_USER"
    printf 'export DATABASE_PASSWORD=%q\n' "$DATABASE_PASSWORD"
    printf 'export DATABASE_DBNAME=%q\n' "$DATABASE_DBNAME"
    printf 'export DATABASE_SSLMODE=%q\n' "$DATABASE_SSLMODE"
    printf '\n'
    printf 'export REDIS_HOST=%q\n' "$REDIS_HOST"
    printf 'export REDIS_PORT=%q\n' "$REDIS_PORT"
    printf 'export REDIS_PASSWORD=%q\n' "$REDIS_PASSWORD"
    printf 'export REDIS_DB=%q\n' "$REDIS_DB"
    printf '\n'
    printf 'export ADMIN_EMAIL=%q\n' "$ADMIN_EMAIL"
    printf 'export ADMIN_PASSWORD=%q\n' "$ADMIN_PASSWORD"
    printf 'export JWT_SECRET=%q\n' "$JWT_SECRET"
    printf 'export TOTP_ENCRYPTION_KEY=%q\n' "$TOTP_ENCRYPTION_KEY"
    printf '\n'
    printf 'export VITE_DEV_PROXY_TARGET=%q\n' "$VITE_DEV_PROXY_TARGET"
    printf 'export VITE_DEV_PORT=%q\n' "$VITE_DEV_PORT"
  } >"$ENV_FILE"
}

wait_for_postgres() {
  local timeout="$1"
  local elapsed=0
  while true; do
    if command -v pg_isready >/dev/null 2>&1; then
      if pg_isready -h "$DATABASE_HOST" -p "$DATABASE_PORT" -U "$DATABASE_USER" >/dev/null 2>&1; then
        return 0
      fi
    else
      if (exec 3<>"/dev/tcp/${DATABASE_HOST}/${DATABASE_PORT}") >/dev/null 2>&1; then
        exec 3>&- 3<&-
        return 0
      fi
    fi

    if (( elapsed >= timeout )); then
      die "PostgreSQL 等待超时：${DATABASE_HOST}:${DATABASE_PORT}"
    fi

    sleep 2
    elapsed=$((elapsed + 2))
  done
}

wait_for_redis() {
  local timeout="$1"
  local elapsed=0
  while true; do
    if command -v redis-cli >/dev/null 2>&1; then
      if [[ -n "$REDIS_PASSWORD" ]]; then
        if REDISCLI_AUTH="$REDIS_PASSWORD" redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -n "$REDIS_DB" ping >/dev/null 2>&1; then
          return 0
        fi
      else
        if redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -n "$REDIS_DB" ping >/dev/null 2>&1; then
          return 0
        fi
      fi
    else
      if (exec 3<>"/dev/tcp/${REDIS_HOST}/${REDIS_PORT}") >/dev/null 2>&1; then
        exec 3>&- 3<&-
        return 0
      fi
    fi

    if (( elapsed >= timeout )); then
      die "Redis 等待超时：${REDIS_HOST}:${REDIS_PORT}"
    fi

    sleep 2
    elapsed=$((elapsed + 2))
  done
}

ensure_frontend_deps() {
  if [[ -d "$ROOT_DIR/frontend/node_modules" ]]; then
    return 0
  fi

  echo "检测到前端依赖缺失，正在执行 pnpm install..."
  pnpm --dir "$ROOT_DIR/frontend" install
}

start_backend() {
  mkdir -p "$LOG_DIR" "$PID_DIR"
  if [[ -f "$BACKEND_PID_FILE" ]] && kill -0 "$(cat "$BACKEND_PID_FILE")" >/dev/null 2>&1; then
    die "后端似乎已经在运行，PID=$(cat "$BACKEND_PID_FILE")"
  fi

  echo "启动后端..."
  (
    cd "$ROOT_DIR/backend"
    env \
      AUTO_SETUP=true \
      DATA_DIR="$DATA_DIR" \
      DATABASE_HOST="$DATABASE_HOST" \
      DATABASE_PORT="$DATABASE_PORT" \
      DATABASE_USER="$DATABASE_USER" \
      DATABASE_PASSWORD="$DATABASE_PASSWORD" \
      DATABASE_DBNAME="$DATABASE_DBNAME" \
      DATABASE_SSLMODE="$DATABASE_SSLMODE" \
      REDIS_HOST="$REDIS_HOST" \
      REDIS_PORT="$REDIS_PORT" \
      REDIS_PASSWORD="$REDIS_PASSWORD" \
      REDIS_DB="$REDIS_DB" \
      SERVER_HOST="$BACKEND_HOST" \
      SERVER_PORT="$BACKEND_PORT" \
      SERVER_MODE=debug \
      RUN_MODE=standard \
      ADMIN_EMAIL=admin@sub2api.local \
      ADMIN_PASSWORD=admin123 \
      JWT_SECRET="$JWT_SECRET" \
      TOTP_ENCRYPTION_KEY="$TOTP_ENCRYPTION_KEY" \
      TZ=Asia/Shanghai \
      go run ./cmd/server
  ) >"$BACKEND_LOG" 2>&1 &
  echo $! >"$BACKEND_PID_FILE"
}

start_frontend() {
  if [[ "${START_FRONTEND}" != "true" ]]; then
    echo "已跳过前端启动（START_FRONTEND=${START_FRONTEND}）"
    return 0
  fi

  mkdir -p "$LOG_DIR" "$PID_DIR"
  if [[ -f "$FRONTEND_PID_FILE" ]] && kill -0 "$(cat "$FRONTEND_PID_FILE")" >/dev/null 2>&1; then
    die "前端似乎已经在运行，PID=$(cat "$FRONTEND_PID_FILE")"
  fi

  echo "启动前端..."
  (
    cd "$ROOT_DIR/frontend"
    env \
      VITE_DEV_PROXY_TARGET="http://${BACKEND_HOST}:${BACKEND_PORT}" \
      VITE_DEV_PORT="$FRONTEND_PORT" \
      pnpm exec vite --strictPort
  ) >"$FRONTEND_LOG" 2>&1 &
  echo $! >"$FRONTEND_PID_FILE"
}

start_log_tail() {
  if [[ "${START_FRONTEND}" == "true" ]]; then
    tail -n +1 -F "$BACKEND_LOG" "$FRONTEND_LOG" &
  else
    tail -n +1 -F "$BACKEND_LOG" &
  fi
  echo $! >"$TAIL_PID_FILE"
}

stop_pid_file() {
  local pid_file="$1"
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file")"
    if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
    rm -f "$pid_file"
  fi
}

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM

  stop_pid_file "$TAIL_PID_FILE"
  stop_pid_file "$FRONTEND_PID_FILE"
  stop_pid_file "$BACKEND_PID_FILE"

  exit "$exit_code"
}

status() {
  initialize_env

  echo "调试目录: $DEV_DIR"
  echo "后端日志: $BACKEND_LOG"
  echo "前端日志: $FRONTEND_LOG"
  echo
  echo "后端端口: ${BACKEND_HOST}:${BACKEND_PORT}"
  echo "前端端口: ${FRONTEND_HOST}:${FRONTEND_PORT}"
  echo "数据库:   ${DATABASE_HOST}:${DATABASE_PORT}"
  echo "Redis:     ${REDIS_HOST}:${REDIS_PORT}"
  echo "数据库账号: ${DATABASE_USER}"
  echo

  if [[ -f "$BACKEND_PID_FILE" ]]; then
    echo "后端 PID: $(cat "$BACKEND_PID_FILE")"
  else
    echo "后端 PID: 未记录"
  fi

  if [[ -f "$FRONTEND_PID_FILE" ]]; then
    echo "前端 PID: $(cat "$FRONTEND_PID_FILE")"
  else
    echo "前端 PID: 未记录"
  fi
}

logs() {
  initialize_env

  local target="${1:-all}"
  case "$target" in
    backend)
      tail -n +1 -F "$BACKEND_LOG"
      ;;
    frontend)
      if [[ "${START_FRONTEND}" != "true" ]]; then
        die "当前未启用前端（START_FRONTEND=${START_FRONTEND}）"
      fi
      tail -n +1 -F "$FRONTEND_LOG"
      ;;
    all)
      if [[ "${START_FRONTEND}" == "true" ]]; then
        tail -n +1 -F "$BACKEND_LOG" "$FRONTEND_LOG"
      else
        tail -n +1 -F "$BACKEND_LOG"
      fi
      ;;
    *)
      die "未知日志目标: $target (可选: backend, frontend, all)"
      ;;
  esac
}

down() {
  initialize_env

  stop_pid_file "$TAIL_PID_FILE"
  stop_pid_file "$FRONTEND_PID_FILE"
  stop_pid_file "$BACKEND_PID_FILE"
  echo "本地调试进程已停止。"
}

up() {
  require_cmd go
  require_cmd pnpm
  require_cmd tail
  require_cmd docker

  mkdir -p "$LOG_DIR" "$PID_DIR"
  initialize_env

  require_port_free "$BACKEND_PORT" "后端"
  if [[ "${START_FRONTEND}" == "true" ]]; then
    require_port_free "$FRONTEND_PORT" "前端"
  fi

  echo "等待本机 PostgreSQL / Redis 就绪..."
  wait_for_postgres "$POSTGRES_WAIT_TIMEOUT"
  wait_for_redis "$REDIS_WAIT_TIMEOUT"

  ensure_frontend_deps
  touch "$BACKEND_LOG" "$FRONTEND_LOG"

  start_backend
  start_frontend
  start_log_tail

  trap cleanup EXIT INT TERM

  echo "本地调试已启动。"
  echo "后端:  http://${BACKEND_HOST}:${BACKEND_PORT}"
  if [[ "${START_FRONTEND}" == "true" ]]; then
    echo "前端:  http://${FRONTEND_HOST}:${FRONTEND_PORT}"
  fi
  echo "数据库: ${DATABASE_HOST}:${DATABASE_PORT}"
  echo "Redis:   ${REDIS_HOST}:${REDIS_PORT}"
  echo "日志:   tail -f $BACKEND_LOG"
  if [[ "${START_FRONTEND}" == "true" ]]; then
    echo "        tail -f $FRONTEND_LOG"
  fi
  echo "停止:   按 Ctrl-C，或执行 $0 down"
  echo

  while true; do
    if ! kill -0 "$(cat "$BACKEND_PID_FILE")" >/dev/null 2>&1; then
      wait "$(cat "$BACKEND_PID_FILE")" || true
      echo "后端已退出，正在清理..."
      exit 1
    fi

    if [[ "${START_FRONTEND}" == "true" ]] && ! kill -0 "$(cat "$FRONTEND_PID_FILE")" >/dev/null 2>&1; then
      wait "$(cat "$FRONTEND_PID_FILE")" || true
      echo "前端已退出，正在清理..."
      exit 1
    fi

    sleep 1
  done
}

main() {
  local cmd="${1:-up}"
  case "$cmd" in
    up)
      up
      ;;
    down)
      down
      ;;
    status)
      status
      ;;
    logs)
      logs "${2:-all}"
      ;;
    help|-h|--help)
      cat <<EOF
用法:
  $0 up        启动后端 / 前端
  $0 down      停止本地调试进程
  $0 status    查看当前状态
  $0 logs      查看日志，默认 all

默认会连接你本机已有的 PostgreSQL / Redis 容器：
  DATABASE_HOST=127.0.0.1
  DATABASE_PORT=5432
  REDIS_HOST=127.0.0.1
  REDIS_PORT=6379

可用环境变量:
  BACKEND_PORT=8082
  FRONTEND_PORT=3002
  DATABASE_PASSWORD=your_password_here
  REDIS_PASSWORD=
  START_FRONTEND=true
EOF
      ;;
    *)
      die "未知命令: $cmd"
      ;;
  esac
}

main "$@"
