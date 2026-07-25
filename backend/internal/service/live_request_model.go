package service

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/tidwall/gjson"
)

// ExtractLiveCallRequestModel reads session.model from the normalized Live
// create payload without consuming the caller's body.
func ExtractLiveCallRequestModel(contentType string, body []byte) (string, error) {
	session, err := extractLiveCallRequestSession(contentType, body)
	if err != nil || len(session) == 0 {
		return "", err
	}
	return strings.TrimSpace(gjson.GetBytes(session, "model").String()), nil
}

func extractLiveCallRequestSession(contentType string, body []byte) ([]byte, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil && strings.TrimSpace(contentType) != "" {
		return nil, err
	}
	if strings.EqualFold(mediaType, "multipart/form-data") {
		boundary := strings.TrimSpace(params["boundary"])
		if boundary == "" {
			return nil, errors.New("multipart boundary is required")
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		for {
			part, nextErr := reader.NextPart()
			if errors.Is(nextErr, io.EOF) {
				return nil, nil
			}
			if nextErr != nil {
				return nil, nextErr
			}
			if strings.EqualFold(strings.TrimSpace(part.FormName()), "session") && part.FileName() == "" {
				data, readErr := io.ReadAll(io.LimitReader(part, 1<<20))
				_ = part.Close()
				return data, readErr
			}
			_ = part.Close()
		}
	}
	if session := gjson.GetBytes(body, "session"); session.Exists() {
		return []byte(session.Raw), nil
	}
	return body, nil
}
