package handler

import (
	"context"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newUsageRecordTestPool(t *testing.T) *service.UsageRecordWorkerPool {
	t.Helper()
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             8,
		TaskTimeout:           time.Second,
		OverflowPolicy:        "drop",
		OverflowSamplePercent: 0,
		AutoScaleEnabled:      false,
	})
	t.Cleanup(pool.Stop)
	return pool
}

func TestGatewayHandlerSubmitUsageRecordTask_WithPool(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	h := &GatewayHandler{usageRecordWorkerPool: pool}

	done := make(chan struct{})
	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task not executed")
	}
}

func TestGatewayHandlerSubmitUsageRecordTask_WithoutPoolSyncFallback(t *testing.T) {
	h := &GatewayHandler{}
	var called atomic.Bool

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected deadline in fallback context")
		}
		called.Store(true)
	})

	require.True(t, called.Load())
}

func TestGatewayHandlerSubmitUsageRecordTask_NilTask(t *testing.T) {
	h := &GatewayHandler{}
	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), nil)
	})
}

func TestGatewayHandlerSubmitUsageRecordTask_WithoutPool_TaskPanicRecovered(t *testing.T) {
	h := &GatewayHandler{}
	var called atomic.Bool

	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
			panic("usage task panic")
		})
	})

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		called.Store(true)
	})
	require.True(t, called.Load(), "panic 后后续任务应仍可执行")
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithPool(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}

	done := make(chan struct{})
	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task not executed")
	}
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithoutPoolSyncFallback(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	var called atomic.Bool

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected deadline in fallback context")
		}
		called.Store(true)
	})

	require.True(t, called.Load())
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_NilTask(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), nil)
	})
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithoutPool_TaskPanicRecovered(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	var called atomic.Bool

	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
			panic("usage task panic")
		})
	})

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		called.Store(true)
	})
	require.True(t, called.Load(), "panic 后后续任务应仍可执行")
}

func TestOpenAIGatewayHandlerSubmitMandatoryUsageRecordTask_DroppedTaskSyncFallback(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             1,
		TaskTimeout:           time.Second,
		OverflowPolicy:        "drop",
		OverflowSamplePercent: 0,
		AutoScaleEnabled:      false,
	})
	t.Cleanup(pool.Stop)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}

	block := make(chan struct{})
	release := make(chan struct{})
	pool.Submit(func(ctx context.Context) {
		close(block)
		<-release
	})
	<-block
	pool.Submit(func(ctx context.Context) {})

	var called atomic.Bool
	h.submitMandatoryUsageRecordTask(context.Background(), func(ctx context.Context) {
		called.Store(true)
	})
	close(release)

	require.True(t, called.Load(), "mandatory usage task must run synchronously when async submit is dropped")
}

func TestOpenAIGatewayHandlerSubmitOpenAIUsageRecordTask_ImageResultUsesMandatoryFallback(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             1,
		TaskTimeout:           time.Second,
		OverflowPolicy:        "drop",
		OverflowSamplePercent: 0,
		AutoScaleEnabled:      false,
	})
	t.Cleanup(pool.Stop)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}

	block := make(chan struct{})
	release := make(chan struct{})
	pool.Submit(func(ctx context.Context) {
		close(block)
		<-release
	})
	<-block
	pool.Submit(func(ctx context.Context) {})

	var called atomic.Bool
	h.submitOpenAIUsageRecordTask(nil, context.Background(), &service.OpenAIForwardResult{ImageCount: 1}, nil, func(ctx context.Context) {
		called.Store(true)
	})
	close(release)

	require.True(t, called.Load(), "image usage task must be mandatory when async submit is dropped")
}

func TestGatewayHandlerSubmitMemberUsageRecordTaskRunsSynchronously(t *testing.T) {
	h := &GatewayHandler{usageRecordWorkerPool: newUsageRecordTestPool(t)}
	memberID := int64(44)
	var called atomic.Bool

	h.submitGatewayUsageRecordTask(nil, context.Background(), &service.APIKey{MemberID: &memberID}, func(context.Context) {
		called.Store(true)
	})

	require.True(t, called.Load())
}

func TestOpenAIGatewayHandlerSubmitMemberUsageRecordTaskRunsSynchronously(t *testing.T) {
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: newUsageRecordTestPool(t)}
	memberID := int64(44)
	var called atomic.Bool

	h.submitOpenAIUsageRecordTask(nil, context.Background(), &service.OpenAIForwardResult{}, &service.APIKey{MemberID: &memberID}, func(context.Context) {
		called.Store(true)
	})

	require.True(t, called.Load())
}

func TestMemberUsageRecordTaskPanicMarksBudgetOutcomeAmbiguous(t *testing.T) {
	memberID := int64(44)
	apiKey := &service.APIKey{MemberID: &memberID}

	t.Run("openai", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		h := &OpenAIGatewayHandler{}
		h.submitOpenAIUsageRecordTask(c, context.Background(), &service.OpenAIForwardResult{}, apiKey, func(context.Context) {
			panic("usage task panic")
		})
		require.True(t, service.IsEnterpriseMemberBudgetOutcomeAmbiguous(c))
		require.Equal(t, "usage_persistence_failed", service.EnterpriseMemberBudgetOutcomeAmbiguousReason(c))
	})

	t.Run("generic_gateway", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		h := &GatewayHandler{}
		h.submitGatewayUsageRecordTask(c, context.Background(), apiKey, func(context.Context) {
			panic("usage task panic")
		})
		require.True(t, service.IsEnterpriseMemberBudgetOutcomeAmbiguous(c))
		require.Equal(t, "usage_persistence_failed", service.EnterpriseMemberBudgetOutcomeAmbiguousReason(c))
	})
}

func TestUsageRecordContextPreservesActiveEnterpriseMemberGroup(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ActiveGroup, &service.ActiveGroupContext{
		MemberID: 44,
		GroupID:  9,
	})

	ctx := usageRecordContext(parent, context.Background())
	active, ok := service.ActiveGroupFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, int64(44), active.MemberID)
	require.Equal(t, int64(9), active.GroupID)

	original, _ := service.ActiveGroupFromContext(parent)
	active.GroupID = 10
	require.Equal(t, int64(9), original.GroupID, "usage context must not retain a mutable routing pointer")
}
