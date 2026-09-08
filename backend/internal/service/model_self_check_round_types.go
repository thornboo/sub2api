package service

import (
	"context"
	"errors"
	"time"
)

var ErrModelSelfCheckRoundInProgress = errors.New("model self check probe round in progress")

// ModelSelfCheckRoundRepository persists actual execution evidence. Implementations
// must never reconstruct a round by joining unrelated account histories.
type ModelSelfCheckRoundRepository interface {
	CreateProbeRound(context.Context, *ModelSelfCheckProbeRound) error
	UpdateProbeRound(context.Context, *ModelSelfCheckProbeRound) error
	ListLatestProbeRounds(context.Context, []ModelSelfCheckTarget) ([]ModelSelfCheckProbeRound, error)
	DeleteProbeRoundsBefore(context.Context, time.Time) (int64, error)
}

// These DTOs contain account identity and are exclusively for the admin API.
type ModelSelfCheckProbeStep struct {
	AccountID         int64      `json:"account_id"`
	AccountName       string     `json:"account_name"`
	Priority          int        `json:"priority"`
	Platform          string     `json:"platform"`
	Order             int        `json:"order"`
	Outcome           string     `json:"outcome"` // pending, skipped, succeeded, failed, not_attempted, incomplete
	ReasonCode        string     `json:"reason_code"`
	StartedAt         *time.Time `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	LatencyMs         *int       `json:"latency_ms"`
	HTTPStatus        *int       `json:"http_status"`
	ErrorCode         string     `json:"error_code"`
	RetryCount        int        `json:"retry_count,omitempty"`
	InitialHTTPStatus *int       `json:"initial_http_status,omitempty"`
}

type ModelSelfCheckProbeRound struct {
	ID              int64                     `json:"id"`
	GroupID         int64                     `json:"group_id"`
	Model           string                    `json:"model"`
	Status          string                    `json:"status"` // checking, operational, degraded, failed, unknown
	ReasonCode      string                    `json:"reason_code"`
	WinnerAccountID *int64                    `json:"winner_account_id"`
	StartedAt       time.Time                 `json:"started_at"`
	FinishedAt      *time.Time                `json:"finished_at"`
	DurationMs      *int                      `json:"duration_ms"`
	Steps           []ModelSelfCheckProbeStep `json:"steps"`
}

type ModelSelfCheckChainCandidate struct {
	AccountID     int64      `json:"account_id"`
	AccountName   string     `json:"account_name"`
	Priority      int        `json:"priority"`
	Platform      string     `json:"platform"`
	Order         int        `json:"order"` // eligible order, zero if excluded
	Eligible      bool       `json:"eligible"`
	ReasonCode    string     `json:"reason_code"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
	LastStatus    string     `json:"last_status"`
}

type ModelSelfCheckChainView struct {
	GroupID               int64                          `json:"group_id"`
	GroupName             string                         `json:"group_name"`
	Model                 string                         `json:"model"`
	UpdatedAt             time.Time                      `json:"updated_at"`
	AttemptTimeoutSeconds int                            `json:"attempt_timeout_seconds"`
	RoundTimeoutSeconds   int                            `json:"round_timeout_seconds"`
	Candidates            []ModelSelfCheckChainCandidate `json:"candidates"`
	LatestRound           *ModelSelfCheckProbeRound      `json:"latest_round"`
}
