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
