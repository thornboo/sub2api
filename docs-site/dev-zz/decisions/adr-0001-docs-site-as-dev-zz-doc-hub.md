# 设计取舍 0001：docs-site 作为 dev-zz 文档中心

## 结论

已采纳。

## 背景

原 `secondary-dev/` 目录用于记录 dev-zz 的二开内容，包括变更记录、补丁记录、合并日志和部署脚本。随着 dev-zz 的二开内容增多，少数几个顶层 Markdown 文件已经无法清楚承载项目文档、二开策略、部署说明、开发指南、功能设计和维护流程。

同时，源项目 `docs/` 目录只有少量 Markdown 文档，不适合作为长期扩展的文档体系。

## 取舍

`docs-site/` 作为 dev-zz 分支的完整文档中心，也是后续维护者查二开资料的默认入口。

保留 `docs/` 作为上游兼容入口，主要为了不破坏现有 README 链接、GitHub 直接阅读和未来上游合并。

删除 `secondary-dev/`，其职责迁移为：

- dev-zz 总览和规则 -> `docs-site/dev-zz/`
- 变更记录 -> `docs-site/dev-zz/changelog.md`
- 补丁记录 -> `docs-site/dev-zz/patches.md`
- 合并记录 -> `docs-site/dev-zz/maintenance/merge-log.md`
- 部署脚本 -> `deploy/deploy-dev-zz.sh`

## 后果

- dev-zz 文档有了完整目录结构，可通过 VitePress 在网页中查看。
- `secondary-dev/` 不再作为单独目录存在，后续维护者应从 `docs-site/dev-zz/` 读取二开上下文。
- 同步上游 main 时，需要更新 `docs-site/dev-zz/maintenance/merge-log.md`。
- 用户可见二开变化需要更新 `docs-site/dev-zz/changelog.md` 和 `docs-site/dev-zz/patches.md`。
- 部署脚本从 `./secondary-dev/deploy-dev-zz.sh` 改为 `./deploy/deploy-dev-zz.sh`。

## 替代方案

### 保留 secondary-dev，只让 docs-site 同步展示

这种方式兼容旧路径，但会让 `secondary-dev/` 继续膨胀，解决不了文档结构不足的问题。

### 把所有文档都迁出 docs/

这种方式路径更统一，但会破坏上游兼容文档入口，不利于后续合并上游 main。
