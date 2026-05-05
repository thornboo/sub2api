package service

import (
	"context"
	"testing"
	"time"
)

type channelMonitorMaintenanceRepoStub struct {
	ChannelMonitorRepository
	watermark          *time.Time
	deleteHistoryCalls int
	deleteRollupCalls  int
}

func (s *channelMonitorMaintenanceRepoStub) LoadAggregationWatermark(ctx context.Context) (*time.Time, error) {
	return s.watermark, nil
}

func (s *channelMonitorMaintenanceRepoStub) DeleteHistoryBefore(ctx context.Context, before time.Time) (int64, error) {
	s.deleteHistoryCalls++
	return 0, nil
}

func (s *channelMonitorMaintenanceRepoStub) DeleteRollupsBefore(ctx context.Context, beforeDate time.Time) (int64, error) {
	s.deleteRollupCalls++
	return 0, nil
}

func TestChannelMonitorRunDailyMaintenanceAutoCleanupDisabledSkipsDeletes(t *testing.T) {
	watermark := time.Now().UTC().Add(24 * time.Hour).Truncate(24 * time.Hour)
	repo := &channelMonitorMaintenanceRepoStub{watermark: &watermark}
	svc := &ChannelMonitorService{repo: repo}

	if err := svc.RunDailyMaintenance(context.Background(), false); err != nil {
		t.Fatalf("RunDailyMaintenance() error = %v", err)
	}
	if repo.deleteHistoryCalls != 0 {
		t.Fatalf("deleteHistoryCalls = %d, want 0", repo.deleteHistoryCalls)
	}
	if repo.deleteRollupCalls != 0 {
		t.Fatalf("deleteRollupCalls = %d, want 0", repo.deleteRollupCalls)
	}
}

func TestChannelMonitorRunDailyMaintenanceAutoCleanupEnabledDeletesOldData(t *testing.T) {
	watermark := time.Now().UTC().Add(24 * time.Hour).Truncate(24 * time.Hour)
	repo := &channelMonitorMaintenanceRepoStub{watermark: &watermark}
	svc := &ChannelMonitorService{repo: repo}

	if err := svc.RunDailyMaintenance(context.Background(), true); err != nil {
		t.Fatalf("RunDailyMaintenance() error = %v", err)
	}
	if repo.deleteHistoryCalls != 1 {
		t.Fatalf("deleteHistoryCalls = %d, want 1", repo.deleteHistoryCalls)
	}
	if repo.deleteRollupCalls != 1 {
		t.Fatalf("deleteRollupCalls = %d, want 1", repo.deleteRollupCalls)
	}
}
