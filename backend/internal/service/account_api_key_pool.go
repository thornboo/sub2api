package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

const (
	AccountAPIKeyStatusActive   = "active"
	AccountAPIKeyStatusInactive = "inactive"
	AccountAPIKeyStatusError    = "error"

	DefaultAccountAPIKeyPriority = 1

	legacyAccountAPIKeyID int64 = 0
)

type AccountAPIKey struct {
	ID                   int64
	AccountID            int64
	Name                 string
	APIKey               string
	Priority             int
	Status               string
	ModelRestrictionMode string
	ModelMapping         map[string]string
	GlobalCooldownUntil  *time.Time
	LastUsedAt           *time.Time
	RecentRequestCount   int64
	RecentErrorCount     int64
	ModelCooldowns       map[string]AccountAPIKeyModelCooldown
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type AccountAPIKeyModelCooldown struct {
	UpstreamModel             string
	Reason                    string
	StatusCode                *int
	CooldownUntil             time.Time
	LastErrorAt               time.Time
	LastErrorMessageSanitized string
}

type AccountAPIKeyInput struct {
	ID                   *int64            `json:"id,omitempty"`
	Name                 string            `json:"name"`
	APIKey               string            `json:"api_key"`
	Priority             int               `json:"priority"`
	Status               string            `json:"status"`
	ModelRestrictionMode string            `json:"model_restriction_mode,omitempty"`
	ModelMapping         map[string]string `json:"model_mapping,omitempty"`
}

type AccountAPIKeySelection struct {
	Key           AccountAPIKey
	UpstreamModel string
}

type AccountAPIKeyPoolRepository interface {
	ReplaceAccountAPIKeys(ctx context.Context, accountID int64, keys []AccountAPIKeyInput) error
}

type AccountAPIKeyRuntimeRepository interface {
	SetAccountAPIKeyModelCooldown(ctx context.Context, keyID int64, upstreamModel string, resetAt time.Time, reason string, statusCode int, message string) error
	MarkAccountAPIKeyUsed(ctx context.Context, keyID int64, when time.Time, failed bool) error
}

func (k AccountAPIKey) IsLegacy() bool {
	return k.ID == legacyAccountAPIKeyID
}

func (k AccountAPIKey) IsActive() bool {
	status := strings.TrimSpace(k.Status)
	return status == "" || status == AccountAPIKeyStatusActive
}

// IsSchedulableForModel reports whether this child key can be used for the
// resolved upstream model at the supplied time. The model value must be the
// final upstream model after account/key mapping, because cooldowns are scoped
// to the key plus the upstream model that actually failed.
func (k AccountAPIKey) IsSchedulableForModel(upstreamModel string, now time.Time) bool {
	if !k.IsActive() {
		return false
	}
	if k.GlobalCooldownUntil != nil && now.Before(*k.GlobalCooldownUntil) {
		return false
	}
	modelKey := strings.TrimSpace(upstreamModel)
	if modelKey == "" {
		return true
	}
	if cooldown, ok := k.ModelCooldowns[modelKey]; ok && now.Before(cooldown.CooldownUntil) {
		return false
	}
	return true
}

// ResolveUpstreamModelForRequest applies the child-key whitelist or mapping to
// a request. requestedModel is the site-facing model from the user request;
// accountUpstreamModel is the account-level mapped model. In mapping mode the
// key may override the account-level target. In whitelist mode it only confirms
// the key supports the already resolved upstream model.
func (k AccountAPIKey) ResolveUpstreamModelForRequest(requestedModel string, accountUpstreamModel string) (string, bool) {
	requestedModel = strings.TrimSpace(requestedModel)
	accountUpstreamModel = strings.TrimSpace(accountUpstreamModel)
	if accountUpstreamModel == "" {
		accountUpstreamModel = requestedModel
	}
	mapping := normalizeModelMapping(k.ModelMapping)
	mode := strings.TrimSpace(k.ModelRestrictionMode)
	if mode == "" {
		mode = "whitelist"
	}
	if len(mapping) == 0 {
		return "", false
	}
	switch mode {
	case "mapping":
		if mappedModel, matched := resolveRequestedModelInMapping(mapping, requestedModel); matched {
			return strings.TrimSpace(mappedModel), true
		}
		if mappedModel, matched := resolveRequestedModelInMapping(mapping, accountUpstreamModel); matched {
			return strings.TrimSpace(mappedModel), true
		}
		return "", false
	case "whitelist":
		if modelAllowedByMapping(mapping, accountUpstreamModel) || modelAllowedByMapping(mapping, requestedModel) {
			return accountUpstreamModel, true
		}
		return "", false
	default:
		return accountUpstreamModel, true
	}
}

func normalizeModelMapping(mapping map[string]string) map[string]string {
	if len(mapping) == 0 {
		return nil
	}
	out := make(map[string]string, len(mapping))
	for from, to := range mapping {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			continue
		}
		out[from] = to
	}
	return out
}

func modelAllowedByMapping(mapping map[string]string, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	if _, matched := resolveRequestedModelInMapping(mapping, model); matched {
		return true
	}
	for _, target := range mapping {
		target = strings.TrimSpace(target)
		if target == model || matchWildcard(target, model) {
			return true
		}
	}
	return false
}

// EffectiveAPIKeysForModel returns schedulable keys for a resolved upstream
// model. It is a compatibility wrapper for callers that do not need to inspect
// the per-key resolved upstream model.
func (a *Account) EffectiveAPIKeysForModel(upstreamModel string, now time.Time) []AccountAPIKey {
	selections := a.EffectiveAPIKeySelectionsForRequest(upstreamModel, upstreamModel, now)
	keys := make([]AccountAPIKey, 0, len(selections))
	for _, selection := range selections {
		keys = append(keys, selection.Key)
	}
	return keys
}

// EffectiveAPIKeySelectionsForRequest returns the ordered child-key candidates
// for one request. Ordering is priority first, then least-recently-used, then
// ID. If an account has no child-key pool, it falls back to the legacy
// credentials.api_key. If a child-key pool exists but no key is schedulable, it
// intentionally returns no fallback so the caller can fail over to the next
// account instead of silently using a shared legacy key.
func (a *Account) EffectiveAPIKeySelectionsForRequest(requestedModel string, accountUpstreamModel string, now time.Time) []AccountAPIKeySelection {
	if a == nil {
		return nil
	}
	selections := make([]AccountAPIKeySelection, 0, len(a.APIKeys)+1)
	for _, key := range a.APIKeys {
		if strings.TrimSpace(key.APIKey) == "" {
			continue
		}
		keyUpstreamModel, ok := key.ResolveUpstreamModelForRequest(requestedModel, accountUpstreamModel)
		if !ok {
			continue
		}
		if key.IsSchedulableForModel(keyUpstreamModel, now) {
			selections = append(selections, AccountAPIKeySelection{Key: key, UpstreamModel: keyUpstreamModel})
		}
	}
	if len(selections) == 0 && len(a.APIKeys) == 0 {
		legacy := strings.TrimSpace(a.GetCredential("api_key"))
		if legacy != "" {
			selections = append(selections, AccountAPIKeySelection{
				Key: AccountAPIKey{
					ID:        legacyAccountAPIKeyID,
					AccountID: a.ID,
					Name:      "legacy",
					APIKey:    legacy,
					Priority:  a.Priority,
					Status:    AccountAPIKeyStatusActive,
				},
				UpstreamModel: strings.TrimSpace(accountUpstreamModel),
			})
		}
	}
	sort.SliceStable(selections, func(i, j int) bool {
		a, b := selections[i].Key, selections[j].Key
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		switch {
		case a.LastUsedAt == nil && b.LastUsedAt != nil:
			return true
		case a.LastUsedAt != nil && b.LastUsedAt == nil:
			return false
		case a.LastUsedAt != nil && b.LastUsedAt != nil && !a.LastUsedAt.Equal(*b.LastUsedAt):
			return a.LastUsedAt.Before(*b.LastUsedAt)
		default:
			return a.ID < b.ID
		}
	})
	return selections
}

// HasAccountAPIKeyPool reports whether this account is configured with the new
// child-key pool. A configured-but-empty schedulable set is different from no
// pool: callers should try the next account instead of falling back to the
// legacy account API key.
func (a *Account) HasAccountAPIKeyPool() bool {
	return a != nil && len(a.APIKeys) > 0
}
