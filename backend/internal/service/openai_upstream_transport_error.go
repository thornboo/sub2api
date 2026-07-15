package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// openAITransportErrorTempUnschedDuration is how long an account is temporarily
// unscheduled after a durable transport failure (matches tokenRefreshTempUnschedDuration).
const openAITransportErrorTempUnschedDuration = 10 * time.Minute

// openAITransportFailoverBody is the OpenAI-format error body attached to the
// failover error for a transport-level failure. Kept identical to the legacy
// inline 502 body so the client-visible payload is unchanged if failover is
// ultimately exhausted.
var openAITransportFailoverBody = []byte(`{"error":{"type":"upstream_error","message":"Upstream request failed"}}`)

// openAITransportErrorClass describes how to react to a transport-level upstream
// failure — i.e. the HTTP round-trip never completed (proxy / DNS / TCP / TLS
// error, no HTTP status code received).
type openAITransportErrorClass struct {
	// Persistent marks failures where retrying the same proxy/account is
	// pointless: expired or rejected proxy credentials, a dead proxy endpoint,
	// or DNS/routing failure. Such accounts should be temporarily unscheduled
	// (and alerted on) instead of being repeatedly scheduled into hard failures.
	Persistent bool
}

// openAIPersistentTransportErrorMarkers are substrings (matched case-insensitively
// against the raw transport error) that indicate a durable proxy/network fault.
// Matched signals are intentionally specific failure *reasons*, not the operation
// (e.g. we match "connection refused", not "proxyconnect") so that a transient
// failure of the same operation (a proxy timeout) is NOT misclassified as durable.
var openAIPersistentTransportErrorMarkers = []string{
	"authentication failed",         // SOCKS5 RFC1929 / proxy credentials rejected (expired account)
	"proxy authentication required", // HTTP proxy 407
	"connection refused",            // proxy/upstream endpoint down
	"no route to host",
	"network is unreachable",
	"no such host", // DNS resolution failure (bad/expired proxy hostname)
}

// classifyOpenAITransportError decides whether a transport-level upstream error
// is durable (Persistent — evict the account + alert) or a transient blip
// (fail over to a healthy account but keep this one schedulable).
//
// Motivating incident: a SOCKS5 proxy whose subscription lapsed returned
// `username/password authentication failed`; the account was nonetheless
// rescheduled on every request, hard-failing users with 502s.
//
// Classification strategy (mirrors sanitizeStreamError in gateway_service.go):
//  1. Typed-error checks first (syscall constants, *net.DNSError) — portable and
//     unambiguous.
//  2. String-marker fallback for errors that have no typed form (e.g. the plain
//     string returned by golang.org/x/net/proxy for SOCKS5 credential rejection).
//     The network-layer string markers ("connection refused", "no route to host",
//     "network is unreachable", "no such host") are kept as a cross-platform safety
//     net even though the typed checks should cover them on modern Go+Linux.
func classifyOpenAITransportError(err error) openAITransportErrorClass {
	if err == nil {
		return openAITransportErrorClass{}
	}

	// — Typed checks (preferred) ——————————————————————————————————————————————
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH) {
		return openAITransportErrorClass{Persistent: true}
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return openAITransportErrorClass{Persistent: true}
	}

	// — String-marker fallback ————————————————————————————————————————————————
	msg := strings.ToLower(err.Error())
	for _, marker := range openAIPersistentTransportErrorMarkers {
		if strings.Contains(msg, marker) {
			return openAITransportErrorClass{Persistent: true}
		}
	}
	return openAITransportErrorClass{}
}

// markEnterpriseMemberBudgetTransportOutcome preserves the receipt whenever a
// transport failure cannot prove that dispatch stopped before upstream
// execution. It is shared by OpenAI and protocol-conversion gateway paths so
// enterprise-member accounting has one fail-closed boundary.
func markEnterpriseMemberBudgetTransportOutcome(c *gin.Context, err error) {
	if c == nil || err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		MarkEnterpriseMemberBudgetOutcomeAmbiguousWithReason(c, "client_disconnected_after_upstream_dispatch")
		return
	}
	if classifyOpenAITransportError(err).Persistent {
		// DNS, routing, refused connections and proxy-auth failures are proven
		// pre-dispatch failures, so the reservation may follow normal release.
		return
	}
	MarkEnterpriseMemberBudgetOutcomeAmbiguousWithReason(c, "upstream_transport_outcome_unknown")
}

// handleUpstreamTransportFailure records the accounting outcome and reports
// whether replay is proven safe. Only failures that conclusively happened
// before upstream dispatch may be retried; timeouts, resets, cancellations and
// all unclassified transport failures are fail-closed because the upstream may
// already have executed the request.
func handleUpstreamTransportFailure(c *gin.Context, err error) bool {
	markEnterpriseMemberBudgetTransportOutcome(c, err)
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return classifyOpenAITransportError(err).Persistent
}

// handleOpenAIUpstreamTransportError handles a transport-level upstream failure
// (Do/DoWithTLS returned a non-HTTP error: proxy/DNS/TCP/TLS). It:
//  1. records the failure in Ops error logs (status 0, kind=request_error);
//  2. for durable faults (expired/rejected proxy creds, dead proxy, DNS/routing)
//     temporarily unschedules the account (DB + in-memory) and logs a stable
//     warn event that alert rules can key on;
//  3. retries only failures proven to occur before the upstream could execute
//     the request. Other transport failures preserve the member budget receipt
//     as ambiguous and stop replay, because the request may already have been
//     accepted upstream even though no HTTP response reached us.
//
// It deliberately does NOT write to the response: the handler owns the response
// (failover, or a protocol-correct error once failover is exhausted).
//
// passthrough tags the Ops error event for the OpenAI passthrough forward path.
func (s *OpenAIGatewayService) handleOpenAIUpstreamTransportError(ctx context.Context, c *gin.Context, account *Account, err error, passthrough bool) error {
	safeErr := sanitizeUpstreamErrorMessage(err.Error())
	setOpsUpstreamError(c, 0, safeErr, "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: 0,
		Passthrough:        passthrough,
		Kind:               "request_error",
		Message:            safeErr,
	})

	class := classifyOpenAITransportError(err)

	// A client disconnect does not prove that the upstream request was canceled
	// before execution. Do not replay it and preserve any member reservation for
	// reconciliation.
	if errors.Is(err, context.Canceled) {
		markEnterpriseMemberBudgetTransportOutcome(c, err)
		return err
	}

	if class.Persistent {
		s.tempUnscheduleOpenAITransportError(ctx, account, safeErr)
		return &UpstreamFailoverError{
			StatusCode:   http.StatusBadGateway,
			ResponseBody: openAITransportFailoverBody,
		}
	}

	markEnterpriseMemberBudgetTransportOutcome(c, err)
	return &UpstreamFailoverError{
		StatusCode:        http.StatusBadGateway,
		ResponseBody:      openAITransportFailoverBody,
		Stage:             GatewayFailureStageInference,
		Scope:             GatewayFailureScopeRequest,
		NextAccountAction: NextAccountStop,
	}
}

// tempUnscheduleOpenAITransportError marks an account temporarily unschedulable
// after a durable transport failure, both persistently (DB, survives restart)
// and in-memory (immediate scheduler effect before the DB/account cache propagates).
//
// Log semantics:
//   - "openai.account_temp_unscheduled_transport" — emitted ONLY after a
//     successful DB write (both in-memory + persisted).
//   - "openai.account_temp_unscheduled_transport_memory_only" — emitted when
//     accountRepo is nil (in-memory only; no persistence).
//   - "openai.account_temp_unscheduled_transport_failed" — DB write attempted
//     but returned an error.
func (s *OpenAIGatewayService) tempUnscheduleOpenAITransportError(ctx context.Context, account *Account, safeErr string) {
	if s == nil || account == nil {
		return
	}
	until := time.Now().Add(openAITransportErrorTempUnschedDuration)
	reason := "upstream transport error (proxy/network): " + safeErr

	// Immediate in-memory block (honoured by the scheduler at selection time),
	// effective even if the DB write below fails or the account cache lags.
	s.BlockAccountScheduling(account, until, "transport_error")

	if s.accountRepo == nil {
		// No DB configured — block is in-memory only; emit a distinct event so
		// operators are not misled into thinking the block survived a restart.
		logger.L().With(zap.String("component", "service.openai_gateway")).Warn(
			"openai.account_temp_unscheduled_transport_memory_only",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.String("platform", account.Platform),
			zap.Time("until", until),
			zap.String("reason", reason),
		)
		return
	}

	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAccountStateUpdateTimeout)
	defer cancel()
	if err := s.accountRepo.SetTempUnschedulable(bgCtx, account.ID, until, reason); err != nil {
		logger.L().With(zap.String("component", "service.openai_gateway")).Warn(
			"openai.account_temp_unscheduled_transport_failed",
			zap.Int64("account_id", account.ID),
			zap.Error(err),
		)
		return
	}

	// DB write succeeded: both in-memory and persisted.
	logger.L().With(zap.String("component", "service.openai_gateway")).Warn(
		"openai.account_temp_unscheduled_transport",
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.String("platform", account.Platform),
		zap.Time("until", until),
		zap.String("reason", reason),
	)
}
