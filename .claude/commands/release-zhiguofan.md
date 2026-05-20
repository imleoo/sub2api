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

### Step 9：创建 GitHub Release（本地构建）

> **说明**：GitHub Actions 当前因账单问题无法运行，改由本地构建后手动上传。Actions 恢复后可改回触发 `release.yml` workflow。

#### 9.1 构建前端（如 dist 不存在或有变更）

```bash
cd frontend && pnpm run build && cd ..
```

#### 9.2 交叉编译五平台二进制

```bash
BUILD_DIR="/tmp/tokenpanel-release-${VERSION}"
rm -rf "$BUILD_DIR" && mkdir -p "$BUILD_DIR"
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-s -w -X main.Commit=${COMMIT} -X main.Date=${DATE} -X main.BuildType=release"

cd backend
for PLATFORM in "linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64"; do
  GOOS=${PLATFORM%/*} GOARCH=${PLATFORM#*/} CGO_ENABLED=0 \
    go build -tags=embed -ldflags="$LDFLAGS" \
    -o "$BUILD_DIR/tokenpanel_${PLATFORM%/*}_${PLATFORM#*/}" ./cmd/server/
done
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -tags=embed -ldflags="$LDFLAGS" \
  -o "$BUILD_DIR/tokenpanel_windows_amd64.exe" ./cmd/server/
cd ..
```

#### 9.3 打包并生成 checksums

```bash
cd "$BUILD_DIR"
for PLATFORM in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  tar -czf "tokenpanel_${VERSION}_${PLATFORM}.tar.gz" "tokenpanel_${PLATFORM}"
done
zip "tokenpanel_${VERSION}_windows_amd64.zip" "tokenpanel_windows_amd64.exe"
shasum -a 256 tokenpanel_${VERSION}_*.tar.gz tokenpanel_${VERSION}_windows_amd64.zip > checksums.txt
cd -
```

#### 9.4 创建 GitHub Release 并上传产物

```bash
gh release create "${TAG}" \
  --repo imleoo/tokenpanel \
  --title "TokenPanel ${TAG}" \
  --notes "基于上游 v0.${VERSION#1.}" \
  --latest

cd "$BUILD_DIR"
for f in tokenpanel_${VERSION}_*.tar.gz tokenpanel_${VERSION}_windows_amd64.zip checksums.txt; do
  gh release upload "${TAG}" "$f" --repo imleoo/tokenpanel --clobber
done
cd -
```

#### 9.5 构建并推送 Docker 镜像到 GHCR

```bash
# 将 linux/amd64 二进制复制到项目根（Dockerfile COPY 需要在 build context 内）
cp "$BUILD_DIR/tokenpanel_linux_amd64" ./tokenpanel

docker build \
  --platform linux/amd64 \
  -f Dockerfile.goreleaser \
  -t "ghcr.io/imleoo/tokenpanel:${VERSION}" \
  -t "ghcr.io/imleoo/tokenpanel:latest" \
  --label "org.opencontainers.image.version=${VERSION}" \
  --label "org.opencontainers.image.revision=${COMMIT}" \
  .

# 确认已登录 GHCR（需要 write:packages scope）
echo $(gh auth token) | docker login ghcr.io -u imleoo --password-stdin
docker push "ghcr.io/imleoo/tokenpanel:${VERSION}"
docker push "ghcr.io/imleoo/tokenpanel:latest"

# 清理
rm -f ./tokenpanel
rm -rf "$BUILD_DIR"
```

### Step 10：切回工作分支

```bash
git checkout feature/maas-refactor
```

---

## 完成后输出摘要

输出以下信息：

```
发布完成
  Tag    : v{VERSION}
  Branch : zhiguofan → origin/zhiguofan
  Release: https://github.com/imleoo/tokenpanel/releases/tag/v{VERSION}
  Docker : ghcr.io/imleoo/tokenpanel:{VERSION}
  Commit : {zhiguofan 当前 SHA}
  
后续步骤：
  · 若需继续开发，在 feature/maas-refactor 正常提交，下次运行 /release-zhiguofan 即可
  · 若上游有新版本，先运行 /sync-upstream 再开发
  · GitHub Actions 账单恢复后，可改用 release.yml workflow 替代本地构建
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
| gh release create 失败 | 中止，提示检查网络或 gh auth status |
| docker push 鉴权失败 | 提示运行 `gh auth refresh -s write:packages` 后重试 Step 9.5 |
