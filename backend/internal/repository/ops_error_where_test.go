package repository

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsErrorLogsWhere_UserScopedFilters(t *testing.T) {
	uid := int64(42)
	kid := int64(7)
	filter := &service.OpsErrorLogFilter{
		UserID:             &uid,
		APIKeyID:           &kid,
		Model:              "claude-sonnet-4-5",
		ExcludeCountTokens: true,
		ErrorPhasesAny:     []string{"auth"},
		ErrorTypesAny:      []string{"rate_limit_error"},
		View:               "all",
	}
	where, args := buildOpsErrorLogsWhere(filter)

	for _, want := range []string{
		"e.user_id = $",
		"e.api_key_id = $",
		"COALESCE(e.requested_model, e.model, '') = $",
		"COALESCE(e.is_count_tokens, false) = false",
		"e.error_phase = ANY($",
		"e.error_type = ANY($",
	} {
		if !strings.Contains(where, want) {
			t.Fatalf("where missing %q\nfull: %s", want, where)
		}
	}
	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d", len(args))
	}
}

func TestBuildOpsErrorLogsWhere_AssignedMembersExcludeRemovedTombstones(t *testing.T) {
	uid := int64(42)
	where, args := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		UserID:      &uid,
		MemberScope: "assigned",
		View:        "all",
	})

	for _, want := range []string{
		"e.member_id IS NOT NULL",
		"visible_member.id = e.member_id",
		"visible_member.enterprise_user_id = e.user_id",
		"visible_member.removed_at IS NULL",
	} {
		if !strings.Contains(where, want) {
			t.Fatalf("assigned member error scope missing %q\nfull: %s", want, where)
		}
	}
	if len(args) != 1 || args[0] != uid {
		t.Fatalf("expected only user id arg %d, got %v", uid, args)
	}
	if strings.Contains(where, "visible_member.deleted_at") {
		t.Fatalf("archived members must remain in owner-visible error history\nfull: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_ModelFuzzy(t *testing.T) {
	// 默认（ModelFuzzy=false）保持精确匹配
	exact := &service.OpsErrorLogFilter{Model: "claude"}
	whereExact, _ := buildOpsErrorLogsWhere(exact)
	if !strings.Contains(whereExact, "COALESCE(e.requested_model, e.model, '') = $") {
		t.Fatalf("default should be exact match, got: %s", whereExact)
	}

	// ModelFuzzy=true → ILIKE
	fuzzy := &service.OpsErrorLogFilter{Model: "claude", ModelFuzzy: true}
	whereFuzzy, args := buildOpsErrorLogsWhere(fuzzy)
	if !strings.Contains(whereFuzzy, "COALESCE(e.requested_model, e.model, '') ILIKE $") {
		t.Fatalf("ModelFuzzy should use ILIKE, got: %s", whereFuzzy)
	}
	if len(args) != 1 || args[0] != "%claude%" {
		t.Fatalf("expected arg \"%%claude%%\", got %v", args)
	}

	// 通配符转义：输入含 % 应被转义为字面量
	esc := &service.OpsErrorLogFilter{Model: "50%off", ModelFuzzy: true}
	_, escArgs := buildOpsErrorLogsWhere(esc)
	if len(escArgs) != 1 || escArgs[0] != `%50\%off%` {
		t.Fatalf("expected escaped arg, got %v", escArgs)
	}

	esc2 := &service.OpsErrorLogFilter{Model: "gpt_4o", ModelFuzzy: true}
	_, escArgs2 := buildOpsErrorLogsWhere(esc2)
	if len(escArgs2) != 1 || escArgs2[0] != `%gpt\_4o%` {
		t.Fatalf("underscore should be escaped, got %v", escArgs2)
	}
}

// TestBuildOpsErrorLogsWhere_CyberPolicyStatusExemption verifies that legacy
// HTTP 200 terminal stream failures remain visible while recovered upstream
// attempts do not leak into customer-failure lists.
func TestBuildOpsErrorLogsWhere_CyberPolicyStatusExemption(t *testing.T) {
	cyberFallback := "LOWER(COALESCE(e.error_type, '')) IN ('cyber_policy', 'cyber_policy_session_blocked')"
	recoveredMarker := "LOWER(COALESCE(e.error_message, '')) LIKE 'recovered %'"

	// Default filter (no phase) must include both cyber-policy variants and the
	// strict stream-terminal compatibility predicate.
	where, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{})
	if !strings.Contains(where, cyberFallback) {
		t.Fatalf("default filter must exempt both cyber-policy outcomes from status >= 400 guard\nfull: %s", where)
	}
	if !strings.Contains(where, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("default filter must still include the status >= 400 guard for non-cyber rows\nfull: %s", where)
	}
	if !strings.Contains(where, "COALESCE(e.stream, FALSE)") || !strings.Contains(where, recoveredMarker) {
		t.Fatalf("default filter must include non-recovered HTTP 200 stream-terminal failures\nfull: %s", where)
	}
	visible := true
	whereVisible, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{CustomerVisible: &visible})
	if strings.Count(whereVisible, "COALESCE(e.customer_visible") != 2 || strings.Count(whereVisible, cyberFallback) < 2 {
		t.Fatalf("default and explicit customer-visible filters must share the cyber-policy fallback\nfull: %s", whereVisible)
	}

	// phase=upstream WITHOUT the recovered-upstream opt-in keeps the status guard:
	// request-error list endpoints filter by phase=upstream as a plain condition.
	whereUpstream, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{Phase: "upstream"})
	if !strings.Contains(whereUpstream, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("upstream phase without IncludeRecoveredUpstream must keep the status guard\nfull: %s", whereUpstream)
	}
	if !strings.Contains(whereUpstream, "e.error_phase = $") {
		t.Fatalf("upstream phase filter must emit the error_phase condition\nfull: %s", whereUpstream)
	}

	// phase=upstream WITH IncludeRecoveredUpstream (ops 上游列表) skips the guard,
	// exposing recovered (<400) upstream rows.
	whereRecovered, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{Phase: "upstream", IncludeRecoveredUpstream: true})
	if strings.Contains(whereRecovered, "status_code") {
		t.Fatalf("upstream phase with IncludeRecoveredUpstream must not add any status_code clause\nfull: %s", whereRecovered)
	}

	// account_auth uses the same explicit provider-health opt-in but remains a
	// distinct phase from inference upstream errors.
	whereAccountAuth, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{Phase: "account_auth", IncludeRecoveredUpstream: true})
	if strings.Contains(whereAccountAuth, "status_code") {
		t.Fatalf("account_auth phase with IncludeRecoveredUpstream must expose recovered rows\nfull: %s", whereAccountAuth)
	}
	if !strings.Contains(whereAccountAuth, "e.error_phase = $") {
		t.Fatalf("account_auth recovered filter must retain its explicit phase\nfull: %s", whereAccountAuth)
	}

	whereProviderHealth, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		ErrorPhasesAny:           []string{"upstream", "account_auth"},
		IncludeRecoveredUpstream: true,
	})
	if strings.Contains(whereProviderHealth, "status_code") {
		t.Fatalf("provider-health ANY filter must expose recovered inference and credential rows\nfull: %s", whereProviderHealth)
	}
	if !strings.Contains(whereProviderHealth, "e.error_phase = ANY($") {
		t.Fatalf("provider-health filter must preserve distinct phase values\nfull: %s", whereProviderHealth)
	}

	whereUserAccountAuth, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{ErrorPhasesAny: []string{"account_auth"}})
	if !strings.Contains(whereUserAccountAuth, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("request-error account_auth filters must exclude recovered successes\nfull: %s", whereUserAccountAuth)
	}

	whereMixed, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		ErrorPhasesAny:           []string{"account_auth", "request"},
		IncludeRecoveredUpstream: true,
	})
	if !strings.Contains(whereMixed, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("recovered opt-in must not bypass the guard for non-provider phases\nfull: %s", whereMixed)
	}
}

func TestBuildOpsErrorLogsWhere_StatusCodesExclude(t *testing.T) {
	where, args := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		Owner:              "provider",
		StatusCodesExclude: []int{429, 529},
	})

	if strings.Contains(where, "e.error_phase =") {
		t.Fatalf("provider-scope upstream filters should not force a single phase, got: %s", where)
	}
	if !strings.Contains(where, "NOT (COALESCE(e.upstream_status_code, e.status_code, 0) = ANY($") {
		t.Fatalf("where should exclude selected status codes, got: %s", where)
	}
	if len(args) != 2 {
		t.Fatalf("expected owner and excluded status args, got %d: %v", len(args), args)
	}
}

// TestBuildOpsErrorLogsWhere_StatusCodesWithExclude locks the placeholder binding
// order when include and exclude filters are combined. Each clause computes its $N
// from len(args) at append time, so a future reorder could silently misbind without
// this guard. Expected ordering: owner=$1, include=$2, exclude=$3.
func TestBuildOpsErrorLogsWhere_StatusCodesWithExclude(t *testing.T) {
	where, args := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		Owner:              "provider",
		StatusCodes:        []int{400, 500},
		StatusCodesExclude: []int{500},
	})

	if !strings.Contains(where, "COALESCE(e.upstream_status_code, e.status_code, 0) = ANY($2)") {
		t.Fatalf("included status codes should bind to $2, got: %s", where)
	}
	if !strings.Contains(where, "NOT (COALESCE(e.upstream_status_code, e.status_code, 0) = ANY($3))") {
		t.Fatalf("excluded status codes should bind to $3, got: %s", where)
	}
	if len(args) != 3 {
		t.Fatalf("expected owner, included and excluded status args, got %d: %v", len(args), args)
	}
}

func TestBuildOpsErrorLogsWhere_MatchDeletedKeyOwner(t *testing.T) {
	uid := int64(42)

	// 开关开启 → 归属放宽为 OR(user_id 或 deleted_key_owner_user_id),且共用同一占位符
	on := &service.OpsErrorLogFilter{UserID: &uid, MatchDeletedKeyOwner: true}
	whereOn, argsOn := buildOpsErrorLogsWhere(on)
	if !strings.Contains(whereOn, "(e.user_id = $1 OR e.deleted_key_owner_user_id = $1)") {
		t.Fatalf("MatchDeletedKeyOwner=true should widen to OR, got: %s", whereOn)
	}
	if len(argsOn) != 1 || argsOn[0] != uid {
		t.Fatalf("expected single reused arg %d, got %v", uid, argsOn)
	}

	// 开关关闭(默认)→ 仅精确 user_id,绝不出现 deleted_key_owner_user_id(admin 回归)
	off := &service.OpsErrorLogFilter{UserID: &uid}
	whereOff, _ := buildOpsErrorLogsWhere(off)
	if !strings.Contains(whereOff, "e.user_id = $1") {
		t.Fatalf("default should match user_id exactly, got: %s", whereOff)
	}
	if strings.Contains(whereOff, "deleted_key_owner_user_id") {
		t.Fatalf("default must NOT include deleted_key_owner_user_id, got: %s", whereOff)
	}
}

func TestBuildOpsErrorLogsWhere_NonRoutingBreakdownMatchesItsAggregate(t *testing.T) {
	where, args := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		FailureCategory: service.OpsFailureBreakdownCategoryNonRouting,
		View:            "all",
	})

	if !strings.Contains(where, "e.failure_domain = 'platform'") {
		t.Fatalf("virtual non-routing category must enforce the platform domain: %s", where)
	}
	if !strings.Contains(where, "e.failure_category <> 'routing_capacity'") {
		t.Fatalf("non-routing filter must include dependency/internal/etc. without routing rows: %s", where)
	}
	if len(args) != 0 {
		t.Fatalf("virtual category must not consume a placeholder, got %v", args)
	}
}

func TestBuildOpsErrorLogsWhere_StructuredFiltersUseIndexableClosedSetEquality(t *testing.T) {
	where, args := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		EventScope:      "REQUEST_TERMINAL",
		FailureDomain:   "PLATFORM",
		FailureCategory: "DEPENDENCY",
		FailureReason:   "DATABASE_UNAVAILABLE",
		ResolutionOwner: "PLATFORM_OPS",
		PoolOwnership:   "PLATFORM",
		View:            "all",
	})

	for _, clause := range []string{
		"e.event_scope = $1",
		"e.failure_domain = $2",
		"e.failure_category = $3",
		"e.failure_reason = $4",
		"e.resolution_owner = $5",
		"e.pool_ownership = $6",
	} {
		if !strings.Contains(where, clause) {
			t.Fatalf("missing indexable structured filter %q: %s", clause, where)
		}
	}
	if strings.Contains(where, "LOWER(COALESCE(e.failure_") {
		t.Fatalf("structured closed-set filters must not wrap indexed columns: %s", where)
	}
	wantArgs := []any{"request_terminal", "platform", "dependency", "database_unavailable", "platform_ops", "platform"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("structured filters must normalize closed-set values, want %v got %v", wantArgs, args)
	}
}
