package service

import "fmt"

// AccountCapabilityMismatchError indicates that the selected account cannot
// preserve a valid request's protocol semantics. Handlers should exclude this
// account and continue scheduling without marking the account unhealthy.
type AccountCapabilityMismatchError struct {
	AccountID int64
	Feature   string
	Message   string
}

func (e *AccountCapabilityMismatchError) Error() string {
	if e == nil {
		return "account capability mismatch"
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("account %d does not support %s", e.AccountID, e.Feature)
}
