package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	BillingTypeBalance      int8 = 0 // 钱包余额
	BillingTypeSubscription int8 = 1 // 订阅套餐
)

type RequestType int16

const (
	RequestTypeUnknown      RequestType = 0
	RequestTypeSync         RequestType = 1
	RequestTypeStream       RequestType = 2
	RequestTypeWSV2         RequestType = 3
	RequestTypeCyberBlocked RequestType = 4 // cyber_policy 命中（透传但被上游安全策略拒绝）
)

func (t RequestType) IsValid() bool {
	switch t {
	case RequestTypeUnknown, RequestTypeSync, RequestTypeStream, RequestTypeWSV2, RequestTypeCyberBlocked:
		return true
	default:
		return false
	}
}

func (t RequestType) Normalize() RequestType {
	if t.IsValid() {
		return t
	}
	return RequestTypeUnknown
}

func (t RequestType) String() string {
	switch t.Normalize() {
	case RequestTypeSync:
		return "sync"
	case RequestTypeStream:
		return "stream"
	case RequestTypeWSV2:
		return "ws_v2"
	case RequestTypeCyberBlocked:
		return "cyber"
	default:
		return "unknown"
	}
}

func RequestTypeFromInt16(v int16) RequestType {
	return RequestType(v).Normalize()
}

func ParseUsageRequestType(value string) (RequestType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "unknown":
		return RequestTypeUnknown, nil
	case "sync":
		return RequestTypeSync, nil
	case "stream":
		return RequestTypeStream, nil
	case "ws_v2":
		return RequestTypeWSV2, nil
	case "cyber":
		return RequestTypeCyberBlocked, nil
	default:
		return RequestTypeUnknown, fmt.Errorf("invalid request_type, allowed values: unknown, sync, stream, ws_v2, cyber")
	}
}

func RequestTypeFromLegacy(stream bool, openAIWSMode bool) RequestType {
	if openAIWSMode {
		return RequestTypeWSV2
	}
	if stream {
		return RequestTypeStream
	}
	return RequestTypeSync
}

func ApplyLegacyRequestFields(requestType RequestType, fallbackStream bool, fallbackOpenAIWSMode bool) (stream bool, openAIWSMode bool) {
	switch requestType.Normalize() {
	case RequestTypeSync:
		return false, false
	case RequestTypeStream:
		return true, false
	case RequestTypeWSV2:
		return true, true
	default:
		return fallbackStream, fallbackOpenAIWSMode
	}
}

type UsageLog struct {
	ID        int64
	UserID    int64
	APIKeyID  int64
	AccountID int64
	RequestID string
	Model     string
	// RequestedModel is the client-requested model name recorded for stable user/admin display.
	// Empty should be treated as Model for backward compatibility with historical rows.
	RequestedModel string
	// UpstreamModel is the actual model sent to the upstream provider after mapping.
	// Nil means no mapping was applied (requested model was used as-is).
	UpstreamModel *string
	// ChannelID 渠道 ID
	ChannelID *int64
	// ModelMappingChain 模型映射链，如 "a→b→c"
	ModelMappingChain *string
	// BillingTier 计费层级标签（per_request/image 模式）
	BillingTier *string
	// BillingMode 计费模式：token/image
	BillingMode *string
	// ServiceTier records the OpenAI service tier used for billing, e.g. "priority" / "flex".
	ServiceTier *string
	// ReasoningEffort is the request's reasoning effort level.
	// OpenAI: "low" / "medium" / "high" / "xhigh"; Claude: "low" / "medium" / "high" / "max".
	// Nil means not provided / not applicable.
	ReasoningEffort *string
	// InboundEndpoint is the client-facing API endpoint path, e.g. /v1/chat/completions.
	InboundEndpoint *string
	// UpstreamEndpoint is the normalized upstream endpoint path, e.g. /v1/responses.
	UpstreamEndpoint *string
	// ScheduleMeta records non-sensitive scheduler diagnostics for admin troubleshooting.
	ScheduleMeta *UsageScheduleMeta

	GroupID            *int64
	SubscriptionID     *int64
	MemberID           *int64
	MemberCodeSnapshot *string
	MemberNameSnapshot *string

	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int

	CacheCreation5mTokens int `gorm:"column:cache_creation_5m_tokens"`
	CacheCreation1hTokens int `gorm:"column:cache_creation_1h_tokens"`

	ImageInputTokens  int
	ImageInputCost    float64
	ImageOutputTokens int
	ImageOutputCost   float64

	InputCost                 float64
	OutputCost                float64
	CacheCreationCost         float64
	CacheReadCost             float64
	TotalCost                 float64
	ActualCost                float64
	RateMultiplier            float64
	LongContextBillingApplied bool
	// AccountRateMultiplier 账号计费倍率快照（nil 表示历史数据，按 1.0 处理）
	AccountRateMultiplier *float64
	// AccountStatsCost 账号统计定价预计算费用（nil = 使用默认公式 total_cost × account_rate_multiplier）
	AccountStatsCost *float64

	BillingType  int8
	RequestType  RequestType
	Stream       bool
	OpenAIWSMode bool
	DurationMs   *int
	FirstTokenMs *int
	UserAgent    *string
	IPAddress    *string

	// Cache TTL Override 标记（管理员强制替换了缓存 TTL 计费）
	CacheTTLOverridden bool

	// 图片生成字段
	ImageCount         int
	ImageSize          *string
	ImageInputSize     *string
	ImageOutputSize    *string
	ImageSizeSource    *string
	ImageSizeBreakdown map[string]int
	MediaType          *string

	// 视频生成字段（Grok 视频按秒计费；video_count>0 的行不要求 image_size）
	VideoCount           int
	VideoResolution      *string
	VideoDurationSeconds *int

	CreatedAt time.Time

	// Repository relationship objects are read-side hydration only. Never put
	// them into durable settlement payloads: APIKey can contain plaintext secret
	// material and the graphs are not part of the immutable usage fact.
	User         *User             `json:"-"`
	APIKey       *APIKey           `json:"-"`
	Account      *Account          `json:"-"`
	Group        *Group            `json:"-"`
	Subscription *UserSubscription `json:"-"`
}

// applyAPIKeyUsageAttribution copies the immutable enterprise-member identity
// snapshot carried by the API key onto a usage fact. Keeping this in one helper
// prevents provider-specific recorders from drifting away from the generic
// gateway path.
func applyAPIKeyUsageAttribution(log *UsageLog, apiKey *APIKey) {
	if log == nil || apiKey == nil || apiKey.MemberID == nil {
		return
	}

	log.MemberID = apiKey.MemberID
	if apiKey.Member == nil {
		return
	}
	log.MemberCodeSnapshot = optionalTrimmedStringPtr(apiKey.Member.MemberCode)
	log.MemberNameSnapshot = optionalTrimmedStringPtr(apiKey.Member.Name)
}

// usageGroupID returns the immutable request-level group selected for an
// enterprise member. Ordinary keys retain their configured fixed group.
func usageGroupID(ctx context.Context, apiKey *APIKey) *int64 {
	if apiKey == nil {
		return nil
	}
	if apiKey.MemberID != nil {
		if active, ok := ActiveGroupFromContext(ctx); ok && active.MemberID == *apiKey.MemberID && active.GroupID > 0 {
			groupID := active.GroupID
			return &groupID
		}
	}
	return apiKey.GroupID
}

type UsageScheduleMeta struct {
	Provider            string  `json:"provider,omitempty"`
	Layer               string  `json:"layer,omitempty"`
	StickyPreviousHit   bool    `json:"sticky_previous_hit,omitempty"`
	StickySessionHit    bool    `json:"sticky_session_hit,omitempty"`
	CandidateCount      int     `json:"candidate_count,omitempty"`
	TopK                int     `json:"top_k,omitempty"`
	LatencyMs           int64   `json:"latency_ms,omitempty"`
	LoadSkew            float64 `json:"load_skew,omitempty"`
	SelectedAccountID   int64   `json:"selected_account_id,omitempty"`
	SelectedAccountType string  `json:"selected_account_type,omitempty"`
}

func UsageScheduleMetaFromOpenAIDecision(decision OpenAIAccountScheduleDecision) *UsageScheduleMeta {
	if decision.Layer == "" &&
		!decision.StickyPreviousHit &&
		!decision.StickySessionHit &&
		decision.CandidateCount == 0 &&
		decision.TopK == 0 &&
		decision.LatencyMs == 0 &&
		decision.LoadSkew == 0 &&
		decision.SelectedAccountID == 0 &&
		decision.SelectedAccountType == "" {
		return nil
	}

	loadSkew := decision.LoadSkew
	if math.IsNaN(loadSkew) || math.IsInf(loadSkew, 0) {
		loadSkew = 0
	}

	return &UsageScheduleMeta{
		Provider:            "openai",
		Layer:               strings.TrimSpace(decision.Layer),
		StickyPreviousHit:   decision.StickyPreviousHit,
		StickySessionHit:    decision.StickySessionHit,
		CandidateCount:      decision.CandidateCount,
		TopK:                decision.TopK,
		LatencyMs:           decision.LatencyMs,
		LoadSkew:            loadSkew,
		SelectedAccountID:   decision.SelectedAccountID,
		SelectedAccountType: strings.TrimSpace(decision.SelectedAccountType),
	}
}

func (u *UsageLog) TotalTokens() int {
	return u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens
}

func (u *UsageLog) EffectiveRequestType() RequestType {
	if u == nil {
		return RequestTypeUnknown
	}
	if normalized := u.RequestType.Normalize(); normalized != RequestTypeUnknown {
		return normalized
	}
	return RequestTypeFromLegacy(u.Stream, u.OpenAIWSMode)
}

func (u *UsageLog) SyncRequestTypeAndLegacyFields() {
	if u == nil {
		return
	}
	requestType := u.EffectiveRequestType()
	u.RequestType = requestType
	u.Stream, u.OpenAIWSMode = ApplyLegacyRequestFields(requestType, u.Stream, u.OpenAIWSMode)
}
