package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/dgraph-io/ristretto"
	"golang.org/x/sync/singleflight"
)

var (
	ErrAPIKeyNotFound       = infraerrors.NotFound("API_KEY_NOT_FOUND", "api key not found")
	ErrGroupNotAllowed      = infraerrors.Forbidden("GROUP_NOT_ALLOWED", "user is not allowed to bind this group")
	ErrAPIKeyExists         = infraerrors.Conflict("API_KEY_EXISTS", "api key already exists")
	ErrAPIKeyTooShort       = infraerrors.BadRequest("API_KEY_TOO_SHORT", "api key must be at least 16 characters")
	ErrAPIKeyInvalidChars   = infraerrors.BadRequest("API_KEY_INVALID_CHARS", "api key can only contain letters, numbers, underscores, and hyphens")
	ErrAPIKeyRateLimited    = infraerrors.TooManyRequests("API_KEY_RATE_LIMITED", "too many failed attempts, please try again later")
	ErrAPIKeyAuthOverloaded = infraerrors.ServiceUnavailable("API_KEY_AUTH_OVERLOADED", "api key authentication is temporarily overloaded")
	ErrInvalidIPPattern     = infraerrors.BadRequest("INVALID_IP_PATTERN", "invalid IP or CIDR pattern")
	ErrAPIKeyBatchInvalid   = infraerrors.BadRequest("API_KEY_BATCH_INVALID", "invalid api key batch create request")
	ErrAPIKeyTagsInvalid    = infraerrors.BadRequest("API_KEY_TAGS_INVALID", "invalid api key tags")
	ErrAPIKeyStatusInvalid  = infraerrors.BadRequest("API_KEY_STATUS_INVALID", "invalid api key status")
	ErrAPIKeyBatchTooLarge  = infraerrors.BadRequest(
		"API_KEY_BATCH_TOO_LARGE",
		"api key batch count exceeds the allowed limit",
	)
	ErrAPIKeyStatusLookupRateLimited = infraerrors.TooManyRequests(
		"API_KEY_STATUS_LOOKUP_RATE_LIMITED",
		"api key status lookup is too frequent, please try again later",
	)
	ErrAPIKeyStatusLookupUnavailable = infraerrors.ServiceUnavailable(
		"API_KEY_STATUS_LOOKUP_UNAVAILABLE",
		"api key status lookup is temporarily unavailable",
	)
	// ErrAPIKeyExpired        = infraerrors.Forbidden("API_KEY_EXPIRED", "api key has expired")
	ErrAPIKeyExpired = infraerrors.Forbidden("API_KEY_EXPIRED", "api key 已过期")
	// ErrAPIKeyQuotaExhausted = infraerrors.TooManyRequests("API_KEY_QUOTA_EXHAUSTED", "api key quota exhausted")
	ErrAPIKeyQuotaExhausted = infraerrors.TooManyRequests("API_KEY_QUOTA_EXHAUSTED", "api key 额度已用完")

	// Rate limit errors
	ErrAPIKeyRateLimit5hExceeded = infraerrors.TooManyRequests("API_KEY_RATE_5H_EXCEEDED", "api key 5小时限额已用完")
	ErrAPIKeyRateLimit1dExceeded = infraerrors.TooManyRequests("API_KEY_RATE_1D_EXCEEDED", "api key 日限额已用完")
	ErrAPIKeyRateLimit7dExceeded = infraerrors.TooManyRequests("API_KEY_RATE_7D_EXCEEDED", "api key 7天限额已用完")
)

const (
	apiKeyMinimumLength          = 16
	MaxAPIKeyCredentialBytes     = 128
	defaultAuthLookupConcurrency = 64
	defaultNegativeAuthCacheSize = 16384
	apiKeyMaxErrorsPerHour       = 20
	apiKeyLastUsedMinTouch       = 30 * time.Second
	apiKeyNameMaxLength          = 100
	apiKeySortCurrentConcurrency = "current_concurrency"
	// DB 写失败后的短退避，避免请求路径持续同步重试造成写风暴与高延迟。
	apiKeyLastUsedFailBackoff  = 5 * time.Second
	apiKeyStatusLookupCooldown = 10 * time.Second
	apiKeyStatusLookupLocalMax = 4096
)

var ErrAPIKeyManagedByEnterpriseMember = infraerrors.Conflict(
	"API_KEY_MANAGED_BY_ENTERPRISE_MEMBER",
	"enterprise member keys must be managed from enterprise members",
)

type APIKeyRepository interface {
	Create(ctx context.Context, key *APIKey) error
	GetByID(ctx context.Context, id int64) (*APIKey, error)
	// GetKeyAndOwnerID 仅获取 API Key 的 key 与所有者 ID，用于删除等轻量场景
	GetKeyAndOwnerID(ctx context.Context, id int64) (string, int64, error)
	GetByKey(ctx context.Context, key string) (*APIKey, error)
	// GetByKeyForAuth 认证专用查询，返回最小字段集
	GetByKeyForAuth(ctx context.Context, key string) (*APIKey, error)
	Update(ctx context.Context, key *APIKey) error
	Delete(ctx context.Context, id int64) error
	// DeleteWithAudit keeps the legacy interface name for rolling-upgrade compatibility.
	// Implementations must tombstone the key and soft-delete it atomically without
	// retaining the deleted credential material.
	DeleteWithAudit(ctx context.Context, id int64) error

	ListByUserID(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error)
	ListByIDsForUser(ctx context.Context, userID int64, ids []int64) ([]APIKey, error)
	ListTagsByUserID(ctx context.Context, userID int64, limit int) ([]string, error)
	VerifyOwnership(ctx context.Context, userID int64, apiKeyIDs []int64) ([]int64, error)
	CountByUserID(ctx context.Context, userID int64) (int64, error)
	ExistsByKey(ctx context.Context, key string) (bool, error)
	ListByGroupID(ctx context.Context, groupID int64, params pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error)
	SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]APIKey, error)
	// SearchAPIKeysIncludingDeleted is admin-evidence-only: it may return soft-deleted keys
	// so historical usage logs can still resolve their original key labels.
	SearchAPIKeysIncludingDeleted(ctx context.Context, userID int64, keyword string, limit int, includeDeleted bool) ([]APIKey, error)
	ClearGroupIDByGroupID(ctx context.Context, groupID int64) (int64, error)
	// UpdateGroupIDByUserAndGroup 将用户下绑定 oldGroupID 的所有 Key 迁移到 newGroupID
	UpdateGroupIDByUserAndGroup(ctx context.Context, userID, oldGroupID, newGroupID int64) (int64, error)
	CountByGroupID(ctx context.Context, groupID int64) (int64, error)
	ListKeysByUserID(ctx context.Context, userID int64) ([]string, error)
	ListKeysByGroupID(ctx context.Context, groupID int64) ([]string, error)

	// Quota methods
	IncrementQuotaUsed(ctx context.Context, id int64, amount float64) (float64, error)
	UpdateLastUsed(ctx context.Context, id int64, usedAt time.Time) error

	// Rate limit methods
	IncrementRateLimitUsage(ctx context.Context, id int64, cost float64) error
	ResetRateLimitWindows(ctx context.Context, id int64) error
	GetRateLimitData(ctx context.Context, id int64) (*APIKeyRateLimitData, error)
}

type apiKeyIncludingDeletedGetter interface {
	GetByIDIncludingDeleted(ctx context.Context, id int64) (*APIKey, error)
}

type apiKeyAllByUserIDLister interface {
	ListAllByUserID(ctx context.Context, userID int64, filters APIKeyListFilters) ([]APIKey, error)
}

// APIKeyRateLimitData holds rate limit usage and window state for an API key.
type APIKeyRateLimitData struct {
	Usage5h       float64
	Usage1d       float64
	Usage7d       float64
	Window5hStart *time.Time
	Window1dStart *time.Time
	Window7dStart *time.Time
}

// EffectiveUsage5h returns the 5h window usage, or 0 if the window has expired.
func (d *APIKeyRateLimitData) EffectiveUsage5h() float64 {
	if IsWindowExpired(d.Window5hStart, RateLimitWindow5h) {
		return 0
	}
	return d.Usage5h
}

// EffectiveUsage1d returns the 1d window usage, or 0 if the window has expired.
func (d *APIKeyRateLimitData) EffectiveUsage1d() float64 {
	if IsWindowExpired(d.Window1dStart, RateLimitWindow1d) {
		return 0
	}
	return d.Usage1d
}

// EffectiveUsage7d returns the 7d window usage, or 0 if the window has expired.
func (d *APIKeyRateLimitData) EffectiveUsage7d() float64 {
	if IsWindowExpired(d.Window7dStart, RateLimitWindow7d) {
		return 0
	}
	return d.Usage7d
}

// APIKeyQuotaUsageState captures the latest quota fields after an atomic quota update.
// It is intentionally small so repositories can return it from a single SQL statement.
type APIKeyQuotaUsageState struct {
	QuotaUsed float64
	Quota     float64
	Key       string
	Status    string
}

type apiKeyTransactionalRepository interface {
	RunInTx(ctx context.Context, fn func(context.Context) error) error
}

type apiKeyStatusLookupCooldownCache interface {
	ClaimStatusLookupCooldown(ctx context.Context, keyHash string, ttl time.Duration) (bool, error)
}

// APIKeyCache defines cache operations for API key service
type APIKeyCache interface {
	GetCreateAttemptCount(ctx context.Context, userID int64) (int, error)
	IncrementCreateAttemptCount(ctx context.Context, userID int64) error
	DeleteCreateAttemptCount(ctx context.Context, userID int64) error

	IncrementDailyUsage(ctx context.Context, apiKey string) error
	SetDailyUsageExpiry(ctx context.Context, apiKey string, ttl time.Duration) error

	GetAuthCache(ctx context.Context, key string) (*APIKeyAuthCacheEntry, error)
	SetAuthCache(ctx context.Context, key string, entry *APIKeyAuthCacheEntry, ttl time.Duration) error
	DeleteAuthCache(ctx context.Context, key string) error

	// Pub/Sub for L1 cache invalidation across instances
	PublishAuthCacheInvalidation(ctx context.Context, cacheKey string) error
	SubscribeAuthCacheInvalidation(ctx context.Context, handler func(cacheKey string)) error
}

type authCacheSubscriptionReadyKey struct{}

func withAuthCacheSubscriptionReady(ctx context.Context, ready func()) context.Context {
	return context.WithValue(ctx, authCacheSubscriptionReadyKey{}, ready)
}

// NotifyAuthCacheSubscriptionReady lets cache implementations report that the
// server acknowledged the subscription without widening the public cache API.
func NotifyAuthCacheSubscriptionReady(ctx context.Context) {
	if ready, ok := ctx.Value(authCacheSubscriptionReadyKey{}).(func()); ok && ready != nil {
		ready()
	}
}

// APIKeyAuthCacheInvalidator 提供认证缓存失效能力
type APIKeyAuthCacheInvalidator interface {
	InvalidateAuthCacheByKey(ctx context.Context, key string)
	InvalidateAuthCacheByUserID(ctx context.Context, userID int64)
	InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64)
}

// CreateAPIKeyRequest 创建API Key请求
type CreateAPIKeyRequest struct {
	Name        string   `json:"name"`
	Tags        []string `json:"tags"`
	GroupID     *int64   `json:"group_id"`
	MemberID    *int64   `json:"-"`
	CustomKey   *string  `json:"custom_key"`   // 可选的自定义key
	IPWhitelist []string `json:"ip_whitelist"` // IP 白名单
	IPBlacklist []string `json:"ip_blacklist"` // IP 黑名单

	// Quota fields
	Quota         float64 `json:"quota"`           // Quota limit in USD (0 = unlimited)
	ExpiresInDays *int    `json:"expires_in_days"` // Days until expiry (nil = never expires)

	// Rate limit fields (0 = unlimited)
	RateLimit5h float64 `json:"rate_limit_5h"`
	RateLimit1d float64 `json:"rate_limit_1d"`
	RateLimit7d float64 `json:"rate_limit_7d"`
}

// UpdateAPIKeyRequest 更新API Key请求
type UpdateAPIKeyRequest struct {
	Name        *string   `json:"name"`
	Tags        *[]string `json:"tags"`
	GroupID     *int64    `json:"group_id"`
	Status      *string   `json:"status"`
	IPWhitelist []string  `json:"ip_whitelist"` // IP 白名单（空数组清空）
	IPBlacklist []string  `json:"ip_blacklist"` // IP 黑名单（空数组清空）

	// Quota fields
	Quota           *float64   `json:"quota"`       // Quota limit in USD (nil = no change, 0 = unlimited)
	ExpiresAt       *time.Time `json:"expires_at"`  // Expiration time (nil = no change)
	ClearExpiration bool       `json:"-"`           // Clear expiration (internal use)
	ResetQuota      *bool      `json:"reset_quota"` // Reset quota_used to 0

	// Rate limit fields (nil = no change, 0 = unlimited)
	RateLimit5h         *float64 `json:"rate_limit_5h"`
	RateLimit1d         *float64 `json:"rate_limit_1d"`
	RateLimit7d         *float64 `json:"rate_limit_7d"`
	ResetRateLimitUsage *bool    `json:"reset_rate_limit_usage"` // Reset all usage counters to 0
}

// APIKeyService API Key服务
// RateLimitCacheInvalidator invalidates rate limit cache entries on manual reset.
type RateLimitCacheInvalidator interface {
	InvalidateAPIKeyRateLimit(ctx context.Context, keyID int64) error
}

type APIKeyService struct {
	apiKeyRepo                APIKeyRepository
	userRepo                  UserRepository
	groupRepo                 GroupRepository
	userSubRepo               UserSubscriptionRepository
	userGroupRateRepo         UserGroupRateRepository
	cache                     APIKeyCache
	rateLimitCacheInvalid     RateLimitCacheInvalidator // optional: invalidate Redis rate limit cache
	settingService            *SettingService
	concurrencyService        *ConcurrencyService
	cfg                       *config.Config
	authCacheL1               *ristretto.Cache
	authNegativeCacheL1       *ristretto.Cache
	authCfg                   apiKeyAuthCacheConfig
	authGroup                 singleflight.Group
	authLookupSlots           chan struct{}
	authLookupTotal           atomic.Uint64
	authLookupRejected        atomic.Uint64
	authLookupInFlight        atomic.Int64
	invalidAuthAbuse          *invalidAuthAbuseLimiter
	authInvalidationStart     sync.Once
	authInvalidationStop      sync.Once
	authInvalidationCancel    context.CancelFunc
	authInvalidationWG        sync.WaitGroup
	authInvalidationConnected atomic.Bool
	authInvalidationFailures  atomic.Uint64
	lastUsedTouchL1           sync.Map // keyID -> nextAllowedAt(time.Time)
	lastUsedTouchSF           singleflight.Group
	statusLookupL1            sync.Map // keyHash -> nextAllowedAt(time.Time)
	statusLookupJanitorSF     singleflight.Group
}

type APIKeyAuthLookupMetrics struct {
	Total    uint64 `json:"total"`
	Rejected uint64 `json:"rejected"`
	InFlight int64  `json:"in_flight"`
	Capacity int    `json:"capacity"`
}

func (s *APIKeyService) AuthLookupMetrics() APIKeyAuthLookupMetrics {
	if s == nil {
		return APIKeyAuthLookupMetrics{}
	}
	return APIKeyAuthLookupMetrics{
		Total:    s.authLookupTotal.Load(),
		Rejected: s.authLookupRejected.Load(),
		InFlight: s.authLookupInFlight.Load(),
		Capacity: cap(s.authLookupSlots),
	}
}

// NewAPIKeyService 创建API Key服务实例
func NewAPIKeyService(
	apiKeyRepo APIKeyRepository,
	userRepo UserRepository,
	groupRepo GroupRepository,
	userSubRepo UserSubscriptionRepository,
	userGroupRateRepo UserGroupRateRepository,
	cache APIKeyCache,
	cfg *config.Config,
) *APIKeyService {
	svc := &APIKeyService{
		apiKeyRepo:        apiKeyRepo,
		userRepo:          userRepo,
		groupRepo:         groupRepo,
		userSubRepo:       userSubRepo,
		userGroupRateRepo: userGroupRateRepo,
		cache:             cache,
		cfg:               cfg,
	}
	svc.initAuthCache(cfg)
	lookupConcurrency := defaultAuthLookupConcurrency
	if cfg != nil && cfg.APIKeyAuth.LookupConcurrency > 0 {
		lookupConcurrency = cfg.APIKeyAuth.LookupConcurrency
	}
	svc.authLookupSlots = make(chan struct{}, lookupConcurrency)
	svc.invalidAuthAbuse = newInvalidAuthAbuseLimiter(cfg)
	return svc
}

// SetRateLimitCacheInvalidator sets the optional rate limit cache invalidator.
// Called after construction (e.g. in wire) to avoid circular dependencies.
func (s *APIKeyService) SetRateLimitCacheInvalidator(inv RateLimitCacheInvalidator) {
	s.rateLimitCacheInvalid = inv
}

func (s *APIKeyService) SetSettingService(settingService *SettingService) {
	s.settingService = settingService
}

func (s *APIKeyService) SetConcurrencyService(concurrencyService *ConcurrencyService) {
	s.concurrencyService = concurrencyService
}

func (s *APIKeyService) compileAPIKeyIPRules(apiKey *APIKey) {
	if apiKey == nil {
		return
	}
	apiKey.CompiledIPWhitelist = ip.CompileIPRules(apiKey.IPWhitelist)
	apiKey.CompiledIPBlacklist = ip.CompileIPRules(apiKey.IPBlacklist)
}

// GenerateKey 生成随机API Key
func (s *APIKeyService) GenerateKey() (string, error) {
	// 生成32字节随机数据
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	// 转换为十六进制字符串并添加前缀
	prefix := "sk-"
	if s.cfg != nil && s.cfg.Default.APIKeyPrefix != "" {
		prefix = s.cfg.Default.APIKeyPrefix
	}

	key := prefix + hex.EncodeToString(bytes)
	return key, nil
}

// ValidateCustomKey 验证自定义API Key格式
func (s *APIKeyService) ValidateCustomKey(key string) error {
	// 检查长度
	if len(key) < apiKeyMinimumLength {
		return ErrAPIKeyTooShort
	}

	// 检查字符：只允许字母、数字、下划线、连字符
	for _, c := range key {
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '-' {
			continue
		}
		return ErrAPIKeyInvalidChars
	}

	return nil
}

func (s *APIKeyService) GetAPIKeyBatchCreateMaxCount(ctx context.Context) int {
	if s != nil && s.settingService != nil {
		return s.settingService.GetAPIKeyBatchCreateMaxCount(ctx)
	}
	return DefaultAPIKeyBatchCreateMaxCount
}

func validateAPIKeyIPLists(whitelist, blacklist []string) error {
	if len(whitelist) > 0 {
		if invalid := ip.ValidateIPPatterns(whitelist); len(invalid) > 0 {
			return fmt.Errorf("%w: %v", ErrInvalidIPPattern, invalid)
		}
	}
	if len(blacklist) > 0 {
		if invalid := ip.ValidateIPPatterns(blacklist); len(invalid) > 0 {
			return fmt.Errorf("%w: %v", ErrInvalidIPPattern, invalid)
		}
	}
	return nil
}

func validateAPIKeySpendingLimits(quota, rate5h, rate1d, rate7d float64) error {
	if quota < 0 || rate5h < 0 || rate1d < 0 || rate7d < 0 {
		return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{
			"field": "quota_or_rate_limit",
		})
	}
	return nil
}

func normalizeAPIKeyTags(tags []string) ([]string, error) {
	if len(tags) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" {
			continue
		}
		if len([]rune(tag)) > APIKeyTagMaxLength {
			return nil, ErrAPIKeyTagsInvalid.WithMetadata(map[string]string{"field": "tags"})
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	if len(out) > DefaultAPIKeyTagsMaxCount {
		return nil, ErrAPIKeyTagsInvalid.WithMetadata(map[string]string{
			"field":     "tags",
			"max_count": strconv.Itoa(DefaultAPIKeyTagsMaxCount),
		})
	}
	return out, nil
}

func applyAPIKeyTagsMode(current []string, mode string, tags []string) []string {
	normalizedCurrent, _ := normalizeAPIKeyTags(current)
	switch mode {
	case APIKeyBatchTagsModeSet:
		return append([]string(nil), tags...)
	case APIKeyBatchTagsModeClear:
		return []string{}
	case APIKeyBatchTagsModeAdd:
		merged := make([]string, 0, len(normalizedCurrent)+len(tags))
		merged = append(merged, normalizedCurrent...)
		merged = append(merged, tags...)
		out, _ := normalizeAPIKeyTags(merged)
		return out
	case APIKeyBatchTagsModeRemove:
		remove := make(map[string]struct{}, len(tags))
		for _, tag := range tags {
			remove[tag] = struct{}{}
		}
		out := make([]string, 0, len(normalizedCurrent))
		for _, tag := range normalizedCurrent {
			if _, ok := remove[tag]; !ok {
				out = append(out, tag)
			}
		}
		return out
	default:
		return normalizedCurrent
	}
}

func validateBatchAPIKeyName(name string, seen map[string]struct{}) error {
	if name == "" || len([]rune(name)) > apiKeyNameMaxLength {
		return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "names"})
	}
	if _, ok := seen[name]; ok {
		return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "names"})
	}
	seen[name] = struct{}{}
	return nil
}

func buildBatchAPIKeyNames(req BatchCreateAPIKeysRequest) ([]string, error) {
	hasTemplate := req.NameTemplate != nil && strings.TrimSpace(*req.NameTemplate) != ""
	hasNames := len(req.Names) > 0
	if hasTemplate == hasNames {
		return nil, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{
			"field": "name_template_or_names",
		})
	}

	if hasNames {
		if req.Count <= 0 || len(req.Names) != req.Count {
			return nil, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "count"})
		}
		names := make([]string, 0, len(req.Names))
		seen := make(map[string]struct{}, len(req.Names))
		for _, raw := range req.Names {
			name := strings.TrimSpace(raw)
			if err := validateBatchAPIKeyName(name, seen); err != nil {
				return nil, err
			}
			names = append(names, name)
		}
		return names, nil
	}

	if req.Count <= 0 {
		return nil, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "count"})
	}
	template := strings.TrimSpace(*req.NameTemplate)
	if !strings.Contains(template, "{seq}") {
		return nil, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "name_template"})
	}

	width := len(strconv.Itoa(req.Count))
	if width < 3 {
		width = 3
	}
	names := make([]string, 0, req.Count)
	seen := make(map[string]struct{}, req.Count)
	for i := 1; i <= req.Count; i++ {
		seq := fmt.Sprintf("%0*d", width, i)
		name := strings.ReplaceAll(template, "{seq}", seq)
		if err := validateBatchAPIKeyName(name, seen); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// BatchCreate creates API keys as one service-level unit of work. The handler must not loop over Create:
// this method performs shared validation once and persists the whole batch in one transaction.
func (s *APIKeyService) BatchCreate(ctx context.Context, userID int64, req BatchCreateAPIKeysRequest) (*BatchCreateAPIKeysResult, error) {
	names, err := buildBatchAPIKeyNames(req)
	if err != nil {
		return nil, err
	}

	maxAllowed := s.GetAPIKeyBatchCreateMaxCount(ctx)
	if len(names) > maxAllowed {
		return nil, ErrAPIKeyBatchTooLarge.WithMetadata(map[string]string{
			"max_count": strconv.Itoa(maxAllowed),
		})
	}
	if err := validateAPIKeySpendingLimits(req.Quota, req.RateLimit5h, req.RateLimit1d, req.RateLimit7d); err != nil {
		return nil, err
	}
	if err := validateAPIKeyIPLists(req.IPWhitelist, req.IPBlacklist); err != nil {
		return nil, err
	}
	tags, err := normalizeAPIKeyTags(req.Tags)
	if err != nil {
		return nil, err
	}
	req.Tags = tags

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if req.GroupID != nil {
		group, err := s.groupRepo.GetByID(ctx, *req.GroupID)
		if err != nil {
			return nil, fmt.Errorf("get group: %w", err)
		}
		if !s.canUserBindGroup(ctx, user, group) {
			return nil, ErrGroupNotAllowed
		}
	}

	txRepo, ok := s.apiKeyRepo.(apiKeyTransactionalRepository)
	if !ok {
		return nil, infraerrors.InternalServer("API_KEY_TRANSACTION_UNAVAILABLE", "api key repository does not support transactions")
	}

	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, *req.ExpiresInDays)
		expiresAt = &t
	}

	created := make([]APIKey, 0, len(names))
	err = txRepo.RunInTx(ctx, func(txCtx context.Context) error {
		for _, name := range names {
			var createdKey *APIKey
			var lastErr error
			for attempt := 0; attempt < 5; attempt++ {
				key, err := s.GenerateKey()
				if err != nil {
					return fmt.Errorf("generate key: %w", err)
				}
				apiKey := &APIKey{
					UserID:      userID,
					Key:         key,
					Name:        html.EscapeString(name),
					Tags:        req.Tags,
					GroupID:     req.GroupID,
					Status:      StatusActive,
					IPWhitelist: req.IPWhitelist,
					IPBlacklist: req.IPBlacklist,
					Quota:       req.Quota,
					QuotaUsed:   0,
					ExpiresAt:   expiresAt,
					RateLimit5h: req.RateLimit5h,
					RateLimit1d: req.RateLimit1d,
					RateLimit7d: req.RateLimit7d,
				}
				if err := s.apiKeyRepo.Create(txCtx, apiKey); err != nil {
					lastErr = err
					if errors.Is(err, ErrAPIKeyExists) {
						continue
					}
					return fmt.Errorf("create api key: %w", err)
				}
				createdKey = apiKey
				break
			}
			if createdKey == nil {
				if lastErr != nil {
					return fmt.Errorf("create api key: %w", lastErr)
				}
				return ErrAPIKeyExists
			}
			created = append(created, *createdKey)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for i := range created {
		s.InvalidateAuthCacheByKey(ctx, created[i].Key)
		s.compileAPIKeyIPRules(&created[i])
	}

	return &BatchCreateAPIKeysResult{
		Keys:       created,
		Created:    len(created),
		MaxAllowed: maxAllowed,
	}, nil
}

func normalizeAPIKeyBatchIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "ids"})
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "ids"})
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) > HardAPIKeyBatchCreateMaxCount {
		return nil, ErrAPIKeyBatchTooLarge.WithMetadata(map[string]string{
			"max_count": strconv.Itoa(HardAPIKeyBatchCreateMaxCount),
		})
	}
	return out, nil
}

func normalizeAPIKeyBatchApplyTo(applyTo string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(applyTo)) {
	case "", APIKeyBatchApplyToSelected:
		return APIKeyBatchApplyToSelected, nil
	case APIKeyBatchApplyToFiltered:
		return APIKeyBatchApplyToFiltered, nil
	default:
		return "", ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "apply_to"})
	}
}

func normalizeMutableAPIKeyStatus(status string) (string, bool) {
	switch strings.TrimSpace(status) {
	case StatusAPIKeyActive:
		return StatusAPIKeyActive, true
	case StatusAPIKeyDisabled, "inactive":
		return StatusAPIKeyDisabled, true
	default:
		return "", false
	}
}

func normalizeAPIKeyFilterStatus(status string) (string, bool) {
	switch strings.TrimSpace(status) {
	case "":
		return "", true
	case StatusAPIKeyQuotaExhausted:
		return StatusAPIKeyQuotaExhausted, true
	case StatusAPIKeyExpired:
		return StatusAPIKeyExpired, true
	default:
		return normalizeMutableAPIKeyStatus(status)
	}
}

func sameAPIKeyGroupID(current, next *int64) bool {
	if current == nil || next == nil {
		return current == nil && next == nil
	}
	return *current == *next
}

func normalizeAPIKeyBatchFilters(filters APIKeyBatchFilters) (APIKeyListFilters, error) {
	search := strings.TrimSpace(filters.Search)
	if len(search) > 100 {
		search = search[:100]
	}

	status, ok := normalizeAPIKeyFilterStatus(filters.Status)
	if !ok {
		return APIKeyListFilters{}, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "filters.status"})
	}

	if filters.GroupID != nil && *filters.GroupID < 0 {
		return APIKeyListFilters{}, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "filters.group_id"})
	}

	tags, err := normalizeAPIKeyTags(filters.Tags)
	if err != nil {
		return APIKeyListFilters{}, err
	}

	return APIKeyListFilters{
		Search:  search,
		Status:  status,
		GroupID: filters.GroupID,
		Tags:    tags,
	}, nil
}

func hasAPIKeyBatchFilters(filters APIKeyListFilters) bool {
	return filters.Search != "" ||
		filters.Status != "" ||
		filters.GroupID != nil ||
		len(filters.Tags) > 0
}

func (s *APIKeyService) resolveAPIKeyBatchIDs(ctx context.Context, userID int64, ids []int64, applyTo string, filters APIKeyBatchFilters) ([]int64, error) {
	mode, err := normalizeAPIKeyBatchApplyTo(applyTo)
	if err != nil {
		return nil, err
	}
	if mode == APIKeyBatchApplyToSelected {
		return normalizeAPIKeyBatchIDs(ids)
	}

	listFilters, err := normalizeAPIKeyBatchFilters(filters)
	if err != nil {
		return nil, err
	}
	if !hasAPIKeyBatchFilters(listFilters) {
		return nil, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "filters"})
	}

	params := pagination.PaginationParams{
		Page:      1,
		PageSize:  HardAPIKeyBatchCreateMaxCount + 1,
		SortBy:    "id",
		SortOrder: pagination.SortOrderAsc,
	}
	keys, page, err := s.apiKeyRepo.ListByUserID(ctx, userID, params, listFilters)
	if err != nil {
		return nil, fmt.Errorf("list api keys for filtered batch: %w", err)
	}
	if page != nil && page.Total > int64(HardAPIKeyBatchCreateMaxCount) {
		return nil, ErrAPIKeyBatchTooLarge.WithMetadata(map[string]string{
			"max_count": strconv.Itoa(HardAPIKeyBatchCreateMaxCount),
		})
	}
	if len(keys) > HardAPIKeyBatchCreateMaxCount {
		return nil, ErrAPIKeyBatchTooLarge.WithMetadata(map[string]string{
			"max_count": strconv.Itoa(HardAPIKeyBatchCreateMaxCount),
		})
	}

	out := make([]int64, 0, len(keys))
	for _, key := range keys {
		out = append(out, key.ID)
	}
	return out, nil
}

func (s *APIKeyService) listOwnedAPIKeysForBatch(ctx context.Context, userID int64, ids []int64) (map[int64]APIKey, error) {
	keys, err := s.apiKeyRepo.ListByIDsForUser(ctx, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("list api keys for batch: %w", err)
	}
	if len(keys) != len(ids) {
		return nil, ErrInsufficientPerms
	}
	byID := make(map[int64]APIKey, len(keys))
	for _, key := range keys {
		if key.MemberID != nil {
			return nil, ErrAPIKeyManagedByEnterpriseMember
		}
		byID[key.ID] = key
	}
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			return nil, ErrInsufficientPerms
		}
	}
	return byID, nil
}

func validateBatchUpdateRequest(req BatchUpdateAPIKeysRequest) error {
	hasUpdate := req.UpdateGroup ||
		req.UpdateStatus ||
		req.UpdateQuota ||
		req.UpdateExpiration ||
		req.UpdateRateLimit ||
		req.ResetRateLimitUsage ||
		req.UpdateIPAccessControl ||
		req.UpdateTags
	if !hasUpdate {
		return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "updates"})
	}
	if req.UpdateStatus {
		if _, ok := normalizeMutableAPIKeyStatus(req.Status); !ok {
			return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "status"})
		}
	}
	if req.UpdateQuota {
		switch req.QuotaMode {
		case APIKeyBatchQuotaModeSet:
			if req.QuotaValue < 0 {
				return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "quota_value"})
			}
		case APIKeyBatchQuotaModeAdd:
			if req.QuotaValue <= 0 {
				return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "quota_value"})
			}
		case APIKeyBatchQuotaModeUnlimited:
		default:
			return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "quota_mode"})
		}
	}
	if req.UpdateRateLimit {
		if err := validateAPIKeySpendingLimits(0, req.RateLimit5h, req.RateLimit1d, req.RateLimit7d); err != nil {
			return err
		}
	}
	if req.UpdateIPAccessControl {
		if err := validateAPIKeyIPLists(req.IPWhitelist, req.IPBlacklist); err != nil {
			return err
		}
	}
	if req.UpdateTags {
		switch req.TagsMode {
		case APIKeyBatchTagsModeSet, APIKeyBatchTagsModeClear:
		case APIKeyBatchTagsModeAdd, APIKeyBatchTagsModeRemove:
			if len(req.Tags) == 0 {
				return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "tags"})
			}
		default:
			return ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "tags_mode"})
		}
	}
	return nil
}

func applyBatchAPIKeyUpdate(key *APIKey, req BatchUpdateAPIKeysRequest, now time.Time) bool {
	resetRateLimit := false
	if req.UpdateGroup {
		key.GroupID = req.GroupID
	}
	if req.UpdateStatus {
		key.Status = req.Status
		if key.Status == StatusAPIKeyActive || key.Status == StatusAPIKeyDisabled {
			key.DisabledReason = ""
		}
	}
	if req.UpdateQuota {
		switch req.QuotaMode {
		case APIKeyBatchQuotaModeSet:
			key.Quota = req.QuotaValue
		case APIKeyBatchQuotaModeAdd:
			if key.Quota <= 0 {
				key.Quota = req.QuotaValue
			} else {
				key.Quota += req.QuotaValue
			}
		case APIKeyBatchQuotaModeUnlimited:
			key.Quota = 0
		}
		if key.Status == StatusAPIKeyQuotaExhausted && key.Quota > key.QuotaUsed {
			key.Status = StatusActive
		}
	}
	if req.UpdateExpiration {
		key.ExpiresAt = req.ExpiresAt
		if key.Status == StatusAPIKeyExpired && (req.ExpiresAt == nil || now.Before(*req.ExpiresAt)) {
			key.Status = StatusActive
		}
	}
	if req.UpdateRateLimit {
		key.RateLimit5h = req.RateLimit5h
		key.RateLimit1d = req.RateLimit1d
		key.RateLimit7d = req.RateLimit7d
	}
	if req.ResetRateLimitUsage {
		key.Usage5h = 0
		key.Usage1d = 0
		key.Usage7d = 0
		key.Window5hStart = nil
		key.Window1dStart = nil
		key.Window7dStart = nil
		resetRateLimit = true
	}
	if req.UpdateIPAccessControl {
		key.IPWhitelist = req.IPWhitelist
		key.IPBlacklist = req.IPBlacklist
	}
	if req.UpdateTags {
		key.Tags = applyAPIKeyTagsMode(key.Tags, req.TagsMode, req.Tags)
	}
	if key.Status == StatusAPIKeyActive {
		key.DisabledReason = ""
	}
	return resetRateLimit
}

func (s *APIKeyService) BatchUpdate(ctx context.Context, userID int64, req BatchUpdateAPIKeysRequest) (*BatchUpdateAPIKeysResult, error) {
	if req.UpdateTags {
		tags, err := normalizeAPIKeyTags(req.Tags)
		if err != nil {
			return nil, err
		}
		req.Tags = tags
	}
	if req.UpdateStatus {
		status, ok := normalizeMutableAPIKeyStatus(req.Status)
		if !ok {
			return nil, ErrAPIKeyBatchInvalid.WithMetadata(map[string]string{"field": "status"})
		}
		req.Status = status
	}
	if err := validateBatchUpdateRequest(req); err != nil {
		return nil, err
	}
	ids, err := s.resolveAPIKeyBatchIDs(ctx, userID, req.IDs, req.ApplyTo, req.Filters)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return &BatchUpdateAPIKeysResult{Updated: 0}, nil
	}

	if req.UpdateGroup && req.GroupID != nil {
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		group, err := s.groupRepo.GetByID(ctx, *req.GroupID)
		if err != nil {
			return nil, fmt.Errorf("get group: %w", err)
		}
		if !s.canUserBindGroup(ctx, user, group) {
			return nil, ErrGroupNotAllowed
		}
	}

	ownedKeys, err := s.listOwnedAPIKeysForBatch(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	txRepo, ok := s.apiKeyRepo.(apiKeyTransactionalRepository)
	if !ok {
		return nil, infraerrors.InternalServer("API_KEY_TRANSACTION_UNAVAILABLE", "api key repository does not support transactions")
	}

	now := time.Now()
	updatedKeys := make([]APIKey, 0, len(ids))
	resetRateLimitIDs := make([]int64, 0, len(ids))
	err = txRepo.RunInTx(ctx, func(txCtx context.Context) error {
		for _, id := range ids {
			key := ownedKeys[id]
			resetRateLimit := applyBatchAPIKeyUpdate(&key, req, now)
			if err := s.apiKeyRepo.Update(txCtx, &key); err != nil {
				return fmt.Errorf("update api key: %w", err)
			}
			updatedKeys = append(updatedKeys, key)
			if resetRateLimit {
				resetRateLimitIDs = append(resetRateLimitIDs, id)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for i := range updatedKeys {
		s.InvalidateAuthCacheByKey(ctx, updatedKeys[i].Key)
		s.compileAPIKeyIPRules(&updatedKeys[i])
	}
	if s.rateLimitCacheInvalid != nil {
		for _, id := range resetRateLimitIDs {
			_ = s.rateLimitCacheInvalid.InvalidateAPIKeyRateLimit(ctx, id)
		}
	}

	return &BatchUpdateAPIKeysResult{Updated: len(updatedKeys)}, nil
}

func (s *APIKeyService) BatchDelete(ctx context.Context, userID int64, req BatchDeleteAPIKeysRequest) (*BatchDeleteAPIKeysResult, error) {
	ids, err := s.resolveAPIKeyBatchIDs(ctx, userID, req.IDs, req.ApplyTo, req.Filters)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return &BatchDeleteAPIKeysResult{Deleted: 0}, nil
	}
	ownedKeys, err := s.listOwnedAPIKeysForBatch(ctx, userID, ids)
	if err != nil {
		return nil, err
	}
	txRepo, ok := s.apiKeyRepo.(apiKeyTransactionalRepository)
	if !ok {
		return nil, infraerrors.InternalServer("API_KEY_TRANSACTION_UNAVAILABLE", "api key repository does not support transactions")
	}

	deletedKeys := make([]APIKey, 0, len(ids))
	err = txRepo.RunInTx(ctx, func(txCtx context.Context) error {
		for _, id := range ids {
			if err := s.apiKeyRepo.DeleteWithAudit(txCtx, id); err != nil {
				return fmt.Errorf("delete api key: %w", err)
			}
			deletedKeys = append(deletedKeys, ownedKeys[id])
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.DeleteCreateAttemptCount(ctx, userID)
	}
	for i := range deletedKeys {
		s.InvalidateAuthCacheByKey(ctx, deletedKeys[i].Key)
		s.lastUsedTouchL1.Delete(deletedKeys[i].ID)
	}

	return &BatchDeleteAPIKeysResult{Deleted: len(deletedKeys)}, nil
}

func apiKeyLookupHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (s *APIKeyService) claimPublicStatusLookup(ctx context.Context, key string) error {
	keyHash := apiKeyLookupHash(key)
	if s.cache != nil {
		if cache, ok := s.cache.(apiKeyStatusLookupCooldownCache); ok {
			allowed, err := cache.ClaimStatusLookupCooldown(ctx, keyHash, apiKeyStatusLookupCooldown)
			if err != nil {
				return ErrAPIKeyStatusLookupUnavailable.WithCause(err)
			}
			if !allowed {
				return ErrAPIKeyStatusLookupRateLimited.WithMetadata(map[string]string{
					"retry_after": strconv.Itoa(int(apiKeyStatusLookupCooldown.Seconds())),
				})
			}
			return nil
		}
	}

	now := time.Now()
	if v, ok := s.statusLookupL1.Load(keyHash); ok {
		if nextAllowedAt, ok := v.(time.Time); ok && now.Before(nextAllowedAt) {
			return ErrAPIKeyStatusLookupRateLimited.WithMetadata(map[string]string{
				"retry_after": strconv.Itoa(int(nextAllowedAt.Sub(now).Seconds()) + 1),
			})
		}
	}
	s.statusLookupL1.Store(keyHash, now.Add(apiKeyStatusLookupCooldown))
	s.cleanupPublicStatusLookupCooldowns(now)
	return nil
}

func (s *APIKeyService) cleanupPublicStatusLookupCooldowns(now time.Time) {
	_, _, _ = s.statusLookupJanitorSF.Do("cleanup", func() (any, error) {
		count := 0
		s.statusLookupL1.Range(func(k, v any) bool {
			count++
			if nextAllowedAt, ok := v.(time.Time); ok && now.After(nextAllowedAt.Add(apiKeyStatusLookupCooldown)) {
				s.statusLookupL1.Delete(k)
			}
			return count <= apiKeyStatusLookupLocalMax
		})
		if count > apiKeyStatusLookupLocalMax {
			s.statusLookupL1.Range(func(k, v any) bool {
				if nextAllowedAt, ok := v.(time.Time); ok && now.After(nextAllowedAt) {
					s.statusLookupL1.Delete(k)
				}
				return true
			})
		}
		return nil, nil
	})
}

func resetAt(windowStart *time.Time, window time.Duration) *time.Time {
	if IsWindowExpired(windowStart, window) {
		return nil
	}
	t := windowStart.Add(window)
	return &t
}

func (s *APIKeyService) GetPublicStatusByKey(ctx context.Context, key string) (*APIKeyPublicStatus, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrAPIKeyNotFound
	}
	if err := s.claimPublicStatusLookup(ctx, key); err != nil {
		return nil, err
	}

	apiKey, err := s.apiKeyRepo.GetByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	return buildAPIKeyPublicStatus(apiKey), nil
}

// GetPublicStatusByID reloads current status for an already-authorized public
// query session. It intentionally skips the plaintext-key lookup cooldown;
// established sessions have their own route/session rate limits.
func (s *APIKeyService) GetPublicStatusByID(ctx context.Context, id int64) (*APIKeyPublicStatus, error) {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	return buildAPIKeyPublicStatus(apiKey), nil
}

func buildAPIKeyPublicStatus(apiKey *APIKey) *APIKeyPublicStatus {
	if apiKey == nil {
		return nil
	}

	status := apiKey.Status
	isExpired := apiKey.IsExpired()
	isQuotaExhausted := apiKey.IsQuotaExhausted()
	if isExpired {
		status = StatusAPIKeyExpired
	} else if isQuotaExhausted {
		status = StatusAPIKeyQuotaExhausted
	}

	staticAccessAvailable := apiKeyStaticAccessAvailable(apiKey)
	if status == StatusActive && !staticAccessAvailable {
		status = StatusAPIKeyDisabled
	}
	isActive := status == StatusActive && staticAccessAvailable
	out := &APIKeyPublicStatus{
		ID:               apiKey.ID,
		UserID:           apiKey.UserID,
		MemberID:         apiKey.MemberID,
		Name:             apiKey.Name,
		Status:           status,
		GroupID:          apiKey.GroupID,
		Quota:            apiKey.Quota,
		QuotaUsed:        apiKey.QuotaUsed,
		QuotaRemaining:   apiKey.GetQuotaRemaining(),
		ExpiresAt:        apiKey.ExpiresAt,
		LastUsedAt:       apiKey.LastUsedAt,
		CreatedAt:        apiKey.CreatedAt,
		RateLimit5h:      apiKey.RateLimit5h,
		RateLimit1d:      apiKey.RateLimit1d,
		RateLimit7d:      apiKey.RateLimit7d,
		Usage5h:          apiKey.EffectiveUsage5h(),
		Usage1d:          apiKey.EffectiveUsage1d(),
		Usage7d:          apiKey.EffectiveUsage7d(),
		Reset5hAt:        resetAt(apiKey.Window5hStart, RateLimitWindow5h),
		Reset1dAt:        resetAt(apiKey.Window1dStart, RateLimitWindow1d),
		Reset7dAt:        resetAt(apiKey.Window7dStart, RateLimitWindow7d),
		IsActive:         isActive,
		IsExpired:        isExpired,
		IsQuotaExhausted: isQuotaExhausted,
	}
	if apiKey.Group != nil {
		out.GroupName = apiKey.Group.Name
		out.GroupPlatform = apiKey.Group.Platform
	}
	return out
}

// apiKeyStaticAccessAvailable mirrors the gateway's identity and fixed-group
// eligibility checks. Request-specific model, endpoint, subscription, balance,
// and IP decisions remain on the gateway request path and are intentionally not
// represented as a global Key status.
func apiKeyStaticAccessAvailable(apiKey *APIKey) bool {
	if apiKey == nil || apiKey.User == nil || !apiKey.User.IsActive() {
		return false
	}
	if apiKey.MemberID != nil {
		member := apiKey.Member
		if !apiKey.User.IsEnterprise() || member == nil || member.ID != *apiKey.MemberID ||
			member.EnterpriseUserID != apiKey.UserID || member.DeletedAt != nil ||
			member.Status != EnterpriseMemberStatusActive || apiKey.GroupID != nil {
			return false
		}
		for i := range member.Groups {
			if apiKeyGroupStaticAccessAvailable(apiKey.User, &member.Groups[i]) {
				return true
			}
		}
		return false
	}
	if apiKey.GroupID == nil {
		return true
	}
	group := apiKey.Group
	return apiKeyGroupStaticAccessAvailable(apiKey.User, group)
}

func apiKeyGroupStaticAccessAvailable(user *User, group *Group) bool {
	if user == nil || group == nil || !group.IsActive() || !IsGroupContextValid(group) {
		return false
	}
	return group.IsSubscriptionType() || user.CanBindGroup(group.ID, group.IsExclusive)
}

// checkAPIKeyRateLimit 检查用户创建自定义Key的错误次数是否超限
func (s *APIKeyService) checkAPIKeyRateLimit(ctx context.Context, userID int64) error {
	if s.cache == nil {
		return nil
	}

	count, err := s.cache.GetCreateAttemptCount(ctx, userID)
	if err != nil {
		// Redis 出错时不阻止用户操作
		return nil
	}

	if count >= apiKeyMaxErrorsPerHour {
		return ErrAPIKeyRateLimited
	}

	return nil
}

// incrementAPIKeyErrorCount 增加用户创建自定义Key的错误计数
func (s *APIKeyService) incrementAPIKeyErrorCount(ctx context.Context, userID int64) {
	if s.cache == nil {
		return
	}

	_ = s.cache.IncrementCreateAttemptCount(ctx, userID)
}

// canUserBindGroup 检查用户是否可以绑定指定分组
// 对于订阅类型分组：检查用户是否有有效订阅
// 对于标准类型分组：使用原有的 AllowedGroups 和 IsExclusive 逻辑
func (s *APIKeyService) canUserBindGroup(ctx context.Context, user *User, group *Group) bool {
	// 订阅类型分组：需要有效订阅
	if group.IsSubscriptionType() {
		_, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, user.ID, group.ID)
		return err == nil // 有有效订阅则允许
	}
	// 标准类型分组：使用原有逻辑
	return user.CanBindGroup(group.ID, group.IsExclusive)
}

// Create 创建API Key
func (s *APIKeyService) Create(ctx context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error) {
	// 验证用户存在
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// 验证 IP 白名单格式
	if len(req.IPWhitelist) > 0 {
		if invalid := ip.ValidateIPPatterns(req.IPWhitelist); len(invalid) > 0 {
			return nil, fmt.Errorf("%w: %v", ErrInvalidIPPattern, invalid)
		}
	}

	// 验证 IP 黑名单格式
	if len(req.IPBlacklist) > 0 {
		if invalid := ip.ValidateIPPatterns(req.IPBlacklist); len(invalid) > 0 {
			return nil, fmt.Errorf("%w: %v", ErrInvalidIPPattern, invalid)
		}
	}

	// 验证分组权限（如果指定了分组）
	if req.GroupID != nil {
		group, err := s.groupRepo.GetByID(ctx, *req.GroupID)
		if err != nil {
			return nil, fmt.Errorf("get group: %w", err)
		}

		// 检查用户是否可以绑定该分组
		if !s.canUserBindGroup(ctx, user, group) {
			return nil, ErrGroupNotAllowed
		}
	}
	tags, err := normalizeAPIKeyTags(req.Tags)
	if err != nil {
		return nil, err
	}

	var key string

	// 判断是否使用自定义Key
	if req.CustomKey != nil && *req.CustomKey != "" {
		// 检查限流（仅对自定义key进行限流）
		if err := s.checkAPIKeyRateLimit(ctx, userID); err != nil {
			return nil, err
		}

		// 验证自定义Key格式
		if err := s.ValidateCustomKey(*req.CustomKey); err != nil {
			return nil, err
		}

		// 检查Key是否已存在
		exists, err := s.apiKeyRepo.ExistsByKey(ctx, *req.CustomKey)
		if err != nil {
			return nil, fmt.Errorf("check key exists: %w", err)
		}
		if exists {
			// Key已存在，增加错误计数
			s.incrementAPIKeyErrorCount(ctx, userID)
			return nil, ErrAPIKeyExists
		}

		key = *req.CustomKey
	} else {
		// 生成随机API Key
		var err error
		key, err = s.GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("generate key: %w", err)
		}
	}

	// 创建API Key记录
	apiKey := &APIKey{
		UserID:      userID,
		Key:         key,
		Name:        html.EscapeString(req.Name),
		Tags:        tags,
		GroupID:     req.GroupID,
		MemberID:    req.MemberID,
		Status:      StatusActive,
		IPWhitelist: req.IPWhitelist,
		IPBlacklist: req.IPBlacklist,
		Quota:       req.Quota,
		QuotaUsed:   0,
		RateLimit5h: req.RateLimit5h,
		RateLimit1d: req.RateLimit1d,
		RateLimit7d: req.RateLimit7d,
	}

	// Set expiration time if specified
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		expiresAt := time.Now().AddDate(0, 0, *req.ExpiresInDays)
		apiKey.ExpiresAt = &expiresAt
	}

	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	s.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	s.compileAPIKeyIPRules(apiKey)

	return apiKey, nil
}

// List 获取用户的API Key列表
func (s *APIKeyService) List(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	if len(filters.Tags) > 0 {
		tags, err := normalizeAPIKeyTags(filters.Tags)
		if err != nil {
			return nil, nil, err
		}
		filters.Tags = tags
	}
	if normalizedAPIKeySortBy(params.SortBy) == apiKeySortCurrentConcurrency {
		return s.listByCurrentConcurrency(ctx, userID, params, filters)
	}

	keys, pagination, err := s.apiKeyRepo.ListByUserID(ctx, userID, params, filters)
	if err != nil {
		return nil, nil, fmt.Errorf("list api keys: %w", err)
	}
	s.fillCurrentConcurrency(ctx, keys)
	return keys, pagination, nil
}

func (s *APIKeyService) ListTags(ctx context.Context, userID int64) ([]string, error) {
	tags, err := s.apiKeyRepo.ListTagsByUserID(ctx, userID, APIKeyTagOptionsMaxCount)
	if err != nil {
		return nil, fmt.Errorf("list api key tags: %w", err)
	}
	return tags, nil
}

func (s *APIKeyService) listByCurrentConcurrency(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	repo, ok := s.apiKeyRepo.(apiKeyAllByUserIDLister)
	if !ok {
		return nil, nil, fmt.Errorf("list api keys by current concurrency: repository does not support unpaginated API key listing")
	}

	keys, err := repo.ListAllByUserID(ctx, userID, filters)
	if err != nil {
		return nil, nil, fmt.Errorf("list api keys: %w", err)
	}
	s.fillCurrentConcurrency(ctx, keys)
	sortAPIKeysByCurrentConcurrency(keys, params.NormalizedSortOrder(pagination.SortOrderDesc))
	return paginateAPIKeys(keys, params), apiKeyPaginationResult(int64(len(keys)), params), nil
}

func normalizedAPIKeySortBy(sortBy string) string {
	return strings.ToLower(strings.TrimSpace(sortBy))
}

func sortAPIKeysByCurrentConcurrency(keys []APIKey, sortOrder string) {
	desc := sortOrder != pagination.SortOrderAsc
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].CurrentConcurrency == keys[j].CurrentConcurrency {
			if desc {
				return keys[i].ID > keys[j].ID
			}
			return keys[i].ID < keys[j].ID
		}
		if desc {
			return keys[i].CurrentConcurrency > keys[j].CurrentConcurrency
		}
		return keys[i].CurrentConcurrency < keys[j].CurrentConcurrency
	})
}

func paginateAPIKeys(keys []APIKey, params pagination.PaginationParams) []APIKey {
	if len(keys) == 0 {
		return []APIKey{}
	}
	limit := params.Limit()
	page := params.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	if offset >= len(keys) {
		return []APIKey{}
	}
	end := offset + limit
	if end > len(keys) {
		end = len(keys)
	}
	return keys[offset:end]
}

func apiKeyPaginationResult(total int64, params pagination.PaginationParams) *pagination.PaginationResult {
	limit := params.Limit()
	pages := int(total) / limit
	if int(total)%limit > 0 {
		pages++
	}
	return &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: limit,
		Pages:    pages,
	}
}

func (s *APIKeyService) fillCurrentConcurrency(ctx context.Context, keys []APIKey) {
	if s == nil || s.concurrencyService == nil || len(keys) == 0 {
		return
	}
	ids := make([]int64, 0, len(keys))
	for i := range keys {
		if keys[i].ID > 0 {
			ids = append(ids, keys[i].ID)
		}
	}
	counts, err := s.concurrencyService.GetAPIKeyConcurrencyBatch(ctx, ids)
	if err != nil {
		return
	}
	for i := range keys {
		keys[i].CurrentConcurrency = counts[keys[i].ID]
	}
}

func (s *APIKeyService) currentConcurrencyForAPIKey(ctx context.Context, apiKeyID int64) int {
	if s == nil || s.concurrencyService == nil || apiKeyID <= 0 {
		return 0
	}
	counts, err := s.concurrencyService.GetAPIKeyConcurrencyBatch(ctx, []int64{apiKeyID})
	if err != nil {
		return 0
	}
	return counts[apiKeyID]
}

func (s *APIKeyService) VerifyOwnership(ctx context.Context, userID int64, apiKeyIDs []int64) ([]int64, error) {
	if len(apiKeyIDs) == 0 {
		return []int64{}, nil
	}

	validIDs, err := s.apiKeyRepo.VerifyOwnership(ctx, userID, apiKeyIDs)
	if err != nil {
		return nil, fmt.Errorf("verify api key ownership: %w", err)
	}
	return validIDs, nil
}

// GetByID 根据ID获取API Key
func (s *APIKeyService) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	s.compileAPIKeyIPRules(apiKey)
	if apiKey != nil {
		apiKey.CurrentConcurrency = s.currentConcurrencyForAPIKey(ctx, apiKey.ID)
	}
	return apiKey, nil
}

// GetByKey 根据Key字符串获取API Key（用于认证）
func (s *APIKeyService) GetByKey(ctx context.Context, key string) (*APIKey, error) {
	if len(key) == 0 || len(key) > MaxAPIKeyCredentialBytes {
		return nil, ErrAPIKeyNotFound
	}
	cacheKey := s.authCacheKey(key)

	if entry, ok := s.getAuthCacheEntry(ctx, cacheKey); ok {
		if apiKey, used, err := s.applyAuthCacheEntry(key, entry); used {
			if err != nil {
				return nil, fmt.Errorf("get api key: %w", err)
			}
			s.compileAPIKeyIPRules(apiKey)
			return apiKey, nil
		}
	}

	if s.authCfg.singleflight {
		value, err, _ := s.authGroup.Do(cacheKey, func() (any, error) {
			return s.loadAuthCacheEntry(ctx, key, cacheKey)
		})
		if err != nil {
			return nil, err
		}
		entry, _ := value.(*APIKeyAuthCacheEntry)
		if apiKey, used, err := s.applyAuthCacheEntry(key, entry); used {
			if err != nil {
				return nil, fmt.Errorf("get api key: %w", err)
			}
			s.compileAPIKeyIPRules(apiKey)
			return apiKey, nil
		}
	} else {
		entry, err := s.loadAuthCacheEntry(ctx, key, cacheKey)
		if err != nil {
			return nil, err
		}
		if apiKey, used, err := s.applyAuthCacheEntry(key, entry); used {
			if err != nil {
				return nil, fmt.Errorf("get api key: %w", err)
			}
			s.compileAPIKeyIPRules(apiKey)
			return apiKey, nil
		}
	}

	apiKey, err := s.lookupAPIKeyForAuth(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	apiKey.Key = key
	s.compileAPIKeyIPRules(apiKey)
	return apiKey, nil
}

// Update 更新API Key
func (s *APIKeyService) Update(ctx context.Context, id int64, userID int64, req UpdateAPIKeyRequest) (*APIKey, error) {
	return s.update(ctx, id, userID, nil, req)
}

// UpdateEnterpriseMemberKey updates a member-owned key after the caller has
// already established the enterprise owner/member scope. Legacy key handlers
// must continue using Update so member keys remain isolated from that surface.
func (s *APIKeyService) UpdateEnterpriseMemberKey(ctx context.Context, id, userID, memberID int64, req UpdateAPIKeyRequest) (*APIKey, error) {
	return s.update(ctx, id, userID, &memberID, req)
}

func (s *APIKeyService) update(ctx context.Context, id int64, userID int64, expectedMemberID *int64, req UpdateAPIKeyRequest) (*APIKey, error) {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}

	// 验证所有权
	if apiKey.UserID != userID {
		return nil, ErrInsufficientPerms
	}
	if expectedMemberID == nil {
		if apiKey.MemberID != nil {
			return nil, ErrAPIKeyManagedByEnterpriseMember
		}
	} else if apiKey.MemberID == nil || *apiKey.MemberID != *expectedMemberID {
		return nil, ErrAPIKeyNotFound
	}

	// 验证 IP 白名单格式
	if len(req.IPWhitelist) > 0 {
		if invalid := ip.ValidateIPPatterns(req.IPWhitelist); len(invalid) > 0 {
			return nil, fmt.Errorf("%w: %v", ErrInvalidIPPattern, invalid)
		}
	}

	// 验证 IP 黑名单格式
	if len(req.IPBlacklist) > 0 {
		if invalid := ip.ValidateIPPatterns(req.IPBlacklist); len(invalid) > 0 {
			return nil, fmt.Errorf("%w: %v", ErrInvalidIPPattern, invalid)
		}
	}

	// 更新字段
	if req.Name != nil {
		apiKey.Name = html.EscapeString(*req.Name)
	}

	if req.GroupID != nil && !sameAPIKeyGroupID(apiKey.GroupID, req.GroupID) {
		// 验证分组权限
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}

		group, err := s.groupRepo.GetByID(ctx, *req.GroupID)
		if err != nil {
			return nil, fmt.Errorf("get group: %w", err)
		}

		if !s.canUserBindGroup(ctx, user, group) {
			return nil, ErrGroupNotAllowed
		}

		apiKey.GroupID = req.GroupID
	}

	if req.Status != nil {
		status, ok := normalizeMutableAPIKeyStatus(*req.Status)
		if !ok {
			return nil, ErrAPIKeyStatusInvalid.WithMetadata(map[string]string{"field": "status"})
		}
		apiKey.Status = status
		if status == StatusAPIKeyActive || status == StatusAPIKeyDisabled {
			apiKey.DisabledReason = ""
		}
		// 如果状态改变，清除Redis缓存
		if s.cache != nil {
			_ = s.cache.DeleteCreateAttemptCount(ctx, apiKey.UserID)
		}
	}
	if req.Tags != nil {
		tags, err := normalizeAPIKeyTags(*req.Tags)
		if err != nil {
			return nil, err
		}
		apiKey.Tags = tags
	}

	// Update quota fields
	if req.Quota != nil {
		apiKey.Quota = *req.Quota
		// If quota now has room, or is changed to unlimited, reactivate exhausted keys.
		if apiKey.Status == StatusAPIKeyQuotaExhausted && (*req.Quota <= 0 || *req.Quota > apiKey.QuotaUsed) {
			apiKey.Status = StatusActive
		}
	}
	if req.ResetQuota != nil && *req.ResetQuota {
		apiKey.QuotaUsed = 0
		// If resetting quota and status was quota_exhausted, reactivate
		if apiKey.Status == StatusAPIKeyQuotaExhausted {
			apiKey.Status = StatusActive
		}
	}
	if req.ClearExpiration {
		apiKey.ExpiresAt = nil
		// If clearing expiry and status was expired, reactivate
		if apiKey.Status == StatusAPIKeyExpired {
			apiKey.Status = StatusActive
		}
	} else if req.ExpiresAt != nil {
		apiKey.ExpiresAt = req.ExpiresAt
		// If extending expiry and status was expired, reactivate
		if apiKey.Status == StatusAPIKeyExpired && time.Now().Before(*req.ExpiresAt) {
			apiKey.Status = StatusActive
		}
	}

	// 更新 IP 限制（空数组会清空设置）
	apiKey.IPWhitelist = req.IPWhitelist
	apiKey.IPBlacklist = req.IPBlacklist

	// Update rate limit configuration
	if req.RateLimit5h != nil {
		apiKey.RateLimit5h = *req.RateLimit5h
	}
	if req.RateLimit1d != nil {
		apiKey.RateLimit1d = *req.RateLimit1d
	}
	if req.RateLimit7d != nil {
		apiKey.RateLimit7d = *req.RateLimit7d
	}
	resetRateLimit := req.ResetRateLimitUsage != nil && *req.ResetRateLimitUsage
	if resetRateLimit {
		apiKey.Usage5h = 0
		apiKey.Usage1d = 0
		apiKey.Usage7d = 0
		apiKey.Window5hStart = nil
		apiKey.Window1dStart = nil
		apiKey.Window7dStart = nil
	}
	if apiKey.Status == StatusAPIKeyActive {
		apiKey.DisabledReason = ""
	}

	if err := s.apiKeyRepo.Update(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("update api key: %w", err)
	}

	s.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	s.compileAPIKeyIPRules(apiKey)

	// Invalidate Redis rate limit cache so reset takes effect immediately
	if resetRateLimit && s.rateLimitCacheInvalid != nil {
		_ = s.rateLimitCacheInvalid.InvalidateAPIKeyRateLimit(ctx, apiKey.ID)
	}

	return apiKey, nil
}

// Delete 删除API Key
func (s *APIKeyService) Delete(ctx context.Context, id int64, userID int64) error {
	return s.delete(ctx, id, userID, nil)
}

// DeleteEnterpriseMemberKey deletes a member-owned key inside the already
// authorized enterprise-member scope without exposing it to legacy handlers.
func (s *APIKeyService) DeleteEnterpriseMemberKey(ctx context.Context, id, userID, memberID int64) error {
	return s.delete(ctx, id, userID, &memberID)
}

func (s *APIKeyService) delete(ctx context.Context, id int64, userID int64, expectedMemberID *int64) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get api key: %w", err)
	}

	// 验证当前用户是否为该 API Key 的所有者
	if apiKey.UserID != userID {
		return ErrInsufficientPerms
	}
	if expectedMemberID == nil {
		if apiKey.MemberID != nil {
			return ErrAPIKeyManagedByEnterpriseMember
		}
	} else if apiKey.MemberID == nil || *apiKey.MemberID != *expectedMemberID {
		return ErrAPIKeyNotFound
	}

	// 事务内:写审计 + 软删除(tombstone)。
	if err := s.apiKeyRepo.DeleteWithAudit(ctx, id); err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}

	// 删除成功后再清理缓存,避免"缓存已清但删除失败"的竞态。
	if s.cache != nil {
		_ = s.cache.DeleteCreateAttemptCount(ctx, userID)
	}
	s.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	s.lastUsedTouchL1.Delete(id)

	return nil
}

// ValidateKey 验证API Key是否有效（用于认证中间件）
func (s *APIKeyService) ValidateKey(ctx context.Context, key string) (*APIKey, *User, error) {
	// 获取API Key
	apiKey, err := s.GetByKey(ctx, key)
	if err != nil {
		return nil, nil, err
	}

	// 检查API Key状态
	if !apiKey.IsActive() {
		return nil, nil, infraerrors.Unauthorized("API_KEY_INACTIVE", "api key is not active")
	}

	// 获取用户信息
	user, err := s.userRepo.GetByID(ctx, apiKey.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("get user: %w", err)
	}

	// 检查用户状态
	if !user.IsActive() {
		return nil, nil, ErrUserNotActive
	}

	return apiKey, user, nil
}

// TouchLastUsed 通过防抖更新 api_keys.last_used_at，减少高频写放大。
// 该操作为尽力而为，不应阻塞主请求链路。
func (s *APIKeyService) TouchLastUsed(ctx context.Context, keyID int64) error {
	if keyID <= 0 {
		return nil
	}

	now := time.Now()
	if v, ok := s.lastUsedTouchL1.Load(keyID); ok {
		if nextAllowedAt, ok := v.(time.Time); ok && now.Before(nextAllowedAt) {
			return nil
		}
	}

	_, err, _ := s.lastUsedTouchSF.Do(strconv.FormatInt(keyID, 10), func() (any, error) {
		latest := time.Now()
		if v, ok := s.lastUsedTouchL1.Load(keyID); ok {
			if nextAllowedAt, ok := v.(time.Time); ok && latest.Before(nextAllowedAt) {
				return nil, nil
			}
		}

		if err := s.apiKeyRepo.UpdateLastUsed(ctx, keyID, latest); err != nil {
			s.lastUsedTouchL1.Store(keyID, latest.Add(apiKeyLastUsedFailBackoff))
			return nil, fmt.Errorf("touch api key last used: %w", err)
		}
		s.lastUsedTouchL1.Store(keyID, latest.Add(apiKeyLastUsedMinTouch))
		return nil, nil
	})
	return err
}

// IncrementUsage 增加API Key使用次数（可选：用于统计）
func (s *APIKeyService) IncrementUsage(ctx context.Context, keyID int64) error {
	// 使用Redis计数器
	if s.cache != nil {
		cacheKey := fmt.Sprintf("apikey:usage:%d:%s", keyID, timezone.Now().Format("2006-01-02"))
		if err := s.cache.IncrementDailyUsage(ctx, cacheKey); err != nil {
			return fmt.Errorf("increment usage: %w", err)
		}
		// 设置24小时过期
		_ = s.cache.SetDailyUsageExpiry(ctx, cacheKey, 24*time.Hour)
	}
	return nil
}

// GetAvailableGroups 获取用户有权限绑定的分组列表
// 返回用户可以选择的分组：
// - 标准类型分组：公开的（非专属）或用户被明确允许的
// - 订阅类型分组：用户有有效订阅的
func (s *APIKeyService) GetAvailableGroups(ctx context.Context, userID int64) ([]Group, error) {
	// 获取用户信息
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// 获取所有活跃分组
	allGroups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}

	// 获取用户的所有有效订阅
	activeSubscriptions, err := s.userSubRepo.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active subscriptions: %w", err)
	}

	// 构建订阅分组 ID 集合
	subscribedGroupIDs := make(map[int64]bool)
	for _, sub := range activeSubscriptions {
		subscribedGroupIDs[sub.GroupID] = true
	}

	// 过滤出用户有权限的分组
	availableGroups := make([]Group, 0)
	for _, group := range allGroups {
		if s.canUserBindGroupInternal(user, &group, subscribedGroupIDs) {
			availableGroups = append(availableGroups, group)
		}
	}

	return availableGroups, nil
}

// canUserBindGroupInternal 内部方法，检查用户是否可以绑定分组（使用预加载的订阅数据）
func (s *APIKeyService) canUserBindGroupInternal(user *User, group *Group, subscribedGroupIDs map[int64]bool) bool {
	// 订阅类型分组：需要有效订阅
	if group.IsSubscriptionType() {
		return subscribedGroupIDs[group.ID]
	}
	// 标准类型分组：使用原有逻辑
	return user.CanBindGroup(group.ID, group.IsExclusive)
}

func (s *APIKeyService) SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]APIKey, error) {
	keys, err := s.apiKeyRepo.SearchAPIKeys(ctx, userID, keyword, limit)
	if err != nil {
		return nil, fmt.Errorf("search api keys: %w", err)
	}
	return keys, nil
}

func (s *APIKeyService) SearchAPIKeysIncludingDeleted(ctx context.Context, userID int64, keyword string, limit int, includeDeleted bool) ([]APIKey, error) {
	keys, err := s.apiKeyRepo.SearchAPIKeysIncludingDeleted(ctx, userID, keyword, limit, includeDeleted)
	if err != nil {
		return nil, fmt.Errorf("search api keys including deleted: %w", err)
	}
	return keys, nil
}

func (s *APIKeyService) GetByIDIncludingDeleted(ctx context.Context, id int64) (*APIKey, error) {
	getter, ok := s.apiKeyRepo.(apiKeyIncludingDeletedGetter)
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	key, err := getter.GetByIDIncludingDeleted(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get api key including deleted: %w", err)
	}
	return key, nil
}

// GetUserGroupRates 获取用户的专属分组倍率配置
// 返回 map[groupID]rateMultiplier
func (s *APIKeyService) GetUserGroupRates(ctx context.Context, userID int64) (map[int64]float64, error) {
	if s.userGroupRateRepo == nil {
		return nil, nil
	}
	rates, err := s.userGroupRateRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user group rates: %w", err)
	}
	return rates, nil
}

// CheckAPIKeyQuotaAndExpiry checks if the API key is valid for use (not expired, quota not exhausted)
// Returns nil if valid, error if invalid
func (s *APIKeyService) CheckAPIKeyQuotaAndExpiry(apiKey *APIKey) error {
	// Check expiration
	if apiKey.IsExpired() {
		return ErrAPIKeyExpired
	}

	// Check quota
	if apiKey.IsQuotaExhausted() {
		return ErrAPIKeyQuotaExhausted
	}

	return nil
}

// UpdateQuotaUsed updates the quota_used field after a request
// Also checks if quota is exhausted and updates status accordingly
func (s *APIKeyService) UpdateQuotaUsed(ctx context.Context, apiKeyID int64, cost float64) error {
	if cost <= 0 {
		return nil
	}

	type quotaStateReader interface {
		IncrementQuotaUsedAndGetState(ctx context.Context, id int64, amount float64) (*APIKeyQuotaUsageState, error)
	}

	if repo, ok := s.apiKeyRepo.(quotaStateReader); ok {
		state, err := repo.IncrementQuotaUsedAndGetState(ctx, apiKeyID, cost)
		if err != nil {
			return fmt.Errorf("increment quota used: %w", err)
		}
		if state != nil && state.Status == StatusAPIKeyQuotaExhausted && strings.TrimSpace(state.Key) != "" {
			s.InvalidateAuthCacheByKey(ctx, state.Key)
		}
		return nil
	}

	// Use repository to atomically increment quota_used
	newQuotaUsed, err := s.apiKeyRepo.IncrementQuotaUsed(ctx, apiKeyID, cost)
	if err != nil {
		return fmt.Errorf("increment quota used: %w", err)
	}

	// Check if quota is now exhausted and update status if needed
	apiKey, err := s.apiKeyRepo.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil // Don't fail the request, just log
	}

	// If quota is set and now exhausted, update status
	if apiKey.Quota > 0 && newQuotaUsed >= apiKey.Quota {
		apiKey.Status = StatusAPIKeyQuotaExhausted
		if err := s.apiKeyRepo.Update(ctx, apiKey); err != nil {
			return nil // Don't fail the request
		}
		// Invalidate cache so next request sees the new status
		s.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	}

	return nil
}

// GetRateLimitData returns rate limit usage and window state for an API key.
func (s *APIKeyService) GetRateLimitData(ctx context.Context, id int64) (*APIKeyRateLimitData, error) {
	return s.apiKeyRepo.GetRateLimitData(ctx, id)
}

// UpdateRateLimitUsage atomically increments rate limit usage counters in the DB.
func (s *APIKeyService) UpdateRateLimitUsage(ctx context.Context, apiKeyID int64, cost float64) error {
	if cost <= 0 {
		return nil
	}
	return s.apiKeyRepo.IncrementRateLimitUsage(ctx, apiKeyID, cost)
}
