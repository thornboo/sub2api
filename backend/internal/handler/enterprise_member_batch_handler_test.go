package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBatchEndpointsRequireIdempotencyKeyBeforeCoordinatorFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("PATCH", "/api/v1/enterprise/members/batch", strings.NewReader(`{}`))

	key, ok := requireEnterpriseMemberBatchIdempotencyKey(ctx)

	require.False(t, ok)
	require.Empty(t, key)
	require.Equal(t, 400, recorder.Code)
	require.Contains(t, recorder.Body.String(), "IDEMPOTENCY_KEY_REQUIRED")
}

func TestEnterpriseMemberBatchMutationSummaryStaysWithinIdempotencyResponseLimit(t *testing.T) {
	payload, err := json.Marshal(enterpriseMemberBatchMutationSummary{UpdatedCount: 500})

	require.NoError(t, err)
	require.JSONEq(t, `{"updated_count":500}`, string(payload))
	require.Less(t, len(payload), 1024)
}
