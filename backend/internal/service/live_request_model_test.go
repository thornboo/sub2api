package service

import (
	"bytes"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractLiveCallRequestModelFromJSONAndMultipart(t *testing.T) {
	model, err := ExtractLiveCallRequestModel(
		"application/json",
		[]byte(`{"sdp":"v=0\r\n","session":{"model":" live-public ","instructions":"test"}}`),
	)
	require.NoError(t, err)
	require.Equal(t, "live-public", model)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("sdp", "v=0\r\n"))
	require.NoError(t, writer.WriteField("session", `{"model":"multipart-live"}`))
	require.NoError(t, writer.Close())

	model, err = ExtractLiveCallRequestModel(writer.FormDataContentType(), body.Bytes())
	require.NoError(t, err)
	require.Equal(t, "multipart-live", model)
}
