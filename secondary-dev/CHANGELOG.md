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
