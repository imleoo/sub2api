# /dev — 本地开发环境管理

管理本地调试环境（后端 :8091，前端 :3091），封装 `script/dev_local.sh`。

## 用法

```
/dev [up|down|status|logs [backend|frontend|all]]
```

参数缺省时默认显示 `status`。

---

## 执行规则

### 参数解析

从用户输入中提取子命令（`up`/`down`/`status`/`logs`）和可选目标（`backend`/`frontend`/`all`）。
无参数时执行 `status`。

---

### `status`（默认）

```bash
cd /Users/leoobai/jiwu-project/SubPanel && ./script/dev_local.sh status
```

输出当前端口占用、PID、日志路径。

---

### `up`

**前置检查**：
```bash
# 检查端口是否已占用（说明已有实例运行）
lsof -nP -iTCP:8091 -sTCP:LISTEN 2>/dev/null && echo "PORT_IN_USE" || echo "PORT_FREE"
lsof -nP -iTCP:3091 -sTCP:LISTEN 2>/dev/null && echo "PORT_IN_USE" || echo "PORT_FREE"
```

- 端口已被占用 → 提示用户先运行 `/dev down`，不重复启动
- 端口空闲 → 继续

**启动**（run_in_background: true，超时 10s）：
```bash
cd /Users/leoobai/jiwu-project/SubPanel && ./script/dev_local.sh up
```

**验证**（等待 5 秒后检查）：
```bash
sleep 5 && cd /Users/leoobai/jiwu-project/SubPanel && ./script/dev_local.sh status
```

**成功输出**：
```
本地环境已启动
  后端:  http://127.0.0.1:8091
  前端:  http://127.0.0.1:3091
  日志:  /dev down 停止 | /dev logs 查看日志
```

---

### `down`

```bash
cd /Users/leoobai/jiwu-project/SubPanel && ./script/dev_local.sh down
```

执行后验证端口已释放：
```bash
lsof -nP -iTCP:8091 -sTCP:LISTEN 2>/dev/null | grep LISTEN && echo "仍有进程占用端口 8091" || echo "后端端口已释放"
lsof -nP -iTCP:3091 -sTCP:LISTEN 2>/dev/null | grep LISTEN && echo "仍有进程占用端口 3091" || echo "前端端口已释放"
```

---

### `logs [target]`

展示最近 100 行日志（不 follow，避免阻塞）：

```bash
LOG_DIR="/Users/leoobai/jiwu-project/SubPanel/.dev/local-debug/logs"

case "{target}" in
  backend)  tail -n 100 "$LOG_DIR/backend.log" ;;
  frontend) tail -n 100 "$LOG_DIR/frontend.log" ;;
  all|*)
    echo "=== 后端日志 ===" && tail -n 50 "$LOG_DIR/backend.log"
    echo "" && echo "=== 前端日志 ===" && tail -n 50 "$LOG_DIR/frontend.log"
    ;;
esac
```

如需持续跟踪，提示用户在终端执行：
```
! tail -f /Users/leoobai/jiwu-project/SubPanel/.dev/local-debug/logs/backend.log
```

---

## 环境变量覆盖（可在命令时传入）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `BACKEND_PORT` | 8091 | 后端端口 |
| `FRONTEND_PORT` | 3091 | 前端端口 |
| `START_FRONTEND` | true | 是否启动前端 |
| `DATABASE_PORT` | 59117 | PostgreSQL 端口 |
| `REDIS_PORT` | 59116 | Redis 端口 |

用法示例：`BACKEND_PORT=18091 ./script/dev_local.sh up`
