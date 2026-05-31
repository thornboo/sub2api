# Patches

## 2026-05-06 - Anthropic Upstream API Key Pool Failover

Scope:
- `backend/migrations/{135_account_api_key_pool,136_account_api_key_pool_defaults,137_account_api_key_pool_scheduler_indexes_notx}.sql`
- `backend/internal/service/{account,account_api_key_pool,gateway_service}.go`
- `backend/internal/repository/{account_repo,account_api_key_pool}.go`
- `backend/internal/handler/{admin/account_handler,dto/*}.go`
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal,AccountAPIKeyPoolEditor}.vue`
- `frontend/src/types/index.ts`
- `frontend/src/i18n/locales/{zh,en}.ts`
- `secondary-dev/PLAN.md`

Changes:
- Added `account_api_keys` and `account_api_key_model_cooldowns` tables so an upstream Anthropic API-key account can manage multiple child keys.
- Kept the already-applied `135_account_api_key_pool.sql` checksum immutable and moved later key-pool default changes into `136_account_api_key_pool_defaults.sql`.
- Added key-pool scheduler indexes in a new `137_*_notx.sql` migration, preserving migration immutability while optimizing account/key priority and LRU scheduling queries.
- Loaded child keys and active key/model cooldowns into account scheduler snapshots.
- Added account create/edit API support for `api_keys`, preserving existing child key secrets when edit forms leave a key blank.
- Added repository audit logs for child-key create/update/preserve/delete save paths; log entries include account/key/name metadata only and never include API key values.
- Added repository regression tests for blank edit secret preservation, deleting all child keys when no valid rows remain, rejecting invalid account IDs, and treating unknown submitted IDs as inserts when a secret is provided.
- Extended model probing for saved child keys: edit-account probe requests may pass `account_id` and `account_api_key_id`, letting the backend use the stored key secret while keeping the edit form's API Key field blank.
- Improved child-key supported-model probe diagnostics: backend failures now log sanitized reason/host/account/key context, upstream non-2xx responses include the upstream HTTP status in the returned message, and probe requests send both Bearer and `X-Api-Key` headers for relay compatibility.
- Updated create/edit/key-pool probe error handling so the admin UI shows the backend-provided reason instead of always replacing it with the generic Base URL / API Key message.
- Added GoDoc comments to exported key-pool scheduling helpers to document model-level cooldown scope, legacy fallback behavior, and account failover semantics.
- Added per-child-key model rules: each child key defines its own whitelist or model mapping.
- In Anthropic API-key passthrough mode, requests now try available child keys in priority + least-recently-used order.
- Anthropic API-key `count_tokens` passthrough now uses the same child-key pool path as message forwarding, so Claude-compatible clients that preflight token counts use the selected child key instead of the legacy account-level API key.
- Child keys whose whitelist/mapping does not match the requested model are skipped before any upstream request is made; matching key-level mappings can override the account-level resolved upstream model for that selected key.
- Upstream failures that mean the selected key/model is unavailable, including relay-specific `cch_session_id` / official-client 400s, cool down only that child key plus the resolved upstream model.
- If all child keys for the selected account are unavailable, the existing account-level failover path receives `UpstreamFailoverError` and can try the next account.
- Plain user request 400s, such as missing request fields, do not trigger key failover and continue to return as request errors.
- Added create/edit account UI for Anthropic upstream child keys, including status, priority, per-key model rules, recent usage counters, and active model cooldown badges.
- Replaced the old top-level single API Key input for Anthropic API-key accounts with the upstream key pool as the only key input surface.
- In edit forms, existing child keys are shown without their secret value; leaving the API Key field blank preserves the stored secret instead of requiring re-entry.
- Required each newly added upstream child key to include a note/name, so operators can record the upstream provider group or purpose for that key.
- Labeled the child-key priority field and hint directly in the form; lower values are tried first, newly added keys default to priority `1`, and same-priority keys rotate by recent use.
- Switched the child-key status field to the shared frontend `Select` control for consistent project styling, and limited operator choices to enabled/disabled. Runtime errors remain represented by key/model cooldowns and counters instead of a manual `error` status.
- Reworked the key-pool editor into a left/right layout where each key owns its model whitelist/mapping; key-level whitelist editing reuses the shared model selector instead of a textarea, and Anthropic API-key accounts no longer show or save the shared account-level model restriction UI.
- Added key-level probe/manual-add badges so newly added models and models not returned by the latest probe remain visible inside each key row.
- Added validation that each upstream child key has its own model whitelist or mapping before saving.
- Clarified `secondary-dev/PLAN.md` that the upstream key-pool scheduler is the target design for all API-key account platforms. Anthropic is only the first platform currently wired through the runtime forwarding path.

Verification:
- `mise x -C backend -- go test ./internal/service -run 'TestAccountEffectiveAPIKeysForModel|TestAccountEffectiveAPIKey|TestGatewayService_AnthropicAPIKeyPassthrough'`
- `mise x -C backend -- go test ./internal/repository -run 'TestReplaceAccountAPIKeys|TestIsMigrationChecksumCompatible|TestApplyMigrations|TestLatestMigrationBaseline|TestMigrationChecksumCompatibilityRules'`
- `mise x -C backend -- go test ./internal/handler/admin -run 'TestAccountHandlerProbeModels|TestBuildProbeModelsEndpoint|TestNewProbeModelsHTTPRequest|TestProbeModelsUpstreamStatusMessage'`
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check`

## 2026-05-07 - Platform-Wide API Key Pool Forwarding

Scope:
- `backend/internal/service/{gateway_service,openai_gateway_service,openai_images,gemini_messages_compat_service,antigravity_gateway_service}.go`
- `backend/internal/service/*apikey*pool*_test.go`
- `backend/internal/service/{gateway_anthropic_apikey_passthrough,bedrock_request,openai_images,gemini_messages_compat_service,antigravity_gateway_service}_test.go`
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal,AccountAPIKeyPoolEditor}.vue`
- `secondary-dev/{PLAN,CHANGELOG,PATCHES}.md`

Changes:
- Extended child-key pool scheduling beyond the original Anthropic passthrough path to the current API-key forwarding paths: Claude/Anthropic API-key forwarding, OpenAI API-key Responses passthrough, OpenAI images, Gemini messages compatibility, Antigravity API-key/upstream forwarding, and Bedrock `auth_mode=apikey`.
- Added a Claude/API-key forwarding scheduler path that clones the selected account and parsed request per child key, rewrites the request body to the selected child key's resolved upstream model, and disables account-level failover side effects while trying keys inside one account.
- Ensured retry/cooldown isolation is child-key plus final upstream model. A failed child key/model records cooldown on the selected key ID and resolved upstream model without cooling the whole account.
- Kept stream safety behavior: once a streaming path has written client bytes, the request does not transparently fail over to another key.
- Preserved legacy single-key account behavior when no `account_api_keys` are configured.
- Opened the frontend API-key pool editor to Anthropic, OpenAI, Gemini, Antigravity, Antigravity upstream, and Bedrock API-key mode, while keeping the top-level single API Key input available for legacy accounts.
- Updated `secondary-dev/PLAN.md` from implementation-in-progress wording to the verified platform-wide status for this demand.

Verification:
- `/Users/thornboo/.local/share/mise/installs/go/1.26.2/bin/go test ./internal/service -run 'TestGatewayService_Forward_APIKeyPoolFailoverCoolsSelectedKeyModel|TestAntigravityGatewayService_ForwardUpstream_APIKeyPool|TestGatewayService_BedrockAPIKeyPool|TestOpenAIGatewayService_Forward_APIKeyPool|TestOpenAIGatewayService_APIKeyPassthrough|TestGeminiMessagesCompatServiceForward_APIKeyPool'`
- `/Users/thornboo/.local/share/mise/installs/go/1.26.2/bin/go test ./internal/service`
- `/Users/thornboo/.local/share/mise/installs/go/1.26.2/bin/go test ./...`
- `pnpm -C frontend exec eslint src/components/account/AccountAPIKeyPoolEditor.vue src/components/account/CreateAccountModal.vue src/components/account/EditAccountModal.vue --fix`
- `pnpm -C frontend typecheck`

## 2026-05-06 - Home Official Model Prices

Scope:
- `frontend/src/views/HomeView.vue`
- `secondary-dev/CHANGELOG.md`
- `secondary-dev/PATCHES.md`

Changes:
- Restored the Home page popular-model displayed prices from 85% discounted values to official prices.
- Kept the existing Chinese/English pricing note that actual pricing follows discounted group pricing.

Verification:
- `rg -n -F '$5/M input tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$30/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$25/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$2/M input tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$12/M output tokens' frontend/src/views/HomeView.vue`
- `git diff --check -- frontend/src/views/HomeView.vue secondary-dev/CHANGELOG.md secondary-dev/PATCHES.md`

## 2026-05-06 - Home Discounted Model Prices

Scope:
- `frontend/src/views/HomeView.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`

Changes:
- Updated the Home page popular-model displayed prices from 80% to 85% of official prices.
- Changed the Chinese pricing note from "实际以分组定价为准" to "实际以优惠后分组价格为准".
- Changed the English pricing note from "Actual price follows group pricing" to "Actual price follows discounted group pricing".

Verification:
- `rg -n -F '$4.25/M input tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$25.5/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$21.25/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$1.7/M input tokens' frontend/src/views/HomeView.vue`
- `rg -n -F '$10.2/M output tokens' frontend/src/views/HomeView.vue`
- `rg -n "Actual price follows group pricing|Actual price follows discounted group pricing|实际以分组定价为准|实际以优惠后分组价格为准" frontend/src/i18n/locales/en.ts frontend/src/i18n/locales/zh.ts`
- `cd frontend && pnpm run typecheck`
- `git diff --check -- frontend/src/views/HomeView.vue frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts`

## 2026-05-06 - Mapping Mode Clear All Models

Scope:
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal}.vue`
- `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`

Changes:
- Added "清除所有模型" / "Clear all models" actions to create/edit account model mapping sections.
- Covered normal account mapping sections, Bedrock mapping sections, and Antigravity's mapping-only account section.
- Clearing mappings keeps the current mapping mode UI active, removes all mapping rows, clears mapping catalog input state, and clears probe "new/missing" markers.
- Added an edit-modal regression test that clears mapping rows and verifies saved credentials no longer include `model_mapping` or `model_restriction_mode`.

Verification:
- `cd frontend && pnpm test:run src/components/account/__tests__/EditAccountModal.spec.ts`
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check`

## 2026-05-06 - Model Probe Mapping Fill

Scope:
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal}.vue`
- `frontend/src/components/account/ModelWhitelistSelector.vue`
- `frontend/src/components/account/ModelCatalogSearch.vue`
- `frontend/src/components/account/channelModelRecommendations.ts`
- `frontend/src/components/account/modelCatalog.ts`
- `frontend/src/i18n/locales/{zh,en}.ts`

Changes:
- Added the existing "获取支持模型" / "Fetch supported models" action to create/edit account model mapping sections.
- Fetched upstream model IDs are appended as same-name mapping rows (`model -> model`) without overwriting existing source-model mappings, so administrators can adjust the target side manually.
- Reused the existing backend probe endpoint, credential resolution, loading state, duplicate handling, and failure messages.
- Probe comparisons in mapping mode now evaluate the right-hand upstream target model, marking rows that were newly added or not returned by the latest upstream model list.
- Saved credentials now include `model_restriction_mode` when model mapping data is present, so same-name mapping rows can reopen in mapping mode instead of being inferred as a whitelist.
- Mapping quick-add recommendations now come from the selected groups' channel configuration: channel model-mapping targets first, falling back to channel pricing models when no mapping is configured.
- Custom model inputs now include a "查询" / "Search" action backed by the public models.dev catalog. Selecting a result fills the input; administrators still explicitly click "填入" or "添加同名映射" to apply it.

Verification:
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `git diff --check`

## 2026-05-05 - Account Model Probe

Scope:
- `backend/internal/handler/admin/account_handler.go`
- `backend/internal/handler/admin/account_handler_probe_models_test.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/api/admin/accounts.ts`
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal,ModelWhitelistSelector}.vue`
- `frontend/src/i18n/locales/{zh,en}.ts`

Changes:
- Added `POST /api/v1/admin/accounts/probe-models` for admin-only, non-persistent probing of OpenAI-compatible upstream model lists.
- The backend builds `/v1/models` from the supplied HTTPS Base URL, blocks private/localhost/link-local resolved hosts for SSRF defense, sends the current API key as a bearer token, parses `data[].id`, and returns de-duplicated model IDs without logging or persisting credentials.
- Added a "获取支持模型" / "Fetch supported models" button before "填入相关模型" / "Fill related models" in create/edit account whitelist selectors.
- The create/edit dialogs use the current form credentials where available, hide the probe action for Bedrock/service-account flows, append fetched models to the current whitelist, and fall back to clear failure messages so administrators can continue filling models manually.

Verification:
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm lint:check`
- `mise x -C backend -- go test ./internal/handler/admin ./internal/server`
- `git diff --check`

## 2026-05-05 - Home and Console UI Refresh

Scope:
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

Changes:
- Reworked the Home page into the current dark/light visual direction with model cards, quick access, testimonials, FAQ accordion, and a simplified footer.
- Removed public GitHub navigation surfaces from Home-related entry points.
- Routed "view more models" to `/available-channels`.
- Restyled console layout primitives and high-use admin/user pages with the stone/neutral/emerald theme.
- Portaled `DateRangePicker` and admin usage column settings to `body` to avoid clipping inside scrollable table/card containers.
- Corrected `HelpTooltip` fixed-position coordinates so scroll position no longer offsets operations-monitoring card tooltips.
- Moved Home page visible hardcoded Chinese copy into i18n keys and made code samples use the current site origin.
- Bound date-range and usage column-settings global listeners only while their menus are open, and kept closed-state guards on position updaters.
- Reworked the shared authentication layout plus login/register page accents to match the Home page stone/emerald theme, including theme/language controls.
- Hid LinuxDo and WeChat auth-platform UI only on the frontend: login/register OAuth buttons, profile binding cards/source hints, and admin auth settings/source defaults. Backend routes and settings data are left untouched.
- Synchronized `ProfileIdentityBindingsSection` tests with the new frontend-only provider visibility policy: LinuxDo/WeChat entries are expected to stay hidden, and OIDC remains covered for visible third-party binding details and unbind behavior.
- Removed unused Home navigation/footer locale keys (`home.nav.pricing`, `home.footer.privateDeploy`, `home.footer.custom`) from both Chinese and English locale files.
- Moved the remaining Home testimonial initials into i18n data and stopped hardcoding testimonial initials in `HomeView.vue`.
- Updated Home footer contact behavior so contact links use configured public contact info when it can be converted to `https`, `mailto`, or `tel`, and otherwise avoid reusing the FAQ anchor as a placeholder contact target.

Verification:
- `cd frontend && pnpm vitest run src/components/common/__tests__/HelpTooltip.spec.ts`
- `cd frontend && pnpm vitest run src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts`
- `cd frontend && pnpm typecheck`
- `cd frontend && pnpm build`
- `cd frontend && pnpm lint:check`
- `git diff --check`

## 2026-05-05 - Permanent Usage Data Retention

Scope:
- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `backend/internal/service/dashboard_aggregation_service.go`
- `backend/internal/service/dashboard_aggregation_service_test.go`
- `backend/internal/service/ops_cleanup_service.go`
- `backend/internal/service/ops_cleanup_service_test.go`
- `backend/internal/service/channel_monitor_service.go`
- `backend/internal/service/channel_monitor_maintenance_test.go`
- `deploy/config.example.yaml`
- `deploy/.env.example`

Changes:
- Added `dashboard_aggregation.retention.auto_cleanup_enabled`, defaulting to `false`, so usage-related data is retained permanently unless an administrator manually cleans it.
- Skipped automatic cleanup of raw `usage_logs`, `usage_billing_dedup`, hourly dashboard aggregates, and daily dashboard aggregates when retention auto cleanup is disabled.
- Kept the existing day-based retention settings available for deployments that explicitly re-enable automatic cleanup.
- Relaxed configuration validation so zero retention days are accepted when automatic cleanup is disabled, while keeping positive-day validation when automatic cleanup is enabled.
- Added `ops.cleanup.auto_cleanup_enabled`, defaulting to `false`, so scheduled ops maintenance no longer deletes old ops logs, system metrics, ops preaggregates, or channel monitor history/rollups unless explicitly re-enabled.
- Kept non-destructive channel monitor daily aggregation maintenance running while automatic deletion is disabled.

Verification:
- `mise x -C backend -- go test ./internal/config`
- `mise x -C backend -- go test ./internal/service -run 'TestDashboardAggregationService|TestDashboardService|TestUsageCleanup'`
- `mise x -C backend -- go test ./internal/service -run 'TestOpsCleanup|TestChannelMonitorRunDailyMaintenance|TestDashboardAggregationService|TestDashboardService|TestUsageCleanup'`

## 2026-05-05 - Ops System Log Cleanup Confirmation

Scope:
- `frontend/src/views/admin/ops/components/OpsSystemLogTable.vue`

Changes:
- Replaced the browser `window.confirm` for system-log cleanup with the shared confirmation modal.
- Added a current-filter summary to the cleanup confirmation so administrators can verify the deletion scope before submitting.
- Converted quick time ranges into explicit cleanup start/end timestamps so "按当前筛选清理" matches the visible time filter.

Verification:
- `cd frontend && pnpm typecheck`

## 2026-05-05 - dev-sd Source-Built Docker Deployment

Scope:
- `secondary-dev/README.md`
- `secondary-dev/deploy-dev-sd.sh`

Changes:
- Documented the deployment path for running the forked `dev-sd` branch from a locally built Docker image instead of the upstream `weishaw/sub2api:latest` image.
- Added a repeatable deployment script that builds `sub2api:dev-sd` from the repository root `Dockerfile`, prepares `deploy/.env` and local data directories, writes `deploy/docker-compose.override.yml`, and starts Docker Compose with the checked-in `deploy/docker-compose.local.yml`.
- Kept existing deployment secrets safe by reusing `deploy/.env` unless `--force-env` is explicitly passed.
- Forced recreation of the `sub2api` container after startup so repeated script runs pick up the rebuilt local image tag.
- Added validation for `IMAGE_NAME` before writing it into the Compose override file and switched `.env` replacement temporary files to `mktemp`.
- Added Docker Compose command detection so the script works with both `docker compose` and legacy `docker-compose`.

Verification:
- `bash -n secondary-dev/deploy-dev-sd.sh`
- `secondary-dev/deploy-dev-sd.sh --help`
- `secondary-dev/deploy-dev-sd.sh --build-only --no-build`
- `IMAGE_NAME='bad image' secondary-dev/deploy-dev-sd.sh --no-build --no-start`
- `git diff --check`

## 2026-05-05 - dev-sd Deployment Pre-Start Backups

Scope:
- `secondary-dev/README.md`
- `secondary-dev/deploy-dev-sd.sh`

Changes:
- Added a default pre-start backup step to the `dev-sd` deployment script before Compose recreates the application container.
- Writes timestamped backups under `deploy/backups/` by default, with `BACKUP_DIR` available for custom locations.
- Uses `pg_dump` for PostgreSQL when the existing `postgres` service is running, and archives deployment files (`.env`, `docker-compose.override.yml`, `data`).
- Added `--skip-backup` for disposable test deployments.
- Documented that live `postgres_data` and `redis_data` directory tarballs are not the default backup mechanism because file-level database archives can be inconsistent while services are running.

Verification:
- `bash -n secondary-dev/deploy-dev-sd.sh`
- `secondary-dev/deploy-dev-sd.sh --help`
- `secondary-dev/deploy-dev-sd.sh --build-only --no-build` (expected error)
- `secondary-dev/deploy-dev-sd.sh --no-build --no-start`
- `git diff --check`
