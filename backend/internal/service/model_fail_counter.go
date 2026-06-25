package service

import "context"

// ModelFailCounterCache 追踪某账号下单个模型 (scope) 的连续失败次数，
// 用于在「连续 N 次失败才限流」的滑动窗口策略下决定何时真正打限流标记。
type ModelFailCounterCache interface {
	// IncrementModelFailCount 原子递增 (account, scope) 的失败计数并返回当前值。
	IncrementModelFailCount(ctx context.Context, accountID int64, scope string, windowMinutes int) (int64, error)
	// ResetModelFailCount 在手动解除该模型限流后清零计数器。
	ResetModelFailCount(ctx context.Context, accountID int64, scope string) error
}
