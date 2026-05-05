# Patches

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

Verification:
- `bash -n secondary-dev/deploy-dev-sd.sh`
- `secondary-dev/deploy-dev-sd.sh --help`
- `git diff --check`
