# 脚本说明

这个目录里放了上游同步与本地调试脚本。

## 1. `sync_upstream_to_zhiguofan.sh`

作用：

- 拉取上游 `upstream/main`
- 合并到本地 `main`
- 推送 `main` 到 `origin`
- 再把 `origin/main` 合并到 `zhiguofan`
- 将 `zhiguofan` 的版本号同步为 `1.x.y`（默认把 `main` 的 `0.x.y` 改成 `1.x.y`）
- 最后推送 `zhiguofan` 到 `origin`

适用场景：

- 你在 GitHub / 上游网络环境下做代码同步
- 希望本地 `main` 始终和上游保持一致

## 冲突处理原则

- `main` 和上游冲突时，以上游为准，`main` 负责跟上游对齐
- `zhiguofan` 和 `main` 冲突时，以 `main` 为基础，在其上重新保留 `zhiguofan` 的改动
- 合并冲突时，脚本会停止并提示你执行以下命令：
```bash
git add <files> && git merge --continue
git merge --abort
```
- 脚本默认不会让 Codebuddy 自动改冲突文件；如确实要试用 AI 辅助冲突处理，需要显式执行：
```bash
AI_RESOLVE=true ./script/sync_upstream_to_zhiguofan.sh
```

## 2. `dev_local.sh`

作用：

- 启动本地调试环境
- 连接你本机已经在 Docker 里运行的 PostgreSQL 和 Redis
- 启动后端 `go run ./cmd/server`
- 启动前端 `pnpm dev`
- 把运行数据隔离到 `.dev/local-debug/`

常用命令：

```bash
# 启动整套本地调试环境
./script/dev_local.sh up

# 查看状态
./script/dev_local.sh status

# 查看日志
./script/dev_local.sh logs

# 停止调试环境
./script/dev_local.sh down
```

本地调试默认使用：

- 后端：`http://127.0.0.1:8082`
- 前端：`http://127.0.0.1:3002`
- PostgreSQL：`127.0.0.1:5432`
- Redis：`127.0.0.1:6379`

如果端口冲突，可以在执行前覆盖环境变量，例如：

```bash
BACKEND_PORT=18082 FRONTEND_PORT=13002 POSTGRES_PORT=15432 REDIS_PORT=16379 ./script/dev_local.sh up
```

调试数据和日志会放在：

- `.dev/local-debug/backend-data`
- `.dev/local-debug/logs`

脚本会优先从当前运行的 Docker 容器里读取 PostgreSQL / Redis 凭据，默认容器名是：

- `tokenpanel-postgres-dev`
- `tokenpanel-redis-dev`

如果你的容器名称不同，或者你想手动覆盖账号密码，请编辑 `./.dev/local-debug/dev.env` 里的：

- `DATABASE_USER`
- `DATABASE_PASSWORD`
- `REDIS_PASSWORD`
