package service

// OpenAIClientRequestError marks a request-local validation failure that did
// not reach any upstream. Handlers must not feed it into account health scores.
type OpenAIClientRequestError struct {
	Message string
	Cause   error
}

func (e *OpenAIClientRequestError) Error() string {
	if e == nil {
		return "invalid OpenAI client request"
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	if e.Message != "" {
		return e.Message
	}
	return "invalid OpenAI client request"
}

func (e *OpenAIClientRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newOpenAIClientRequestError(message string, cause error) error {
	return &OpenAIClientRequestError{Message: message, Cause: cause}
}
