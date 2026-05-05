# Patches

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
