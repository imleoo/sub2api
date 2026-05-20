# /push-internal — 推送 zhiguofan 到内网 Git

将 `zhiguofan` 分支推送到内网生产 Git 服务器。
目标：`ssh://git_prod_backend@192.168.1.10/home/git_prod_backend/wmtoken_platform.git`

**执行前必须确认已切换到内网环境（VPN 或局域网）。**

---

## 执行步骤

### Step 1：预检

```bash
git branch --show-current   # 必须是 zhiguofan
git status --short          # 工作区必须干净
```

- 当前分支不是 `zhiguofan` → 中止，提示：`git checkout zhiguofan`
- 工作区有未提交内容 → 中止，提示先 stash 或 commit

### Step 2：网络连通性检查

```bash
nc -z -w 5 192.168.1.10 22 2>/dev/null && echo "SSH_REACHABLE" || echo "SSH_UNREACHABLE"
```

- `SSH_UNREACHABLE` → 中止，提示：当前无法访问内网服务器，请检查 VPN 或网络环境

### Step 3：执行推送

```bash
cd /Users/leoobai/jiwu-project/SubPanel && ./script/push_zhiguofan_to_internal_git.sh
```

### Step 4：验证

推送成功后显示：

```
内网推送完成
  分支: zhiguofan
  远端: ssh://git_prod_backend@192.168.1.10/home/git_prod_backend/wmtoken_platform.git
  Commit: {当前 SHA}
```

推送失败时直接输出错误信息，不做重试。常见原因：
- SSH key 未授权 → 检查 `~/.ssh/authorized_keys`
- 内网不通 → 检查 VPN
- 远端分支保护 → 联系运维

---

## 中止条件

| 条件 | 动作 |
|------|------|
| 当前分支不是 zhiguofan | 中止 |
| 工作区有未提交改动 | 中止 |
| 内网 SSH 不可达 | 中止 |
