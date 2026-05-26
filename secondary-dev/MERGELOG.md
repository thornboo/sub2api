# Merge Log

This file records upstream synchronization work for secondary-development branches.

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

## 2026-05-05 - Sync upstream WebSocket recovery fix into `dev-sd`

Branch:
- Target: `dev-sd`
- Upstream: `main`
- Result commit: `2d6e114a`

Upstream commits:
- `e71b55ec` fix: skip previous_response_id recovery when payload has function_call_output
- `94e49431` Merge pull request #2197 from learnerLj/fix/ws-preflight-ping-fc-output-recovery

Merge strategy:
- Merged `main` into `dev-sd`.
- Kept the existing secondary-development commits on `dev-sd`.
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
