# 合并 main 到 dev-zz

这页记录把上游 `main` 合并到 `dev-zz` 的标准流程。默认比较对象是 `origin/main`，避免本地 `main` 停在旧提交上，导致误判合并范围。

## 目标

- 吸收上游 `main` 的修复和功能。
- 保留 dev-zz 已记录的二开策略。
- 记录冲突、取舍和验证命令。

## 前置检查

```bash
git switch dev-zz
git status --short --branch
git fetch origin
```

确认本地 `main` 与目标上游一致。如果只是预检，也可以直接以 `origin/main` 为比较对象，不必切换本地 `main`。

```bash
git switch main
git pull --ff-only origin main
git switch dev-zz
```

## 读取上下文

合并前先阅读：

- [dev-zz 分支策略](../branch-policy.md)
- [补丁记录](../patches.md)
- [上游合并记录](./merge-log.md)
- [变更记录](../changelog.md)
- [dev-zz 变更地图](../reference/change-map.md)
- [验证矩阵](../testing/verification-matrix.md)

## 预判冲突

```bash
git merge-tree --write-tree "$(git merge-base HEAD origin/main)" HEAD origin/main
```

这一步是只读预检，用来提前看到可能冲突的文件。

## 执行合并

```bash
git merge --no-commit origin/main
```

解决冲突时按下面的优先级判断：

- 后端正确性修复优先吸收。
- dev-zz 已记录的视觉、认证显示、数据保留、模型探测和源码部署策略默认保留。
- 上游新增功能如果不破坏 dev-zz 策略，优先合并。

## 验证

基础检查：

```bash
git diff --check
git diff --cached --check
rg -n "^(<<<<<<<|=======|>>>>>>>)$"
```

前端常用检查：

```bash
pnpm --dir frontend typecheck
pnpm --dir frontend lint:check
```

后端常用检查：

```bash
mise x -C backend -- go test ./internal/server ./internal/handler ./internal/config
```

再根据冲突和变更范围补充更具体的测试。

文档站检查：

```bash
pnpm --dir docs-site docs:build
```

## 更新记录

合并完成后更新 [上游合并记录](./merge-log.md)，记录：

- 目标分支
- 上游分支
- base、合并前目标、上游 head
- 上游 highlights
- 冲突文件
- 解决策略
- 验证命令
- 未验证范围

如果合并带来用户可见行为变化，也更新：

- [变更记录](../changelog.md)
- [补丁记录](../patches.md)
- [dev-zz 变更地图](../reference/change-map.md)
- [验证矩阵](../testing/verification-matrix.md)

## 完成条件

- 没有冲突标记。
- `git diff --check` 通过。
- 针对性验证通过。
- [上游合并记录](./merge-log.md) 已更新。
- `docs-site` 中与本次合并相关的功能、部署、接口和验证文档已同步。
- 合并提交创建完成。
