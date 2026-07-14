# Design

## Source of truth

- Status: Active
- Last refreshed: 2026-07-15
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
  - Let enterprise owners migrate external member identities, API Keys, current-period spending, and aggregate token baselines without requiring external systems to know sub2api group IDs.
  - Give each member one shared set of 5h, 1d, 7d, and calendar-month spending limits across all assigned Keys.
  - Let enterprise owners apply shared member policy changes in one atomic batch without overwriting fields they did not explicitly select.
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
- Principle 5: External migration facts and sub2api authorization policy are separate inputs.
  - Import files describe external identities, credentials, limits, and opening usage evidence.
  - Group delegation is selected from the authenticated owner's current sub2api groups inside the import flow or a later batch action; public templates never require deployment-specific group IDs.
  - Import policy version 2 treats the owner's in-product selection as authoritative even when it is empty; an empty selection means "暂不授权" and must never fall back to group IDs carried by a historical file. Policy version 1 alone preserves the legacy row-group behavior for already-created jobs.
  - Members imported without a group policy remain disabled and are presented as "待配置" until an owner assigns at least one valid group and explicitly enables them.
- Principle 6: The owner chooses the lifecycle outcome; storage mechanics stay a server concern.
  - Disable is reversible operational control. Archive is reversible removal from the default workspace. Delete is an irreversible owner-facing removal and is always available after archive.
  - A clean member may be physically deleted. A member with historical facts becomes an invisible tombstone so billing, usage, and audit relationships remain valid; the UI must not turn evidence retention into an undeletable-member dead end.
- Principle 7: Bulk policy changes and usage reconciliation are different jobs.
  - Bulk policy editing may change selected limits, status, and ordered group delegation; every field is opt-in and omitted fields remain unchanged.
  - Bulk usage adjustment changes accounting projections and therefore lives in a separate destructive flow with immutable ledger/audit evidence.
  - Both flows are all-or-nothing, carry every selected member's expected version, and show the affected member count before submission.
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
  - `BaseDialog`, `ConfirmDialog`, `Select`, `DateRangePicker`, `Pagination`, admin usage charts, usage tables, and existing `components/keys` panels.
- New/changed components:
  - Owner analytics dashboard components should live under `frontend/src/components/keys` or a future `frontend/src/components/enterprise-usage`.
  - Admin-only analytics components should stay under `frontend/src/components/admin`.
- Variants and states:
  - Every analytics panel needs loading, empty, error, and stale-data states.
  - Member limit editing shows limit and consumed amount together for 5h, 1d, 7d, and calendar month; consumed changes write system-attributed before/after audit evidence without requiring extra operator input.
  - Enterprise member import is a guided flow: upload and authoritative preview, system-side access policy, confirmation, then one-click follow-up for any members left pending.
  - Bulk member actions include ordered group replacement in addition to enable/disable. Group replacement must state that it overwrites the selected members' current routing policy and must never enable members implicitly.
  - Bulk policy editing uses explicit field toggles for the calendar-month, 5h, 1d, and 7d limits; `0` means unlimited only when the corresponding field toggle is enabled.
  - Bulk policy group mode is one of keep, replace, or append. Replacing with an empty set disables affected members; enabling is rejected when the resulting group route is empty.
  - Bulk usage adjustment applies signed deltas to the selected members' current month and rate-limit windows. It never rewrites request logs, cannot reduce any projection below zero, and displays the aggregate adjustment before confirmation.
  - Archived members are read-only but provide two explicit exits: restore as disabled, or permanently remove. Destructive confirmation explains that historical billing/audit evidence can remain even though the member disappears from management.
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
  - Import success distinguishes created members, created Keys, migration opening usage, assigned access policy, and members still awaiting configuration.
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
  - `member_code` is immutable while a member exists and remains unique across current and archived members. Irreversible owner-facing removal replaces historical tombstones with a server-only code, allowing the original code to be reused without reassigning old facts.
  - Restore clears archive state but leaves the member disabled so group access and Keys cannot resume without an explicit owner enable action.
  - Delete is allowed only after archive. The server rechecks historical facts under a row lock and chooses physical deletion or an invisible tombstone atomically; current member Keys are revoked during tombstoning, and restrictive evidence foreign keys remain intact.
  - Member 5h/1d/7d/month limits are aggregate controls shared by all member Keys and use durable reservations; per-Key quota/rate limits remain an additional stricter layer.
  - Consumed-amount corrections must be auditable. Calendar-month corrections are immutable ledger deltas; window projections retain before/after evidence plus a stable system source and note.
  - Bulk member policy updates are limited to current, non-removed members, accept no more than 500 targets, lock targets in deterministic ID order, revalidate both newly selected and retained group authorization in the write transaction, and roll back the complete batch on any version or validation failure.
  - Bulk usage adjustments accept no more than 500 current members and one signed delta per supported window. Expired 5h/1d/7d projections are treated as zero before applying the delta. The transaction locks members and projections in deterministic ID order, writes one calendar-month ledger entry and one before/after audit event per affected member, and requires one request idempotency key to make the whole operation replay-safe. A client must retain that key, target versions, and payload while the result is unknown; successful responses return only the committed member count so idempotency storage remains bounded.
  - Member creation may establish non-zero current-period usage for 5h/1d/7d/month without an extra reason field, while the backend commits the member, group bindings, opening ledger/projections, and system-attributed audit evidence atomically. Calendar-month opening usage is `migration_opening`, not fabricated request usage.
  - Member group delegation must reuse current group authorization semantics: public vs exclusive groups, `users.AllowedGroups`, subscription eligibility, and group fallback behavior.
  - New public import templates omit group IDs. Historical CSV `groups` columns and XLSX `MemberGroups` sheets remain accepted for backward-compatible policy-version-1 jobs and are always server-authorized; policy-version-2 jobs use only the owner-selected system policy, including an intentionally empty selection.
  - Imported monetary opening usage affects the current calendar-month budget through an immutable `migration_opening` ledger entry. Imported aggregate token values are immutable migration baselines and never fabricated into `usage_logs`.
  - Import summaries expose migration baselines separately from native request facts; owner screens may present them side by side but must not silently merge them into request-log totals.
  - Import files may use customer-facing aliases such as `用户名称`, `api key`, `消费金额`, `月限制金额`, and aggregate token headers; normalized server fields remain the stored authority.
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
