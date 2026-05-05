# Merge Log

This file records upstream synchronization work for secondary-development branches.

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
