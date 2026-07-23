package middleware

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OrchestrateEnterpriseMemberGroups retries a terminal, group-local routing
// failure on the next ordered member group while the response is still wholly
// uncommitted. Flush, hijack, an explicit WriteHeaderNow, or the first success
// byte permanently locks the active group for the request.
func OrchestrateEnterpriseMemberGroups(next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		plan, ok := enterpriseMemberGroupPlanFromContext(c)
		if !ok || len(plan.candidates) < 2 || c.Request == nil {
			next(c)
			return
		}

		body, err := readReplayableRequestBody(c.Request)
		if err != nil {
			next(c)
			return
		}
		baseContext := c.Request.Context()
		baseKeys := cloneGinKeys(c.Keys)
		baseErrors := len(c.Errors)
		originalWriter := c.Writer
		requestedModel, _ := baseContext.Value(ctxkey.Model).(string)

		for {
			restoreRequestBody(c.Request, body)
			tx := newEnterpriseMemberTransactionalWriter(originalWriter)
			c.Writer = tx
			next(c)

			_, retryable := service.OpsGroupRetryReasonFromContext(c)
			if tx.committed || !retryable || service.IsEnterpriseMemberBudgetOutcomeAmbiguous(c) || plan.current+1 >= len(plan.candidates) {
				tx.commitBuffered()
				c.Writer = originalWriter
				return
			}

			// The failed attempt never escaped to the client. Restore request-local
			// handler state, retain only the middleware baseline, and activate the
			// next immutable group snapshot.
			upstreamErrors, hasUpstreamErrors := c.Get(service.OpsUpstreamErrorsKey)
			c.Writer = originalWriter
			c.Keys = cloneGinKeys(baseKeys)
			if hasUpstreamErrors {
				c.Set(service.OpsUpstreamErrorsKey, upstreamErrors)
			}
			if len(c.Errors) > baseErrors {
				c.Errors = c.Errors[:baseErrors]
			}
			c.Request = c.Request.WithContext(baseContext)
			activateEnterpriseMemberGroupCandidate(c, plan, plan.current+1, requestedModel)
		}
	}
}

func enterpriseMemberGroupPlanFromContext(c *gin.Context) (*enterpriseMemberGroupPlan, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(enterpriseMemberGroupPlanKey)
	if !ok {
		return nil, false
	}
	plan, ok := value.(*enterpriseMemberGroupPlan)
	return plan, ok && plan != nil && plan.current >= 0 && plan.current < len(plan.candidates)
}

func readReplayableRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodDelete {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	restoreRequestBody(r, body)
	return body, nil
}

func restoreRequestBody(r *http.Request, body []byte) {
	if r == nil || body == nil {
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

func cloneGinKeys(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

type enterpriseMemberTransactionalWriter struct {
	original  gin.ResponseWriter
	header    http.Header
	status    int
	size      int
	written   bool
	committed bool
	body      bytes.Buffer
}

func newEnterpriseMemberTransactionalWriter(original gin.ResponseWriter) *enterpriseMemberTransactionalWriter {
	return &enterpriseMemberTransactionalWriter{
		original: original,
		header:   cloneHTTPHeader(original.Header()),
		status:   http.StatusOK,
		size:     -1,
	}
}

func (w *enterpriseMemberTransactionalWriter) Header() http.Header { return w.header }

func (w *enterpriseMemberTransactionalWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.status = code
}

func (w *enterpriseMemberTransactionalWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.written = true
		w.size = 0
	}
	if w.committed || w.status < http.StatusBadRequest {
		w.commitHeaders()
		n, err := w.original.Write(data)
		w.size += n
		return n, err
	}
	n, err := w.body.Write(data)
	w.size += n
	return n, err
}

func (w *enterpriseMemberTransactionalWriter) WriteString(value string) (int, error) {
	return w.Write([]byte(value))
}

func (w *enterpriseMemberTransactionalWriter) Status() int { return w.status }
func (w *enterpriseMemberTransactionalWriter) Size() int   { return w.size }
func (w *enterpriseMemberTransactionalWriter) Written() bool {
	return w.written
}

func (w *enterpriseMemberTransactionalWriter) WriteHeaderNow() {
	if !w.written {
		w.written = true
		w.size = 0
	}
	w.commitHeaders()
}

func (w *enterpriseMemberTransactionalWriter) Flush() {
	w.WriteHeaderNow()
	w.original.Flush()
}

func (w *enterpriseMemberTransactionalWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.WriteHeaderNow()
	return w.original.Hijack()
}

func (w *enterpriseMemberTransactionalWriter) CloseNotify() <-chan bool {
	return w.original.CloseNotify()
}

func (w *enterpriseMemberTransactionalWriter) Pusher() http.Pusher {
	return w.original.Pusher()
}

func (w *enterpriseMemberTransactionalWriter) commitHeaders() {
	if w.committed {
		return
	}
	replaceHTTPHeader(w.original.Header(), w.header)
	w.original.WriteHeader(w.status)
	w.committed = true
}

func (w *enterpriseMemberTransactionalWriter) commitBuffered() {
	if w.committed {
		return
	}
	w.commitHeaders()
	if w.body.Len() == 0 {
		return
	}
	_, _ = w.original.Write(w.body.Bytes())
}

func cloneHTTPHeader(src http.Header) http.Header {
	dst := make(http.Header, len(src))
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
	return dst
}

func replaceHTTPHeader(dst, src http.Header) {
	for key := range dst {
		delete(dst, key)
	}
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
}
