# Merge Log

This file records upstream synchronization work for secondary-development branches.

## 2026-06-06 - Sync upstream `main` into `dev-zz` for failed-request visibility and gateway fixes

Branch:
- Target: `dev-zz`
- Upstream: `main`
- Base: `f1aa5896`
- Target before merge: `a3997b07`
- Upstream head: `1cecd271`
- Result commit: this merge commit

Upstream highlights:
- Added failed-request persistence and visibility across ops/admin/user usage surfaces, including API-key/account attribution, deleted-key audit lookup, and user-facing error-request tables behind a public setting.
- Added gateway correctness fixes for Responses-to-Anthropic tool pairing, chat-completions failed responses, missing stream terminal output, OpenAI image rate-limit cooldown failover, and Claude Code client recognition.
- Added DB pool connection-lifetime floors, scheduler sticky-health escape, EasyPay query-order status handling, admin moderation auto-ban exemption, group description clearing, Go 1.26.4 toolchain updates, and related regression tests.

Merge strategy:
- Read `secondary-dev/README.md`, `secondary-dev/PATCHES.md`, `secondary-dev/MERGELOG.md`, and `secondary-dev/CHANGELOG.md` before merging.
- Refreshed local remote refs with `git fetch origin`; local `main` matched `origin/main` at `1cecd271`.
- Used `git merge-tree --write-tree dev-zz main` before the live merge; it predicted two content conflicts.
- Merged upstream `main` into `dev-zz` with `git merge --no-commit main`.
- Accepted upstream failed-request visibility and backend gateway/runtime fixes because they are additive or correctness-oriented and do not change secondary-development account model probing/mapping behavior, frontend auth visibility policy, permanent retention defaults, or source-built deployment policy.

Conflict files:
- `frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`
- `frontend/src/views/user/UsageView.vue`

Resolution notes:
- Kept the secondary-development stone/emerald admin ops table styling while adding upstream API-key and account attribution columns, including the deleted-key badge and the widened empty-state colspan.
- Kept the user usage page on secondary-development stone/emerald styling and image-usage display semantics while accepting upstream null-safe token/cost rendering and the new user error-request tab.
- The frozen `secondary-dev/DEV_SEED_DESIGN.md` document remained isolated in `stash@{0}` and was not included in this merge.

Verification:
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$"`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts src/views/admin/__tests__/UsageView.spec.ts src/views/user/__tests__/UsageView.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/server/middleware ./internal/handler ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/pkg/apicompat ./internal/payment/provider`

Not verified:
- Full frontend test suite was not run.
- Full backend test suite was not run.

## 2026-06-05 - Sync upstream `main` into `dev-zz` for OpenAI 5h usage semantics

Branch:
- Target: `dev-zz`
- Upstream: `main`
- Base: `aa69e394`
- Target before merge: `437e2df5`
- Upstream head: `f1aa5896`
- Result commit: this merge commit

Upstream highlights:
- Reverted the OpenAI Codex 5h usage percentage normalization so stored 5h usage follows upstream's direct `used_percent` semantics again.
- Removed the now-obsolete 5h remaining-to-used normalization helper and associated snapshot/account-usage tests.
- Updated OpenAI rate-limit and account usage tests to match the reverted 5h percentage behavior.

Merge strategy:
- Read `secondary-dev/README.md`, `secondary-dev/PATCHES.md`, and `secondary-dev/MERGELOG.md` before merging.
- Refreshed local remote refs with `git fetch origin`.
- Merged upstream `main` into `dev-zz` with `git merge --no-commit main`.
- No content conflicts were reported by `git merge-tree --write-tree dev-zz main` or the live merge.
- Accepted the upstream OpenAI 5h usage semantics revert because it is isolated to backend OpenAI Codex usage accounting and does not change secondary-development frontend auth visibility, account model probing/mapping behavior, available-channel export behavior, retention defaults, or deployment policy.

Conflict files:
- None.

Resolution notes:
- Automatic merge applied the upstream removal of `normalizeCodexFiveHourUsedPercent` and restored direct assignment from OpenAI Codex primary/secondary `used_percent` headers.
- Automatic merge kept secondary-development commits after the previous upstream sync, including available-channel table/export work and secondary deployment documentation alignment.
- The previously stashed billing-dimensional-pricing design document remained in stash and was not included in this merge.

Verification:
- `git diff --check`
- `git diff --cached --check`
- `rg -n "^(<<<<<<<|=======|>>>>>>>)$"`
- `mise x -C backend -- go test ./internal/service`

Not verified:
- Frontend typecheck, lint, and tests were not run because this upstream sync only changed backend OpenAI service files and tests.
- Full backend test suite was not run.

## 2026-06-02 - Sync upstream `main` into `dev-zz` for Codex bridge follow-up

Branch:
- Target: `dev-zz`
- Upstream: `main`
- Base: `bc5813f0`
- Target before merge: `8bd61b24`
- Upstream head: `aa69e394`
- Result commit: this merge commit

Upstream highlights:
- Codex Responses to Chat Completions bridge redesign, including request invariant coverage and response stream event wire helpers.
- WebSocket Codex image bridge tool injection and additional WebSocket ingress-session coverage.
- Antigravity Gemini rate-limit, scheduler-cache, quota-scope, and account-scheduling fixes.
- Admin user balance handling changed to pointer-based input so zero and omitted balance can be distinguished.

Merge strategy:
- Merged upstream `main` into `dev-zz` with `git merge --no-commit main`.
- No content conflicts were reported by `git merge-tree --write-tree dev-zz main` or the live merge.
- Accepted upstream backend compatibility, scheduler, Antigravity, and admin-user fixes because they are additive or correctness-oriented and do not change the secondary-development frontend auth-visibility policy.
- Preserved existing secondary-development records, account model probing/mapping behavior, permanent data-retention defaults, and source-built deployment guidance.

Conflict files:
- None.

Resolution notes:
- Automatic merge kept the previous `dev-zz` sync results intact while adding the upstream `apicompat` bridge redesign and new regression tests.
- Automatic merge accepted the admin user-create balance pointer change and its API typing update because it preserves explicit zero-balance input behavior.
- Automatic merge accepted Antigravity scheduler/rate-limit fixes without touching secondary-development UI policy or deployment records.

Verification:
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/views/admin/__tests__/UsersView.spec.ts`
- `mise x -C backend -- go test ./internal/pkg/apicompat ./internal/service ./internal/repository ./internal/handler/admin ./internal/handler`

Not verified:
- Full frontend test suite was not run.
- Full backend test suite was not run.

## 2026-06-02 - Sync upstream `main` into `dev-zz`

Branch:
- Target: `dev-zz`
- Upstream: `main`
- Base: `f18451e5`
- Target before merge: `e6e7e5b9`
- Upstream head: `bc5813f0`
- Result commit: this merge commit

Upstream highlights:
- OpenAI request-body retention/refactor, OOM handling, failover cached-body remapping coverage, WebSocket usage dedup fixes, oversized WebSocket request bridging, and WebSocket-to-HTTP bridge recovery.
- Account create flow can sync upstream models from entered credentials before the account is persisted.
- Admin usage performance/query-cache updates, model filter option loading from current stats, and support for viewing historical usage from deleted users.
- OpenAI OAuth refresh enrichment, Claude Code count_tokens allowance, OpenAI 5h usage-window percentage fix, and account usage-window tooltip copy.

Merge strategy:
- Merged upstream `main` into `dev-zz` with `git merge --no-commit main`.
- Preserved secondary-development account model probing, models.dev search, mapping-fill, same-name mapping persistence, clear-all model mapping, permanent data-retention defaults, Home/console visual direction, and source-built deployment records.
- Accepted upstream create-account upstream-model sync because it is additive and can coexist with the secondary-development credential-based probe flow.
- Accepted upstream admin-usage deleted-user and usage-window improvements while preserving the secondary-development popover/stone UI styling.

Conflict files:
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/admin/usage/UsageFilters.vue`

Resolution notes:
- Combined `ModelWhitelistSelector` props in create-account whitelist sections so secondary-development `/probe-models` loading/new/missing markers and upstream `syncCredentials` preview are both available.
- Kept Bedrock/Anthropic whitelist behavior consistent with the existing secondary-development probe flow while enabling upstream preview sync where credentials are present.
- Kept `UsageFilters` on the secondary-development `popover-item` styling and added upstream deleted-user badges plus deleted-user sorting.

Verification:
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts src/components/admin/usage/__tests__/UsageFilters.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts src/views/admin/__tests__/AccountsView.usageWindowsHint.spec.ts src/views/admin/__tests__/UsageView.spec.ts src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/handler`

Not verified:
- Full frontend test suite was not run.
- Full backend test suite was not run.

## 2026-06-01 - Sync upstream `main` into `dev-zz`

Branch:
- Target: `dev-zz`
- Upstream: `main`
- Base: `bebc0823`
- Target before merge: `f1ca9454`
- Upstream head: `f18451e5`
- Result commit: this merge commit

Upstream highlights:
- User platform quota DB aggregation flusher and sentinel backfill to reduce repeated preflight database reads.
- Account 5h/7d usage-threshold auto-pause, same-account retry status-code configuration, and account created-time display.
- Custom group-level `/v1/models` list configuration and candidate model loading.
- OpenAI embeddings gateway support, endpoint capability gating, Codex CLI/Claude Code allowed-client handling, and Responses/WebSocket compatibility fixes.
- Usage request-context preservation, concurrency error classification, model-not-found cooldown behavior, and local business-limit reason classification.
- Billing, long-context cache pricing, Gemini messages, Anthropic/Responses conversion, Bedrock context-management, pricing metadata, and version updates through `0.1.133`.

Merge strategy:
- Merged upstream `main` into `dev-zz` with no content conflicts reported by `git merge-tree --write-tree dev-zz main` or the live `git merge --no-commit main`.
- Preserved secondary-development account model probing, models.dev search, mapping-fill, same-name mapping mode persistence, clear-all model mapping, permanent data-retention defaults, Home/console visual direction, and source-built deployment records.
- Accepted upstream quota, auto-pause, group model-list, embeddings, endpoint capability, request-context, retry-status, account-created-time, pricing, billing, compatibility, and ops/risk-control updates.
- Continued the secondary-development frontend policy of hiding LinuxDo and WeChat auth surfaces while keeping upstream-visible providers and backend settings/data intact.

Conflict files:
- None.

Resolution notes:
- Automatic merge kept both account model-discovery paths: secondary-development credential-based `/probe-models` and upstream saved-account `/models/sync-upstream`.
- Automatic merge kept explicit `model_restriction_mode: mapping` handling and secondary-development clear-all mapping regression coverage while accepting upstream account auto-pause and retry-status configuration.
- Automatic merge kept the frontend-only LinuxDo/WeChat visibility policy in login/profile/admin surfaces while accepting upstream DingTalk and WeChat backend/settings/payment code where applicable.
- Accepted upstream custom group `/v1/models` list configuration because it is additive and does not alter secondary-development account whitelist, mapping, or scheduler behavior.

Verification:
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts src/composables/__tests__/useModelWhitelist.spec.ts src/views/admin/__tests__/groupsModelsList.spec.ts src/views/admin/__tests__/groupsModelsListCandidates.spec.ts src/views/admin/__tests__/groupsModelsListLayout.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts src/views/admin/__tests__/RiskControlView.spec.ts src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/handler/admin ./internal/config ./internal/service ./internal/repository ./internal/handler`

Not verified:
- Full frontend test suite was not run.
- Full backend test suite was not run.

## 2026-05-27 - Sync upstream `main` into `dev-zz`

Branch:
- Target: `dev-zz`
- Upstream: `main`
- Base: `18790386`
- Target before merge: `68877dcc`
- Upstream head: `bebc0823`
- Result commit: this merge commit

Upstream highlights:
- User platform USD quota support.
- DingTalk OAuth and provider-default grant support.
- Content moderation per-model controls and risk-threshold configuration.
- Account upstream model sync and OpenAI API Key Responses support controls.
- Channel monitor API-mode/template updates.
- User/admin usage image billing metadata and daily usage views.
- Redeem code batch updates, email templates, subscription reminder mail, and related admin UI updates.
- OpenAI Responses/WebSocket/tool-output continuation fixes, HTTP/2 timeout fix, and dependency/security updates.

Merge strategy:
- Merged upstream `main` into `dev-zz`.
- Preserved secondary-development account model probing, models.dev search, mapping-fill, same-name mapping mode persistence, clear-all model mapping, permanent data-retention defaults, Home/console visual direction, and source-built deployment records.
- Accepted upstream DingTalk, user platform quota, account upstream-model sync, OpenAI Responses controls, risk-control, channel-monitor, image-usage, redeem, email-template, subscription reminder, and gateway compatibility updates.
- Continued the secondary-development frontend policy of hiding LinuxDo and WeChat auth surfaces while accepting visible upstream DingTalk surfaces.

Conflict files:
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/ModelWhitelistSelector.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/user/profile/ProfileIdentityBindingsSection.vue`
- `frontend/src/components/user/profile/ProfileInfoCard.vue`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/views/admin/ChannelsView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/admin/UsersView.vue`
- `frontend/src/views/auth/LoginView.vue`
- `frontend/src/views/user/UsageView.vue`

Resolution notes:
- Kept both account model-discovery paths: secondary-development credential-based `/probe-models` and upstream saved-account `/models/sync-upstream`.
- Kept the secondary-development "Fill related models" / "填入相关模型" action label and added separate upstream sync labels.
- Preserved explicit `model_restriction_mode: mapping` so same-name mappings reopen as mapping mode, while keeping upstream mixed whitelist/mapping behavior for legacy mappings without an explicit mode.
- Kept secondary-development clear-all mapping regression coverage and upstream OpenAI Responses override regression coverage.
- Kept LinuxDo/WeChat hidden from frontend login/profile/admin default-source surfaces; DingTalk remains visible because it is an upstream feature outside the existing hide policy.
- Merged upstream channel pricing sync and user column forced-visible behavior into the secondary-development popover/emerald visual style.
- Fixed user usage tooltip resolution so image metadata is shown whenever image usage is present, even when stored `billing_mode` is empty.

Verification:
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/views/user/__tests__/UsageView.spec.ts src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/handler/admin ./internal/config`

Not verified:
- Full frontend test suite was not run.
- Full backend test suite was not run.

## 2026-05-14 - Sync upstream `main` into `dev-zz`

Branch:
- Target: `dev-zz`
- Upstream: `main`
- Base: `a1106e81`
- Target before merge: `6bdd4f1b`
- Upstream head: `18790386`
- Result commit: this merge commit

Upstream commits:
- `af550fa6` feat: 增加 GitHub 和 Google 邮箱快捷登录
- `e872cbec` feat: 添加登录注册条款确认
- `b23055af` feat: add Airwallex payments and multi-currency support
- `fff4a300` feat(risk-control): add content moderation audit
- `7a9c1d7e` feat(frontend): add account Codex image bridge control
- `18790386` fix(deploy): 移除数据库与 Redis 宿主机端口映射

Merge strategy:
- Merged upstream `main` into `dev-zz`.
- Preserved secondary-development account model probing, mapping-fill, clear-all model mapping, Home pricing text, permanent data-retention defaults, and source-built deployment records.
- Accepted upstream payment, email OAuth, login agreement, content moderation, Codex image bridge, OpenAI/Gemini compatibility, and deployment updates.

Conflict files:
- `backend/internal/server/routes/admin.go`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`
- `frontend/src/components/user/profile/ProfileInfoCard.vue`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/views/admin/AccountsView.vue`
- `frontend/src/views/admin/SettingsView.vue`
- `frontend/src/views/auth/LoginView.vue`
- `frontend/src/views/auth/RegisterView.vue`

Resolution notes:
- Kept both admin account routes: secondary-development `/probe-models` and upstream `/import/codex-session`.
- Kept both edit-account regression tests: clearing mapping-mode models and upstream Codex image bridge override.
- Kept GitHub/Google profile, login, registration, and auth-source default support from upstream while continuing to hide LinuxDo/WeChat frontend login/register/profile/settings surfaces.
- Kept secondary-development account model probe locale keys and the existing "Fill related models" / "填入相关模型" action label.
- Merged upstream account tools menu actions with the secondary-development popover styling.
- Kept upstream login agreement gating while preserving the secondary-development auth page visual style.

Verification:
- `git diff --check`
- `git diff --cached --check`
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend test:run src/components/account/__tests__/EditAccountModal.spec.ts src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts src/components/auth/__tests__/EmailOAuthButtons.spec.ts src/views/auth/__tests__/OAuthCallbackView.spec.ts`
- `mise x -C backend -- go test ./internal/server ./internal/handler/admin ./internal/config`

Not verified:
- Full frontend test suite was not run.
- Full backend test suite was not run.

## 2026-05-05 - Sync upstream WebSocket recovery fix into `dev-zz`

Branch:
- Target: `dev-zz`
- Upstream: `main`
- Result commit: `2d6e114a`

Upstream commits:
- `e71b55ec` fix: skip previous_response_id recovery when payload has function_call_output
- `94e49431` Merge pull request #2197 from learnerLj/fix/ws-preflight-ping-fc-output-recovery

Merge strategy:
- Merged `main` into `dev-zz`.
- Kept the existing secondary-development commits on `dev-zz`.
- No conflicts occurred.

Resolution notes:
- Accepted the upstream backend change in `backend/internal/service/openai_ws_forwarder.go`.
- Existing Home/auth/console UI secondary-development changes were preserved unchanged.

Verification:
- `pnpm --dir frontend typecheck`
- `pnpm --dir frontend lint:check`
- `git diff --check`

Not verified:
- Backend Go tests were not run because `go` was not available in the current shell.

Notes:
- `stash@{0}: On main: 数据永久保存` remains local and was not merged.
