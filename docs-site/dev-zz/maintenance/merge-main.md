# 同步上游 main

这页记录把上游 `main` 同步到 dev-zz 系列分支的标准流程。默认先合并到 `dev-zz-develop` 做集成和测试；验证通过后，再把改动推进到正式线 `dev-zz`。默认比较对象是 `origin/main`，避免本地 `main` 停在旧提交上，导致误判合并范围。

## 目标

- 把上游 `main` 的修复和功能先吸收到 `dev-zz-develop`。
- 保留 dev-zz 已记录的二开策略。
- 记录冲突、取舍、验证命令和是否已经推进 `dev-zz`。

## 前置检查

```bash
git switch dev-zz-develop
git status --short --branch
git fetch origin
```

确认本地 `main` 与目标上游一致。如果只是预检，也可以直接以 `origin/main` 为比较对象，不必切换本地 `main`。

```bash
git switch main
git pull --ff-only origin main
git switch dev-zz-develop
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

- 目标分支和后续是否推进 `dev-zz`
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

## 推进正式线

`dev-zz-develop` 验证通过后，再把同一批改动推进到 `dev-zz`。如果 `dev-zz` 没有额外提交，优先快进，避免制造无意义的合并提交：

```bash
git switch dev-zz
git merge --ff-only dev-zz-develop
```

如果不能快进，先查清楚 `dev-zz` 上多出的提交是什么，再决定是合并、挑选还是先补同步记录。

## 完成条件

- 没有冲突标记。
- `git diff --check` 通过。
- 针对性验证通过。
- [上游合并记录](./merge-log.md) 已更新。
- `docs-site` 中与本次合并相关的功能、部署、接口和验证文档已同步。
- `dev-zz-develop` 合并提交创建完成；需要发布到正式线时，`dev-zz` 也已按验证结果推进。
