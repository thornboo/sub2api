package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayHandlerImages_DisabledGroupRejectsBeforeScheduling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-image-2","prompt":"draw","size":"1024x1024"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	groupID := int64(111)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      222,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			AllowImageGeneration: false,
		},
		User: &service.User{ID: 333},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 333, Concurrency: 1})

	h := &OpenAIGatewayHandler{
		gatewayService:      &service.OpenAIGatewayService{},
		billingCacheService: &service.BillingCacheService{},
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   &ConcurrencyHelper{concurrencyService: &service.ConcurrencyService{}},
	}

	h.Images(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "permission_error", gjson.GetBytes(rec.Body.Bytes(), "error.type").String())
	require.Contains(t, rec.Body.String(), service.ImageGenerationPermissionMessage())
	_, retryMarked := service.GroupAttemptResultFromContext(c)
	require.False(t, retryMarked, "ordinary keys must keep existing non-enterprise retry semantics")
}

func TestOpenAIGatewayHandlerImages_DisabledEnterpriseGroupMarksSafeLocalCapabilityMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-image-2","prompt":"draw","size":"1024x1024"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	groupID := int64(111)
	memberID := int64(444)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ActiveGroup, &service.ActiveGroupContext{
		GroupID:       groupID,
		AttemptNumber: 1,
	}))
	c.Request = req
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:       222,
		GroupID:  &groupID,
		MemberID: &memberID,
		Member:   &service.EnterpriseMember{ID: memberID, Status: service.EnterpriseMemberStatusActive},
		Group: &service.Group{
			ID:                   groupID,
			AllowImageGeneration: false,
		},
		User: &service.User{ID: 333},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 333, Concurrency: 1})

	h := &OpenAIGatewayHandler{
		gatewayService:      &service.OpenAIGatewayService{},
		billingCacheService: &service.BillingCacheService{},
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   &ConcurrencyHelper{concurrencyService: &service.ConcurrencyService{}},
	}

	h.Images(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	attempt, retryMarked := service.GroupAttemptResultFromContext(c)
	require.True(t, retryMarked, "a local capability rejection must not terminate a multi-group member before another authorized group is considered")
	require.Equal(t, service.OpsGroupRetryReasonCapabilityMismatch, attempt.Reason)
}

func TestMarkEnterpriseMemberGroupRetryPreservesSafeRecoveryInEveryAdmissionMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(111)
	memberID := int64(444)
	apiKey := &service.APIKey{
		ID: 222, GroupID: &groupID, MemberID: &memberID,
		Member: &service.EnterpriseMember{ID: memberID},
	}

	for _, test := range []struct {
		name        string
		mode        service.EnterpriseMemberModelAdmissionMode
		planApplied bool
	}{
		{name: "legacy plan", mode: service.EnterpriseMemberModelAdmissionLegacyOrderOnly},
		{name: "shadow plan", mode: service.EnterpriseMemberModelAdmissionShadowPublished},
		{name: "enforce without applied plan", mode: service.EnterpriseMemberModelAdmissionEnforcePublished},
		{name: "applied enforce plan", mode: service.EnterpriseMemberModelAdmissionEnforcePublished, planApplied: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			req = req.WithContext(context.WithValue(req.Context(), ctxkey.ActiveGroup, &service.ActiveGroupContext{
				GroupID: groupID, AttemptNumber: 1,
				RoutePlanMode: test.mode, ModelPlanApplied: test.planApplied,
			}))
			c.Request = req
			c.Set(string(middleware2.ContextKeyAPIKey), apiKey)

			markEnterpriseMemberGroupRetry(c, apiKey, service.OpsGroupRetryReasonCapabilityMismatch)

			attempt, marked := service.GroupAttemptResultFromContext(c)
			require.True(t, marked)
			require.Equal(t, service.OpsGroupRetryReasonCapabilityMismatch, attempt.Reason)
		})
	}
}
