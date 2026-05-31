# Changelog

## 2026-05-06

- Added an Anthropic API-key upstream key pool for second-layer relay deployments, allowing one account to hold multiple upstream keys with per-key model whitelist/mapping, key/model-scoped cooldown, and failover before switching accounts.
- Extended the upstream API-key pool from Anthropic-only coverage to the current API-key forwarding paths: Claude/Anthropic API-key forwarding, OpenAI Responses/images passthrough, Gemini messages compatibility, Antigravity API-key/upstream forwarding, and Bedrock API-key mode.
- Kept legacy single API-key accounts usable while allowing account create/edit forms to configure per-child-key model rules for Anthropic, OpenAI, Gemini, Antigravity, Antigravity upstream, and Bedrock API-key mode.
- Split later key-pool default changes into a follow-up migration so already-applied local `135_account_api_key_pool.sql` checksums remain valid.
- Added scheduler indexes for the upstream key pool in a new non-transactional migration instead of mutating already-applied migrations.
- Added repository audit logs for child-key create/update/preserve/delete save paths without logging secret values.
- Added repository regression coverage proving edit forms can leave a child key secret blank while preserving the stored key.
- Fixed edit-account key-pool model probing so existing child keys can fetch supported models through the stored server-side secret without exposing the secret back to the browser.
- Improved supported-model probing diagnostics: probe failures now surface backend error details in the admin UI, log sanitized failure context on the backend, include upstream `/v1/models` HTTP status when available, and send both Bearer and `X-Api-Key` auth headers for relay compatibility.
- Extended Anthropic API-key `count_tokens` passthrough to use the upstream key pool, including per-key model mapping, key/model cooldown, and next-key failover.
- Documented key-pool scheduling semantics in exported service methods so model-scoped cooldown and failover behavior is explicit in code.
- Made the Anthropic API-key account form use the upstream API key pool as the only key input surface, with per-key notes, default key priority `1`, and a two-state enabled/disabled key status control.
- Moved Anthropic API-key model limits fully into each upstream key row, removing the misleading inherited/shared account model restriction path for key-pool accounts.
- Kept key-level model whitelists on the shared model selector UI, including the models.dev search/fill flow, probe badges, and clear-all behavior; edit forms keep existing child-key secrets when the key field is left blank.
- Restored the Home page popular-model displayed prices to official prices while keeping discounted group pricing as the actual billing note.
- Updated the Home page popular-model displayed prices from 80% to 85% of official prices and clarified the Chinese/English group-pricing note as discounted group pricing.
- Extended the account model-probe action to create/edit model mapping sections, appending fetched upstream models as same-name mapping pairs for administrator adjustment.
- Refined account model probing and mapping setup so probe results compare against upstream target models, mark newly added and missing models, preserve explicit whitelist/mapping mode, and use selected-channel configuration for mapping recommendations.
- Added a models.dev catalog search next to custom model input fields so administrators can look up public model IDs and fill whitelist or same-name mapping rows without replacing manual entry.
- Added "Clear all models" actions to create/edit account model mapping sections so administrators can bulk-clear mapping rows without switching back to whitelist mode.

## 2026-05-05

- Refined the Home page visual system and removed public GitHub entry points from the Home/footer/header surfaces.
- Updated console layout, sidebar, header, cards, tables, dialogs, dropdowns, announcements, usage views, and operations-monitoring pages toward the new stone/neutral/emerald theme.
- Fixed clipped date-range and column-setting dropdowns by rendering them through body-level portals.
- Fixed operations-monitoring help tooltip positioning after page scroll.
- Moved newly added Home page visible Chinese copy into locale files so English mode no longer shows Chinese fallback copy.
- Stopped date-range and usage column-setting dropdowns from keeping global scroll/resize/click listeners active while closed.
- Restyled login and registration entry pages to match the Home page stone/emerald dark-light visual direction.
- Hid LinuxDo and WeChat third-party auth platform entries from frontend login, registration, profile binding, and admin auth settings displays.
- Updated profile identity binding tests to match the frontend-only LinuxDo/WeChat hiding behavior while preserving OIDC binding coverage.
- Cleaned up unused Home i18n keys, moved remaining testimonial initials into locale data, and made footer contact links use configured contact info instead of the FAQ anchor.
- Disabled automatic dashboard retention cleanup by default so usage logs, billing dedup data, and usage dashboard aggregates are kept until an administrator manually deletes them.
- Disabled automatic ops retention cleanup by default so ops logs, metrics, preaggregates, and channel monitor history are not deleted by scheduled maintenance.
- Replaced the ops system-log cleanup browser confirm with the project modal confirmation and current-filter summary.
- Added secondary-development Docker deployment documentation and a source-build deployment script for the `dev-sd` branch.
- Added automatic pre-start deployment backups to the `dev-sd` source-build script.
- Added an SSRF-guarded admin account model-probe action that fetches OpenAI-compatible `/v1/models` results through the backend and appends them to create/edit model whitelists.
