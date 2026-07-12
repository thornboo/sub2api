# Design

## Source of truth

- Status: Active
- Last refreshed: 2026-07-13
- Primary product surfaces:
  - User console: `frontend/src/views/user/**`
  - User API Key management: `frontend/src/views/user/KeysView.vue`, `frontend/src/components/keys/**`
  - Enterprise member control plane: `frontend/src/views/user/EnterpriseMembersView.vue`
  - User usage records: `frontend/src/views/user/UsageView.vue`
  - Admin usage and dashboard: `frontend/src/views/admin/UsageView.vue`, `frontend/src/api/admin/dashboard.ts`
  - dev-zz product records: `docs-site/dev-zz/**`
- Evidence reviewed:
  - `docs-site/dev-zz/decisions/adr-0002-key-as-enterprise-member.md`
  - `docs-site/dev-zz/decisions/adr-0003-enterprise-member-entity.md`
  - `docs-site/dev-zz/features/enterprise-member-management.md`
  - `docs-site/dev-zz/features/enterprise-key-member-management.md`
  - `docs-site/dev-zz/features/api-key-usage-drilldown.md`
  - `backend/ent/schema/api_key.go`
  - `backend/ent/schema/group.go`
  - `backend/ent/schema/usage_log.go`
  - `backend/internal/service/user.go`
  - `backend/internal/service/api_key_auth_cache.go`
  - `frontend/src/views/admin/UsageView.vue`
  - `frontend/src/types/index.ts`

## Brand

- Personality: calm, operational, enterprise-grade, precise, and low-drama.
- Trust signals: clear ownership boundaries, explicit permission language, deterministic controls, compact high-density tables, and stable drilldown paths.
- Avoid:
  - Marketing-page composition inside the console.
  - Decorative gradients, oversized hero sections, or card-heavy empty decoration.
  - Ambiguous cost wording where administrator cost and user billed amount could be confused.

## Product goals

- Goals:
  - Make AI gateway usage observable at the right level: platform admin, enterprise owner, employee Key, group, model, and request.
  - Let enterprise owners manage durable, non-login member identities that may each own multiple API Keys.
  - Give each member one shared set of 5h, 1d, 7d, and calendar-month spending limits across all assigned Keys.
  - Preserve platform administrator-only visibility into upstream account cost, routing, and operational internals.
  - Keep future feature work grounded in small, reviewable slices that fit the dev-zz branch discipline.
- Non-goals:
  - Do not convert the app into a broad BI product.
  - Do not expose platform-wide analytics to ordinary users.
  - Do not introduce a new design system while the existing Tailwind/component style can be extended.
  - Do not introduce subaccount login entities unless a future ADR reverses ADR 0002.
- Success signals:
  - An owner can answer "which employee Key spent the most, on which models, in which period?"
  - A platform admin can answer "which user, group, account, and route is driving operational cost?"
  - Reviewers can tell from DTO names and routes whether a field is user-safe or admin-only.

## Personas and jobs

- Primary personas:
  - Platform administrator: operates the whole site, upstream accounts, channels, groups, pricing, abuse, and profitability.
  - Enterprise owner: a normal enterprise user who manages non-login member identities, their Keys, access groups, and aggregate limits.
  - Employee with a Key only: has no site account and can only inspect that Key's limited status.
- User jobs:
  - Platform administrator: troubleshoot global usage, cost, routing, failed requests, and user behavior.
  - Enterprise owner: create members, issue multiple Keys per member, delegate accessible groups, set aggregate limits, correct consumed projections with immutable audit evidence, and inspect usage evidence.
  - Employee with a Key only: confirm whether the Key is active, expired, rate limited, or out of quota.
- Key contexts of use:
  - Dense admin console on desktop.
  - Owner-side console for repeated operational checks.
  - Occasional mobile/tablet lookup for Key state, not full analysis.

## Information architecture

- Primary navigation:
  - User side: Dashboard, API Keys, Usage Records, Profile.
  - Admin side: Dashboard, Usage, Users, Groups, Accounts, Ops.
- Core routes/screens:
  - User API Keys remain the owner workspace for employee-seat Key management.
  - User Usage Records remain the owner request-log surface.
  - Enterprise Members is the owner workspace for member identity, shared spending limits, group delegation/order, Keys, usage, and audit.
  - Admin Usage remains the platform-wide request-log and analytics surface.
  - Admin Dashboard remains the platform-wide aggregate surface.
- Content hierarchy:
  - Summary cards first, then filters, then charts/rankings, then table drilldown.
  - Rank and anomaly panels must always link to the underlying Key/user/group/request details.

## Design principles

- Principle 1: Every analytics surface must make its authority obvious.
  - User-side surfaces say and enforce "my account / my Keys".
  - Admin surfaces say and enforce "platform-wide".
- Principle 2: Show enough detail to act, not every internal implementation detail.
  - Owners see billed usage and their own Key metadata.
  - Admins see upstream cost, account, channel, model mapping, and operational traces.
- Principle 3: Prefer drilldown over overloaded lists.
  - Keep Key lists scan-friendly.
  - Put historical trends, model distributions, and request logs in panels or dedicated analytics views.
- Principle 4: Product language must describe owner intent before routing mechanics.
  - Say "成员可访问的分组" for delegation; present ordering as the routing priority of the selected subset.
  - Say "成员编号" for the immutable import/audit identity; do not expose the internal adjective "stable" as the field name.
- Tradeoffs:
  - First versions may use raw `usage_logs` with strict date limits.
  - Add pre-aggregation only when a measured query path needs it.

## Visual language

- Color: follow the existing stone/neutral/emerald console direction used in dev-zz. Use red only for destructive states or true errors.
- Typography: compact console typography; no viewport-based font scaling.
- Spacing/layout rhythm: dense but readable, with consistent filter rows, table headers, and chart cards.
- Shape/radius/elevation: keep cards and dialogs consistent with existing app cards; avoid nested cards.
- Motion: restrained loading states only; no decorative animation.
- Imagery/iconography: use existing icon components/libraries for action buttons and tabs.

## Components

- Existing components to reuse:
  - `BaseDialog`, `Select`, `DateRangePicker`, `Pagination`, admin usage charts, usage tables, and existing `components/keys` panels.
- New/changed components:
  - Owner analytics dashboard components should live under `frontend/src/components/keys` or a future `frontend/src/components/enterprise-usage`.
  - Admin-only analytics components should stay under `frontend/src/components/admin`.
- Variants and states:
  - Every analytics panel needs loading, empty, error, and stale-data states.
  - Member limit editing shows limit and consumed amount together for 5h, 1d, 7d, and calendar month; consumed changes write system-attributed before/after audit evidence without requiring extra operator input.
  - Tables need compact numeric formatting with full values available in tooltip/title.
- Token/component ownership:
  - Extend existing Tailwind utility style and local component patterns.
  - Do not add chart or UI dependencies unless the existing stack cannot represent the required view.

## Accessibility

- Target standard: practical WCAG 2.1 AA for contrast, keyboard access, labels, and focus.
- Keyboard/focus behavior:
  - Tabs, dialogs, filters, and table actions must be keyboard reachable.
  - Dialog focus must stay inside the modal and restore on close.
- Contrast/readability:
  - Numeric tables must remain readable in dark mode.
  - Error and warning states cannot rely on color alone.
- Screen-reader semantics:
  - Chart panels need summary text or table equivalents.
  - Icon-only buttons need labels or tooltips.
- Reduced motion and sensory considerations:
  - Avoid animated chart transitions that hide data changes.

## Responsive behavior

- Supported breakpoints/devices:
  - Desktop is primary for analytics.
  - Tablet/mobile should remain usable for lookup, not full comparison workflows.
- Layout adaptations:
  - Cards stack on small screens.
  - Wide data tables may scroll horizontally, but controls should not overlap.
- Touch/hover differences:
  - Important values cannot be hover-only on mobile.
  - Tooltips are supplementary, not the only access to data.

## Interaction states

- Loading:
  - Show per-panel loading so one slow chart does not blank the whole page.
- Empty:
  - Empty analytics states should say what filter or date range produced no data.
- Error:
  - Use retryable panel errors; preserve filters and last successful context when possible.
- Success:
  - Prefer quiet inline updates over toast spam for chart refreshes.
- Disabled:
  - Disabled controls should explain missing permissions or invalid filter combinations.
- Offline/slow network:
  - Request race guards or abort controllers are required for rapidly changing filters.

## Content voice

- Tone: operational, direct, and specific.
- Terminology:
  - "实际扣除" for user-billed amount.
  - "账号成本" only on admin surfaces.
  - "企业成员" for the durable non-login identity and "成员 Key" for a Key assigned to that member.
  - "成员编号" for the immutable import/audit identifier.
  - "成员可访问的分组" for owner-delegated access; "调用优先级" for the selected order.
  - "分组" for routing/billing group.
- Microcopy rules:
  - Avoid explaining the UI in visible product copy.
  - Do use short permission copy where a field is intentionally hidden.

## Implementation constraints

- Framework/styling system:
  - Vue 3, TypeScript, Tailwind-style classes, existing app components.
- Design-token constraints:
  - Keep dev-zz stone/neutral/emerald direction.
- Performance constraints:
  - Owner analytics must enforce backend date-range limits.
  - Avoid loading full per-Key time series for every Key in a list.
  - Use pre-aggregation only after the raw query path is measured or clearly bounded.
- Compatibility constraints:
  - Preserve ADR 0003: enterprise members are non-login entities; member Keys inherit the member's ordered group delegation.
  - `member_code` is immutable after creation and remains globally unique within the enterprise, including archived members.
  - Member 5h/1d/7d/month limits are aggregate controls shared by all member Keys and use durable reservations; per-Key quota/rate limits remain an additional stricter layer.
  - Consumed-amount corrections must be auditable. Calendar-month corrections are immutable ledger deltas; window projections retain before/after evidence plus a stable system source and note.
  - Member creation may establish non-zero current-period usage for 5h/1d/7d/month without an extra reason field, while the backend commits the member, group bindings, opening ledger/projections, and system-attributed audit evidence atomically. Calendar-month opening usage is `migration_opening`, not fabricated request usage.
  - Member group delegation must reuse current group authorization semantics: public vs exclusive groups, `users.AllowedGroups`, subscription eligibility, and group fallback behavior.
  - Admin DTOs must not be reused for user-facing analytics if they include admin-only fields.
  - Owner tag analytics must not present repeated multi-tag attribution as a 100% cost split unless the API contract defines an explicit denominator.
  - Owner summary cards must distinguish selected-range historical aggregates from current realtime governance snapshots such as quota and rate-limit proximity.
- Test/screenshot expectations:
  - Backend permission tests must cover cross-user denial and admin-only field absence.
  - Frontend typecheck and lint are required for new analytics components.
  - Visual QA should be done on the user-run dev server when the user asks for browser validation.

## Open questions

- [ ] Whether owner analytics should be a tab inside API Keys or a dedicated user route / owner / impact: product owner / route and navigation scope.
- [ ] Whether the public Key-status surface should expose member aggregate remaining limits in addition to the Key's own limits / owner / impact: privacy and support expectations.
- [ ] Whether owner-visible model analytics should include mapped model names or only requested model names / owner / impact: privacy and debugging usefulness.
- [ ] Whether owner analytics needs CSV export in the first implementation phase / owner / impact: scope and data volume.
