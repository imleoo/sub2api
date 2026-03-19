# 脚本说明

这个目录里放了两个彼此独立的 Git 脚本，方便你按网络环境分别执行。

## 1. `sync_upstream_to_zhiguofan.sh`

作用：

- 拉取上游 `upstream/main`
- 合并到本地 `main`
- 推送 `main` 到 `origin`
- 再把 `origin/main` 合并到 `zhiguofan`
- 最后推送 `zhiguofan` 到 `origin`

适用场景：

- 你在 GitHub / 上游网络环境下做代码同步
- 希望本地 `main` 始终和上游保持一致

## 2. `push_zhiguofan_to_internal_git.sh`

作用：

- 仅把本地 `zhiguofan` 分支推送到内网 Git 服务器

内网地址：

- `ssh://git_prod_backend@192.168.1.10/home/git_prod_backend/wmtoken_platform.git`

适用场景：

- 你切到内网网络后，只做最终推送
- 不影响 GitHub / 上游那边的同步流程

## 冲突处理原则

- `main` 和上游冲突时，以上游为准，`main` 负责跟上游对齐
- `zhiguofan` 和 `main` 冲突时，以 `main` 为基础，在其上重新保留 `zhiguofan` 的改动
- 合并冲突时，脚本会停止并提示你执行：
  - `git add <files> && git merge --continue`
  - `git merge --abort`

