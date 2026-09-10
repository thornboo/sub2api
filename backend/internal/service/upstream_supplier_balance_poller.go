package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	upstreamSupplierBalancePollInterval   = time.Minute
	upstreamSupplierBalancePollDelay      = 5 * time.Minute
	upstreamSupplierBalancePollBatchSize  = 4
	upstreamSupplierBalancePollConcurrent = 4
	upstreamSupplierBalancePollMaxPerRun  = 64
	upstreamSupplierBalancePollRunBudget  = 45 * time.Second
	upstreamSupplierBalanceRetention      = 35 * 24 * time.Hour
)

type UpstreamSupplierBalancePoller struct {
	db upstreamCostPoolSQLExecutor

	pollInterval time.Duration
	nextDelay    time.Duration
	batchSize    int
	concurrency  int
	retention    time.Duration
	refresh      func(context.Context, int64) error

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	once   sync.Once
}

func NewUpstreamSupplierBalancePoller(db *dbent.Client, cfg *config.Config, encryptor SecretEncryptor) *UpstreamSupplierBalancePoller {
	return NewUpstreamSupplierBalancePollerWithOptions(
		db,
		cfg,
		encryptor,
		upstreamSupplierBalancePollInterval,
		upstreamSupplierBalancePollDelay,
		upstreamSupplierBalancePollBatchSize,
		upstreamSupplierBalancePollConcurrent,
		upstreamSupplierBalanceRetention,
	)
}

func NewUpstreamSupplierBalancePollerWithOptions(
	db upstreamCostPoolSQLExecutor,
	cfg *config.Config,
	encryptor SecretEncryptor,
	pollInterval time.Duration,
	nextDelay time.Duration,
	batchSize int,
	concurrency int,
	retention time.Duration,
) *UpstreamSupplierBalancePoller {
	if pollInterval <= 0 {
		pollInterval = upstreamSupplierBalancePollInterval
	}
	if nextDelay <= 0 {
		nextDelay = upstreamSupplierBalancePollDelay
	}
	if batchSize <= 0 || batchSize > upstreamSupplierBalancePollBatchSize {
		batchSize = upstreamSupplierBalancePollBatchSize
	}
	if concurrency <= 0 || concurrency > upstreamSupplierBalancePollConcurrent {
		concurrency = upstreamSupplierBalancePollConcurrent
	}
	if retention <= 0 {
		retention = upstreamSupplierBalanceRetention
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &UpstreamSupplierBalancePoller{
		db:           db,
		pollInterval: pollInterval,
		nextDelay:    nextDelay,
		batchSize:    batchSize,
		concurrency:  concurrency,
		retention:    retention,
		ctx:          ctx,
		cancel:       cancel,
		refresh: func(ctx context.Context, supplierID int64) error {
			_, err := refreshUpstreamSupplierBalance(ctx, db, cfg, encryptor, supplierID)
			return err
		},
	}
}

func (p *UpstreamSupplierBalancePoller) Start() {
	if p == nil || p.db == nil {
		return
	}
	p.once.Do(func() {
		p.wg.Add(1)
		go p.loop()
	})
}

func (p *UpstreamSupplierBalancePoller) Stop() {
	if p == nil {
		return
	}
	p.cancel()
	p.wg.Wait()
}

func (p *UpstreamSupplierBalancePoller) loop() {
	defer p.wg.Done()
	p.runOnce(p.ctx)
	cleanupTicker := time.NewTicker(time.Hour)
	defer cleanupTicker.Stop()
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.runOnce(p.ctx)
		case <-cleanupTicker.C:
			p.pruneOldSamples(p.ctx, 1000)
		}
	}
}

func (p *UpstreamSupplierBalancePoller) runOnce(ctx context.Context) {
	deadline := time.Now().Add(upstreamSupplierBalancePollRunBudget)
	processed := 0
	for ctx.Err() == nil && processed < upstreamSupplierBalancePollMaxPerRun && time.Now().Before(deadline) {
		ids, err := p.claimDueSuppliers(ctx)
		if err != nil {
			if ctx.Err() == nil {
				slog.WarnContext(ctx, "Supplier balance poll claim failed", "error", err)
			}
			return
		}
		if len(ids) == 0 {
			return
		}
		p.refreshClaimedSuppliers(ctx, ids)
		processed += len(ids)
		if len(ids) < p.batchSize {
			return
		}
	}
}

func (p *UpstreamSupplierBalancePoller) refreshClaimedSuppliers(ctx context.Context, ids []int64) {
	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	for _, supplierID := range ids {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := p.refresh(ctx, id); err != nil && ctx.Err() == nil {
				slog.WarnContext(ctx, "Supplier balance poll refresh failed", "supplier_id", id, "error", err)
			}
		}(supplierID)
	}
	wg.Wait()
}

func (p *UpstreamSupplierBalancePoller) claimDueSuppliers(ctx context.Context) ([]int64, error) {
	rows, err := p.db.QueryContext(ctx, `
WITH due AS (
    SELECT id
    FROM upstream_suppliers
    WHERE status = 'active'
      AND is_system = FALSE
      AND (balance_config->>'enabled')::boolean IS TRUE
      AND balance_access_token <> ''
      AND balance_next_poll_at <= NOW()
    ORDER BY balance_next_poll_at ASC, id ASC
    FOR UPDATE SKIP LOCKED
    LIMIT $1
)
UPDATE upstream_suppliers s
SET balance_next_poll_at = NOW() + $2::interval
FROM due
WHERE s.id = due.id
RETURNING s.id`, p.batchSize, durationIntervalLiteral(p.nextDelay))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := make([]int64, 0, p.batchSize)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (p *UpstreamSupplierBalancePoller) pruneOldSamples(ctx context.Context, limit int) {
	if p == nil || p.db == nil || limit <= 0 {
		return
	}
	_, err := p.db.ExecContext(ctx, `
DELETE FROM upstream_supplier_balance_samples
WHERE id IN (
    SELECT id
    FROM upstream_supplier_balance_samples
    WHERE sampled_at < NOW() - $1::interval
    ORDER BY sampled_at ASC, id ASC
    LIMIT $2
)`, durationIntervalLiteral(p.retention), limit)
	if err != nil && ctx.Err() == nil {
		slog.WarnContext(ctx, "Supplier balance sample cleanup failed", "error", err)
	}
}

func durationIntervalLiteral(d time.Duration) string {
	if d < time.Second {
		d = time.Second
	}
	return fmt.Sprintf("%d seconds", int64(d.Truncate(time.Second)/time.Second))
}
