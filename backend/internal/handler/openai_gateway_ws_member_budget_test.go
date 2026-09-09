package handler

import (
	"errors"
	"fmt"
	"testing"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBudgetWSCloseReasonPreservesCodeAndCompleteMessage(t *testing.T) {
	for _, limitErr := range []*infraerrors.ApplicationError{
		service.ErrEnterpriseMemberBudgetExceeded,
		service.ErrEnterpriseMemberRateLimit5hExceeded,
		service.ErrEnterpriseMemberRateLimit1dExceeded,
		service.ErrEnterpriseMemberRateLimit7dExceeded,
		service.ErrEnterpriseMemberAsyncBudgetUnavailable,
	} {
		t.Run(limitErr.Reason, func(t *testing.T) {
			for _, err := range []error{limitErr, fmt.Errorf("reserve member budget: %w", limitErr)} {
				reason := enterpriseMemberBudgetWSCloseReason(err)
				require.Equal(t, limitErr.Reason+": "+limitErr.Message, reason)
				require.LessOrEqual(t, len(reason), 120, "Chinese messages must fit before close-frame truncation")
				require.True(t, utf8.ValidString(reason))
				require.NotContains(t, reason, "metadata=")
			}
		})
	}
}

func TestEnterpriseMemberBudgetWSCloseReasonKeepsOtherErrorsUnchanged(t *testing.T) {
	for _, err := range []error{
		service.ErrEnterpriseMemberBudgetUnbounded,
		service.ErrEnterpriseMemberBudgetConflict,
		errors.New("budget service unavailable"),
	} {
		require.Equal(t, err.Error(), enterpriseMemberBudgetWSCloseReason(err))
	}
}
