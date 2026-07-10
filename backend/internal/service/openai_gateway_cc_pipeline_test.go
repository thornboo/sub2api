package service

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanCCStreamStopsOnConversionError(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"first"}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"second"}}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(stream))}
	sentinel := errors.New("conversion limit reached")
	emitted := 0

	state := (&OpenAIGatewayService{}).scanCCStream(
		resp,
		"test chat fallback",
		"req_test",
		time.Now(),
		func(*apicompat.ChatCompletionsChunk) error {
			emitted++
			return sentinel
		},
	)

	require.ErrorIs(t, state.Err, sentinel)
	assert.Equal(t, 1, emitted, "conversion error must stop reading further chunks")
	assert.False(t, state.SawDone)
}
