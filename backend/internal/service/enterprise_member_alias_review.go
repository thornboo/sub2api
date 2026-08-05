package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	EnterpriseMemberAliasReviewStatusPending          = "pending"
	EnterpriseMemberAliasReviewStatusRegistered       = "registered"
	EnterpriseMemberAliasReviewStatusRejectedInvalid  = "rejected_invalid"
	EnterpriseMemberAliasReviewStatusObsolete         = "obsolete"
	EnterpriseMemberAliasReviewStatusNeedsOwnerAction = "needs_owner_action"

	EnterpriseMemberAliasReviewMaxModelLen = 255
	EnterpriseMemberAliasReviewMaxNoteLen  = 1000
)

type EnterpriseMemberAliasReviewRepository interface {
	ListLegacySuccessNewPruned(ctx context.Context, input EnterpriseMemberAliasReviewListInput) ([]EnterpriseMemberAliasReviewItem, error)
	UpsertReview(ctx context.Context, input EnterpriseMemberAliasReviewUpsert) (*EnterpriseMemberAliasReviewRecord, error)
	EnterpriseMemberAliasReviewReadinessRepository
}

type EnterpriseMemberAliasReviewReadinessRepository interface {
	GetReadinessSummary(ctx context.Context, now time.Time) (*EnterpriseMemberAliasReviewReadinessSummary, error)
}

type EnterpriseMemberAliasReviewListInput struct {
	Now   time.Time
	Limit int
}

type EnterpriseMemberAliasReviewItem struct {
	PublicModel                string     `json:"public_model"`
	PublicModelNorm            string     `json:"public_model_norm"`
	Endpoint                   string     `json:"endpoint"`
	LegacyOutcome              string     `json:"legacy_outcome"`
	PlannedOutcome             string     `json:"planned_outcome"`
	ReasonCodes                []string   `json:"reason_codes"`
	FinalGroupID               *int64     `json:"final_group_id,omitempty"`
	ChannelID                  *int64     `json:"channel_id,omitempty"`
	RequestCount7d             int64      `json:"request_count_7d"`
	RequestCount30d            int64      `json:"request_count_30d"`
	SuccessCount7d             int64      `json:"success_count_7d"`
	SuccessCount30d            int64      `json:"success_count_30d"`
	AffectedEnterpriseCount7d  int64      `json:"affected_enterprise_count_7d"`
	AffectedEnterpriseCount30d int64      `json:"affected_enterprise_count_30d"`
	LastSeenAt                 time.Time  `json:"last_seen_at"`
	ReviewStatus               string     `json:"review_status"`
	ReviewedBy                 *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt                 *time.Time `json:"reviewed_at,omitempty"`
	ReviewNote                 string     `json:"review_note,omitempty"`
	StableRouteEvidence        string     `json:"stable_route_evidence"`
}

type EnterpriseMemberAliasReviewRecord struct {
	ID                 int64           `json:"id"`
	PublicModel        string          `json:"public_model"`
	PublicModelNorm    string          `json:"public_model_norm"`
	Endpoint           string          `json:"endpoint"`
	Status             string          `json:"status"`
	FinalGroupID       *int64          `json:"final_group_id,omitempty"`
	ChannelID          *int64          `json:"channel_id,omitempty"`
	ReviewNote         string          `json:"review_note,omitempty"`
	ValidationEvidence json.RawMessage `json:"validation_evidence,omitempty"`
	ReviewedBy         *int64          `json:"reviewed_by,omitempty"`
	ReviewedAt         *time.Time      `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type EnterpriseMemberAliasReviewUpsert struct {
	PublicModel        string
	PublicModelNorm    string
	Endpoint           string
	Status             string
	FinalGroupID       *int64
	ChannelID          *int64
	ReviewNote         string
	ValidationEvidence json.RawMessage
	ReviewedBy         int64
}

type EnterpriseMemberAliasReviewRequest struct {
	PublicModel  string `json:"public_model"`
	Endpoint     string `json:"endpoint"`
	Status       string `json:"status"`
	FinalGroupID *int64 `json:"final_group_id,omitempty"`
	ChannelID    *int64 `json:"channel_id,omitempty"`
	ReviewNote   string `json:"review_note,omitempty"`
}

type EnterpriseMemberAliasReviewReadinessSummary struct {
	Ready                           bool      `json:"ready"`
	BlockingUnreviewedActive7d      int64     `json:"blocking_unreviewed_active_7d"`
	BlockingUnreviewedActive30d     int64     `json:"blocking_unreviewed_active_30d"`
	LegacySuccessNewPrunedActive7d  int64     `json:"legacy_success_new_pruned_active_7d"`
	LegacySuccessNewPrunedActive30d int64     `json:"legacy_success_new_pruned_active_30d"`
	GeneratedAt                     time.Time `json:"generated_at"`
	Reason                          string    `json:"reason"`
}

type EnterpriseMemberAliasReviewService struct {
	repo        EnterpriseMemberAliasReviewReadinessRepository
	reviewRepo  EnterpriseMemberAliasReviewRepository
	publication EnterpriseMemberRoutePublicationResolver
	delivery    EnterpriseMemberRouteDeliveryResolver
	groupRepo   GroupRepository
}

func NewEnterpriseMemberAliasReviewService(
	repo EnterpriseMemberAliasReviewRepository,
	publication EnterpriseMemberRoutePublicationResolver,
	delivery EnterpriseMemberRouteDeliveryResolver,
	groupRepo GroupRepository,
) *EnterpriseMemberAliasReviewService {
	return &EnterpriseMemberAliasReviewService{
		repo: repo, reviewRepo: repo, publication: publication, delivery: delivery, groupRepo: groupRepo,
	}
}

func ProvideEnterpriseMemberAliasReviewService(
	repo EnterpriseMemberAliasReviewRepository,
	channel *ChannelService,
	delivery *ModelDeliveryService,
	groupRepo GroupRepository,
) *EnterpriseMemberAliasReviewService {
	return NewEnterpriseMemberAliasReviewService(repo, NewEnterpriseMemberChannelPublicationResolver(channel), delivery, groupRepo)
}

func (s *EnterpriseMemberAliasReviewService) ListLegacySuccessNewPruned(ctx context.Context, limit int) ([]EnterpriseMemberAliasReviewItem, error) {
	if s == nil || s.reviewRepo == nil {
		return nil, errors.New("enterprise member alias review service is not configured")
	}
	return s.reviewRepo.ListLegacySuccessNewPruned(ctx, EnterpriseMemberAliasReviewListInput{Now: time.Now().UTC(), Limit: limit})
}

func (s *EnterpriseMemberAliasReviewService) GetReadinessSummary(ctx context.Context) (*EnterpriseMemberAliasReviewReadinessSummary, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("enterprise member alias review service is not configured")
	}
	summary, err := s.repo.GetReadinessSummary(ctx, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = &EnterpriseMemberAliasReviewReadinessSummary{}
	}
	summary.Ready = summary.BlockingUnreviewedActive7d == 0 && summary.BlockingUnreviewedActive30d == 0
	if summary.Ready {
		summary.Reason = "alias_review_gate_satisfied"
	} else {
		summary.Reason = "legacy_success_new_pruned_requires_review"
	}
	return summary, nil
}

func (s *EnterpriseMemberAliasReviewService) Review(ctx context.Context, req EnterpriseMemberAliasReviewRequest, actorUserID int64) (*EnterpriseMemberAliasReviewRecord, error) {
	if s == nil || s.reviewRepo == nil {
		return nil, errors.New("enterprise member alias review service is not configured")
	}
	model, norm, err := NormalizeEnterpriseMemberAliasReviewModel(req.PublicModel)
	if err != nil {
		return nil, err
	}
	status := normalizeEnterpriseMemberAliasReviewStatus(req.Status)
	if status == "" {
		return nil, fmt.Errorf("invalid alias review status")
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	if len(endpoint) > 128 || hasControlRune(endpoint) {
		return nil, fmt.Errorf("invalid endpoint")
	}
	note := strings.TrimSpace(req.ReviewNote)
	if len([]rune(note)) > EnterpriseMemberAliasReviewMaxNoteLen {
		return nil, fmt.Errorf("review note is too long")
	}
	if hasControlRune(note) {
		return nil, fmt.Errorf("review note contains control characters")
	}
	evidence := json.RawMessage(`{}`)
	if status == EnterpriseMemberAliasReviewStatusRegistered {
		evidence, err = s.validateRegistered(ctx, model, endpoint, req.FinalGroupID, req.ChannelID)
		if err != nil {
			return nil, err
		}
	}
	return s.reviewRepo.UpsertReview(ctx, EnterpriseMemberAliasReviewUpsert{
		PublicModel: model, PublicModelNorm: norm, Endpoint: endpoint, Status: status,
		FinalGroupID: req.FinalGroupID, ChannelID: req.ChannelID, ReviewNote: note,
		ValidationEvidence: evidence, ReviewedBy: actorUserID,
	})
}

func (s *EnterpriseMemberAliasReviewService) validateRegistered(ctx context.Context, model, endpoint string, groupID, channelID *int64) (json.RawMessage, error) {
	if groupID == nil || *groupID <= 0 {
		return nil, fmt.Errorf("registered alias requires final_group_id")
	}
	if s.publication == nil || s.delivery == nil || s.groupRepo == nil {
		return nil, fmt.Errorf("registered alias validation dependencies are unavailable")
	}
	group, err := s.groupRepo.GetByID(ctx, *groupID)
	if err != nil {
		return nil, fmt.Errorf("load registered alias group: %w", err)
	}
	if !IsGroupContextValid(group) || !group.IsActive() {
		return nil, fmt.Errorf("registered alias group is not active")
	}
	publications, err := s.publication.ResolveEnterpriseMemberRoutePublications(ctx, []int64{*groupID}, model)
	if err != nil {
		return nil, fmt.Errorf("validate registered alias publication: %w", err)
	}
	pub, ok := publications[*groupID]
	if !ok {
		return nil, fmt.Errorf("registered alias requires exact mapping or exact pricing")
	}
	if pub.Source != EnterpriseMemberRoutePublicationExactMapping && pub.Source != EnterpriseMemberRoutePublicationExactPricing {
		return nil, fmt.Errorf("registered alias cannot rely on wildcard mapping expansion")
	}
	if channelID != nil && *channelID > 0 && pub.ChannelID != *channelID {
		return nil, fmt.Errorf("registered alias channel_id does not match current publication")
	}
	protocol := normalizeEnterpriseMemberRouteProtocol("", endpoint)
	if protocol == "" {
		return nil, fmt.Errorf("registered alias endpoint is not supported by stable planning")
	}
	projection, err := s.delivery.ResolveForEnterpriseMemberRoute(ctx, []*Group{group}, model, protocol)
	if err != nil {
		return nil, fmt.Errorf("validate registered alias delivery: %w", err)
	}
	groupProjection := projection.Group(model, *groupID)
	if enterpriseMemberRouteReasonForProjection(groupProjection, protocol) != EnterpriseMemberRouteReasonEligible {
		return nil, fmt.Errorf("registered alias has no stable delivery projection")
	}
	payload := map[string]any{
		"publication_source":   pub.Source,
		"channel_id":           pub.ChannelID,
		"channel_mapped_model": pub.ChannelMappedModel,
		"inbound_protocol":     protocol,
		"route_count":          len(groupProjection.Routes),
		"validated_at":         time.Now().UTC().Format(time.RFC3339Nano),
	}
	return json.Marshal(payload)
}

func NormalizeEnterpriseMemberAliasReviewModel(raw string) (string, string, error) {
	model := strings.TrimSpace(raw)
	if model == "" {
		return "", "", fmt.Errorf("public_model is required")
	}
	if len([]rune(model)) > EnterpriseMemberAliasReviewMaxModelLen {
		return "", "", fmt.Errorf("public_model is too long")
	}
	if hasControlRune(model) {
		return "", "", fmt.Errorf("public_model contains control characters")
	}
	return model, strings.ToLower(model), nil
}

func normalizeEnterpriseMemberAliasReviewStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", EnterpriseMemberAliasReviewStatusPending:
		return EnterpriseMemberAliasReviewStatusPending
	case EnterpriseMemberAliasReviewStatusRegistered:
		return EnterpriseMemberAliasReviewStatusRegistered
	case EnterpriseMemberAliasReviewStatusRejectedInvalid:
		return EnterpriseMemberAliasReviewStatusRejectedInvalid
	case EnterpriseMemberAliasReviewStatusObsolete:
		return EnterpriseMemberAliasReviewStatusObsolete
	case EnterpriseMemberAliasReviewStatusNeedsOwnerAction:
		return EnterpriseMemberAliasReviewStatusNeedsOwnerAction
	default:
		return ""
	}
}

func hasControlRune(value string) bool {
	for _, r := range value {
		// Format characters include bidi overrides/isolates and zero-width
		// controls. They are not needed in model identifiers, endpoints, or
		// operator notes and can make audit/export text visually misleading.
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return true
		}
	}
	return false
}
