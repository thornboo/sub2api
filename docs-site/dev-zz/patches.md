# 补丁记录

## 2026-06-13 - 企业 Key 批量维护

范围：
- `backend/internal/{handler,repository,server,service}/**`
- `frontend/src/{api,i18n,types,views/user}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,features/enterprise-key-member-management.md}`

改动：
- 用户侧 Key 列表新增按 `api_keys.id` 勾选的批量操作栏，批量动作只提交 ID，不依赖名称或脱敏 Key，避免同名 Key 或 Key 展示脱敏导致误操作。
- 新增 `POST /api/v1/keys/batch-update`，支持统一修改分组、状态、quota、过期时间、5h/1d/7d 限流、限流窗口用量和 IP 黑白名单。
- quota 批量更新支持设置固定额度、追加额度和改为无限制；过期时间支持统一设置或清空。
- 新增 `POST /api/v1/keys/batch-delete`，对选中 Key 做批量软删除。
- 批量更新和批量删除均先校验全部 ID 属于当前用户，再在单个事务内执行；任一写入失败时整批回滚。
- 事务提交后再失效认证缓存；重置限流用量时同步失效 Redis 限流缓存。
- 前端批量创建结果表为每把新 Key 增加单独复制按钮，保留复制全部与 CSV 导出。
- 本轮不引入 `api_keys.tags`，也不实现按筛选条件批量操作；当前批量维护范围限定为页面勾选的 ID 集合。

验证：
- `go test ./internal/service -run 'Test(APIKeyServiceBatch|BuildBatchAPIKeyNames)'`
- `go test ./internal/server/routes -run 'TestUserRoutesAPIKeyBatchPathsAreRegisteredBeforeIDRoute'`
- `go test ./...`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend build`
- `git diff --check`

## 2026-06-13 - 企业 Key 批量创建

范围：
- `backend/internal/{handler,repository,server,service}/**`
- `frontend/src/{api,i18n,types,views/user}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,decisions/adr-0002-key-as-enterprise-member.md,features/enterprise-key-member-management.md}`

改动：
- 新增用户侧 `POST /api/v1/keys/batch`，支持按名称模板或名称列表批量创建 API Key，并统一配置分组、quota、有效期、5h/1d/7d 限流和 IP 黑白名单。
- 批量创建在 service 层集中校验并通过 repository 事务一次性写入，任意一把失败时整批回滚；Key 唯一冲突做有界重试，事务提交后再失效认证缓存和编译 IP 规则。
- 新增设置项 `api_key_batch_create_max_count`，默认 `200`，服务端硬上限 `500`。
- 批量创建使用用户写幂等，但成功记录落库前会脱敏完整 Key；首次响应展示完整 Key，幂等重放只返回不可再次展示明文的摘要。
- 用户侧 Key 页面新增批量创建弹窗、结果弹窗、一次性明文提示、复制全部和包含完整字段的 CSV 导出。
- 阶段一不修改 `api_keys` schema，不引入子账号实体，不影响个人用户已有 Key 的认证、扣费、限流和使用链路。

验证：
- `go test ./internal/service ./internal/handler ./internal/server/routes ./internal/repository`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run lint:check`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

## 2026-06-13 - Key 自助状态查询

范围：
- `backend/internal/{handler,repository,server,service}/**`
- `frontend/src/{api,types}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,decisions/adr-0002-key-as-enterprise-member.md,features/enterprise-key-member-management.md}`

改动：
- 作为企业 Key 成员管理阶段一的补充需求，新增公共只读 `POST /api/v1/key/status`，允许只有 Key、没有站点账号的员工查询本人 Key 状态、quota 用量、过期时间、最近使用和限流配置。
- 查询结果只返回当前 Key 自身信息，不返回 owner 账号余额、邮箱、角色、其它 Key 或企业全局数据。
- 查询不走网关认证缓存，不更新 `last_used_at`，不扣 quota，不改限流窗口，只做读查询和状态推导。
- 同一 Key 10 秒内限查一次，限流标识使用 Key 哈希；Redis 冷却写入失败时 fail-close 返回不可用，不静默降级为多实例不一致的进程内限流。
- 路由层叠加 IP 级 `30/min` fail-close 限流，降低暴力枚举风险。

验证：
- `go test ./internal/service ./internal/handler ./internal/server/routes ./internal/repository`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run lint:check`
- `pnpm --dir docs-site docs:build`
- `git diff --check`

## 2026-06-13 - 运维明细弹窗栈与筛选体验优化

范围：
- `frontend/src/components/common/{BaseDialog,Select}.vue`
- `frontend/src/components/common/__tests__/{BaseDialog,Select}.spec.ts`
- `frontend/src/views/admin/ops/**`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `docs-site/dev-zz/{changelog.md,patches.md}`

改动：
- 将通用 `BaseDialog` 升级为模块级弹窗栈，自动按有效 z-index 分层，并确保 Escape、遮罩点击、关闭按钮和 body 滚动锁只由视觉最上层弹窗接管。
- 将 Ops 运维看板的请求详情、错误列表和单条错误详情状态抽取到 `useOpsModalStack`，支持父级明细弹窗与子级错误详情叠加打开，关闭子级不再连带关闭父级。
- 修复通用 `Select` 在弹窗等 `@click.stop` 容器内点击外部无法收起的问题，改用捕获阶段监听和真实 DOM ref 判断外部点击。
- 优化错误明细筛选区布局，为搜索、状态码、错误阶段、归属方和显示范围提供明确标签，并将搜索占位文案改为用户可读描述。
- 为错误列表取数增加请求序号，快速切换请求/上游错误类型时丢弃过期响应，避免旧数据覆盖新数据。
- 让单条错误详情的响应内容和关联上游响应预览使用阅读型自动换行代码块，保留 JSON 缩进和纵向滚动，移除横向阅读负担。

验证：
- `pnpm --dir frontend test:run src/components/common/__tests__/BaseDialog.spec.ts src/components/common/__tests__/Select.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts src/views/admin/ops/components/__tests__/OpsErrorDetailsModal.spec.ts src/views/admin/ops/components/__tests__/OpsRequestDetailsModal.spec.ts src/views/admin/ops/composables/__tests__/useOpsModalStack.spec.ts`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `git diff --check`

## 2026-06-12 - 上游 main 同步：合规确认与网关修复

范围：
- `.gitignore`
- `backend/internal/{handler,server,service,pkg}/**`
- `backend/migrations/150_account_group_scheduler_indexes_notx.sql`
- `docs/legal/**`
- `frontend/src/{api,components,composables,i18n,router,stores,views}/**`
- `docs-site/dev-zz/{changelog.md,patches.md,maintenance/merge-log.md}`

改动：
- 合并上游管理端部署与运营合规确认 gate，包括后端接口/中间件、前端确认弹窗、合规状态 store、公开法律文档和中英文文案。
- 合并上游网关正确性修复：错误透传/非流式错误帧重复写入保护、`MarkResponseCommitted` 覆盖、OpenAI failover 模型请求体替换，以及 idempotency 响应 UTF-8 截断。
- 合并上游 Bedrock / Claude 兼容修复、账号分组调度索引优化、调度日志循环优化、`claude-fable-5` 常量与 sponsor 资料更新。
- 解决 `.gitignore` 冲突时同时保留 dev-zz 的 `docs-site` 缓存忽略规则和上游 `docs/legal/*.md` 反忽略规则。

验证：
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$"`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/keys/__tests__/UseKeyModal.spec.ts src/api/__tests__/client.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/server/middleware ./internal/handler ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/pkg/apicompat ./internal/pkg/openai`

## 2026-06-10 - dev-zz 文档中心迁移

范围：
- `.gitignore`
- `deploy/deploy-dev-zz.sh`
- `docs-site/package.json`
- `docs-site/index.md`
- `docs-site/.vitepress/config.ts`
- `docs-site/project/**`
- `docs-site/dev-zz/**`
- `docs/LOCAL_DEVELOPMENT.md`
- `docs/AVAILABLE_CHANNELS_MODEL_MARKETPLACE_PLAN.md`
- `secondary-dev/**`

改动：
- 把 `docs-site/` 从一个生成的镜像目录改造为 `dev-zz` 的源文档中心。
- 在 `docs-site/project/` 下新增结构化项目文档。
- 将二开记录迁移到 `docs-site/dev-zz/`，包括变更记录、补丁说明、分支策略、部署文档、合并流程、合并记录、功能规划，以及文档中心的决策记录。
- 把 dev-zz 源码构建部署脚本移到 `deploy/deploy-dev-zz.sh`。
- 移除生成内容的同步脚本，并取消 `secondary-dev/` 作为独立文档目录。
- 把本地开发和可用渠道模型广场规划文档移入 `docs-site/dev-zz/`。

验证：
- `pnpm --dir docs-site docs:build`
- `bash -n deploy/deploy-dev-zz.sh`
- `git diff --check`

## 2026-05-06 - 首页官方模型价格

范围：
- `frontend/src/views/HomeView.vue`
- `docs-site/dev-zz/changelog.md`
- `docs-site/dev-zz/patches.md`

改动：
- 把首页热门模型展示价格从 85% 折扣值恢复为官方价格。
- 保留原有的中英文价格说明：实际价格以折扣后的分组价格为准。

验证：
- `rg -n -F '$5/M input tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$30/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$25/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$2/M input tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$12/M output tokens' frontend/src/views/HomeView.vue`
- `git diff --check -- frontend/src/views/HomeView.vue docs-site/dev-zz/changelog.md docs-site/dev-zz/patches.md`

## 2026-05-06 - 首页折扣模型价格

范围：
- `frontend/src/views/HomeView.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`

改动：
- 把首页热门模型展示价格从官方价的 80% 调整为 85%。
- 把中文价格说明从“实际以分组定价为准”改为“实际以优惠后分组价格为准”。
- 把英文价格说明从 "Actual price follows group pricing" 改为 "Actual price follows discounted group pricing"。

验证：
- `cd frontend && pnpm run typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check -- frontend/src/views/HomeView.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts`

## 2026-05-06 - 映射模式清空全部模型

范围：
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal}.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`

改动：
- 为创建/编辑账号模型映射区新增“清除所有模型” / "Clear all models" 操作。
- 覆盖普通账号映射区、Bedrock 映射区，以及 Antigravity 的仅映射账号区。
- 清空映射时保持当前映射模式 UI 激活，移除所有映射行，清空映射目录输入状态，并清除探测的“新增/缺失”标记。
- 新增一个编辑弹窗回归测试：清空映射行后，验证保存的凭据不再包含 `model_mapping` 或 `model_restriction_mode`。

验证：
- `cd frontend && pnpm test:run src/components/account/__tests__/EditAccountModal.spec.ts`
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check`

## 2026-05-06 - 模型探测映射填充

范围：
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal}.vue`
- `frontend/src/components/account/ModelWhitelistSelector.vue`
- `frontend/src/components/account/ModelCatalogSearch.vue`
- `frontend/src/components/account/channelModelRecommendations.ts`
- `frontend/src/components/account/modelCatalog.ts`
- `frontend/src/i18n/locales/{zh,en}.ts`

改动：
- 为创建/编辑账号模型映射区新增已有的“获取支持模型” / "Fetch supported models" 操作。
- 探测到的上游模型 ID 以同名映射行（`model -> model`）追加，不覆盖已有的源模型映射，管理员可手动调整目标侧。
- 复用已有的后端探测接口、凭据解析、加载状态、去重处理和失败提示。
- 映射模式下的探测比对现在评估右侧的上游目标模型，标记新增的行，以及最新上游模型列表未返回的行。
- 当存在模型映射数据时，保存的凭据会包含 `model_restriction_mode`，使同名映射行能以映射模式重新打开，而不被推断为白名单。
- 映射快速添加的推荐现在来自所选分组的渠道配置：优先用渠道模型映射目标，未配置映射时回落到渠道定价模型。
- 自定义模型输入框新增基于公开 models.dev 目录的“查询” / "Search" 操作。选中结果会填入输入框；管理员仍需显式点击“填入”或“添加同名映射”才会应用。

验证：
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check`

## 2026-05-05 - 账号模型探测

范围：
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/account_handler_probe_models_test.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal,ModelWhitelistSelector}.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`

改动：
- 新增 `POST /api/v1/admin/accounts/probe-models`，用于管理员专属、不持久化地探测 OpenAI 兼容的上游模型列表。
- 后端从传入的 HTTPS Base URL 构造 `/v1/models` 请求，为防御 SSRF 拦截解析到私有/本地/链路本地地址的主机，以 bearer token 发送当前 API key，解析 `data[].id`，并返回去重后的模型 ID，不记录也不持久化凭据。
- 在创建/编辑账号白名单选择器中，于“填入相关模型” / "Fill related models" 之前新增“获取支持模型” / "Fetch supported models" 按钮。
- 创建/编辑对话框会尽量使用当前表单凭据，对 Bedrock/服务账号流程隐藏探测操作，把探测到的模型追加到当前白名单，并在失败时回落到清晰的提示，让管理员可以继续手动填模型。

验证：
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `mise x -C backend -- go test ./internal/handler/admin ./internal/server`
- `git diff --check`

## 2026-05-05 - 首页与控制台 UI 焕新

范围：
- `frontend/src/views/HomeView.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `frontend/src/views/auth/{LoginView,RegisterView}.vue`
- `frontend/src/components/auth/*OAuthSection.vue`
- `frontend/src/style.css`
- `frontend/src/components/common/*`
- `frontend/src/components/layout/*`
- `frontend/src/views/admin/*`
- `frontend/src/views/admin/ops/*`
- `frontend/src/views/user/*`

改动：
- 把首页改造为当前的明暗视觉方向，包含模型卡片、快捷入口、用户推荐、FAQ 折叠面板和简化的页脚。
- 从首页相关入口移除公开的 GitHub 导航。
- 将“查看更多模型”指向 `/available-channels`。
- 用 stone/neutral/emerald 主题重新设计控制台布局基础组件和高频的管理端/用户端页面。
- 将 `DateRangePicker` 和管理端用量列设置通过 portal 渲染到 `body`，避免在可滚动的表格/卡片容器内被裁切。
- 修正 `HelpTooltip` 的 fixed 定位坐标，使滚动位置不再偏移运维监控卡片的提示。
- 把首页可见的硬编码中文文案移入 i18n key，并让代码示例使用当前站点 origin。
- 仅在日期范围和用量列设置菜单打开时绑定全局监听，并对位置更新器保留关闭状态的守卫。
- 重做共享认证布局以及登录/注册页的强调色，使其匹配首页的 stone/emerald 主题，包括主题/语言控件。
- 仅在前端隐藏 LinuxDo 和微信认证平台 UI：登录/注册 OAuth 按钮、资料绑定卡片/来源提示，以及管理端认证设置/来源默认值。后端路由和设置数据保持不变。

验证：
- `cd frontend && pnpm vitest run src/components/common/__tests__/HelpTooltip.spec.ts`
- `cd frontend && pnpm vitest run src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts`
- `cd frontend && pnpm typecheck`
