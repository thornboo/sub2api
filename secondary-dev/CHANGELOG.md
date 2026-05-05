# Changelog

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
