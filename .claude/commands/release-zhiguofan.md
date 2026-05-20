# /release-zhiguofan — maas-refactor 发布到 zhiguofan

将已开发验证完成的 `feature/maas-refactor` 合并到 `zhiguofan`，打版本 tag，并推送。
**执行前必须确认代码已通过人工验证，本命令不替代功能测试。**

## 版本约定

fork 版本号规则：上游 `0.x.y` → 本 fork `1.x.y`（主号固定为 1）。
tag 格式：`v1.x.y`，与 `backend/cmd/server/VERSION` 保持一致。
**一个上游版本对应一个 tag**；若同一上游版本已有 tag，说明需要先同步上游再发布。

---

## 执行步骤

### Step 1：预检

运行以下命令，确认发布前提：

```bash
git branch --show-current          # 必须在 feature/maas-refactor
git status --short                 # 工作区必须干净（无未提交改动）
cat backend/cmd/server/VERSION     # 读取当前 fork 版本
git show upstream/main:backend/cmd/server/VERSION 2>/dev/null  # 上游最新版本
```

**检查要点**：
- 当前分支必须是 `feature/maas-refactor`，否则中止
- 工作区有未提交改动时，提示用户先 stash 或 commit，否则中止
- 对比 fork 版本与上游版本：若上游版本 `0.x.y` 对应的 `1.x.y` 与当前 VERSION 不匹配，说明 VERSION 文件未跟上上游，需先运行 `/sync-upstream`

### Step 2：检查 tag 是否已存在

```bash
VERSION=$(cat backend/cmd/server/VERSION)
TAG="v${VERSION}"
git tag --list "$TAG"              # 空输出说明 tag 不存在
git log --oneline "$TAG" 2>/dev/null | head -1  # 如存在，显示对应 commit
```

**检查要点**：
- 若 tag `v{VERSION}` 已存在，说明本版本已发布过。**中止并告知用户**：
  - 如需修复 bug 后重新发布，应先与上游同步（获得新版本号 `0.x.{y+1}`）再重新运行本命令
  - 不能复用已有 tag 指向不同 commit（破坏版本可追溯性）

### Step 3：构建验证

```bash
cd backend && go build ./cmd/server/ 2>&1
```

build 失败则立即中止，不进行后续步骤。

### Step 4：fork 守护检查

```bash
bash script/check_fork12_guards.sh
```

任何 `[FAIL]` 则中止，输出失败的守护规则。额外检查：

```bash
grep -q 'ProtocolBucketEnabled' backend/internal/config/config.go && echo "OK: P5 回滚开关" || echo "MISSING"
grep -q 'upstream_total_cost'   backend/ent/schema/usage_log.go   && echo "OK: P0 9列" || echo "MISSING"
grep -A2 "^on:" .github/workflows/*.yml | grep -v "workflow_dispatch" | grep -v "^--$" | grep -v "^$" \
  && echo "WARNING: 发现非 workflow_dispatch 触发器！" || echo "OK: workflows 触发器安全"
```

### Step 5：关键单元测试

```bash
cd backend && go test -tags=unit ./internal/service/... ./internal/handler/... ./internal/repository/... 2>&1 | tail -15
cd frontend && pnpm exec vitest run \
  src/views/user/__tests__/ModelsView.spec.ts \
  src/i18n/__tests__/usageServiceTierLocales.spec.ts 2>&1 | tail -10
```

测试失败则中止。

### Step 6：合并 maas-refactor → zhiguofan

```bash
git checkout zhiguofan
git merge feature/maas-refactor --ff-only 2>/dev/null \
  || git merge feature/maas-refactor --no-edit -m "release: merge feature/maas-refactor into zhiguofan (${TAG})"
```

**说明**：
- 优先尝试 fast-forward（maas-refactor 是 zhiguofan 的严格超集时适用）
- fast-forward 失败说明 zhiguofan 有独立提交，自动回落到 merge commit
- 若产生冲突，中止并提示用户手动解决后重新运行

### Step 7：打 tag

```bash
git tag -a "${TAG}" -m "release ${TAG}

fork version: ${TAG} (upstream base: 0.${VERSION#1.})
branch: zhiguofan
date: $(date '+%Y-%m-%d')"
```

**说明**：使用带注释的 annotated tag（-a），注释中记录对应的上游版本。

### Step 8：推送

```bash
git push origin zhiguofan
git push origin "${TAG}"
```

推送失败时报告原因，不回滚本地状态（本地 tag 和分支已是干净状态，可重试 push）。

### Step 9：切回工作分支

```bash
git checkout feature/maas-refactor
```

---

## 完成后输出摘要

输出以下信息：

```
发布完成
  Tag   : v{VERSION}
  Branch: zhiguofan → origin/zhiguofan
  Commit: {zhiguofan 当前 SHA}
  
后续步骤：
  · 若需部署，在 GitHub Actions 手动触发 docker-push.yml，选择 zhiguofan 分支
  · 若需继续开发，在 feature/maas-refactor 正常提交，下次运行 /release-zhiguofan 即可
  · 若上游有新版本，先运行 /sync-upstream 再开发
```

---

## 中止条件汇总

| 条件 | 动作 |
|------|------|
| 当前分支不是 feature/maas-refactor | 中止，说明需要先 checkout |
| 工作区有未提交改动 | 中止，提示 stash 或 commit |
| VERSION 与上游不匹配 | 中止，提示先运行 /sync-upstream |
| tag 已存在 | 中止，说明需要先同步上游获得新版本号 |
| go build 失败 | 中止 |
| fork 守护检查失败 | 中止，列出失败项 |
| 单元测试失败 | 中止 |
| merge 产生冲突 | 中止，提示手动解决 |
