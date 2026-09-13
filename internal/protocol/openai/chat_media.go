package openai

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/phongsathornpt/kokekokkor/internal/domain/llm"
)

func decodeImageURL(value string) llm.MediaSource {
	if strings.HasPrefix(value, "data:") {
		if mediaType, data, ok := parseDataURL(value); ok {
			return llm.MediaSource{Type: llm.MediaSourceBase64, MediaType: mediaType, Data: data}
		}
	}
	return llm.MediaSource{Type: llm.MediaSourceURL, URL: value}
}

func encodeImageURL(source llm.MediaSource) (string, error) {
	switch source.Type {
	case llm.MediaSourceURL:
		return source.URL, nil
	case llm.MediaSourceBase64:
		if _, err := base64.StdEncoding.DecodeString(source.Data); err != nil {
			return "", fmt.Errorf("invalid base64 image data: %w", err)
		}
		return "data:" + source.MediaType + ";base64," + source.Data, nil
	default:
		return "", fmt.Errorf("chat codec does not support media source %q", source.Type)
	}
}

func parseDataURL(value string) (string, string, bool) {
	header, data, ok := strings.Cut(value, ",")
	if !ok || !strings.HasSuffix(header, ";base64") {
		return "", "", false
	}
	mediaType := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
	if mediaType == "" || data == "" {
		return "", "", false
	}
	return mediaType, data, true
}
