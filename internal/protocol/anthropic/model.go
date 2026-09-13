package anthropic

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
)

func requestModel(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	contentType := r.Header.Get("Content-Type")
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || (mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json")) {
			return ""
		}
	}

	original := r.Body
	var captured bytes.Buffer
	decoder := json.NewDecoder(io.TeeReader(original, &captured))
	model, _ := topLevelModel(decoder)
	r.Body = &replayReadCloser{
		Reader: io.MultiReader(bytes.NewReader(captured.Bytes()), original),
		Closer: original,
	}
	return model
}

func topLevelModel(decoder *json.Decoder) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", err
	}
	opening, ok := token.(json.Delim)
	if !ok || opening != '{' {
		return "", nil
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return "", err
		}
		key, ok := keyToken.(string)
		if !ok {
			return "", nil
		}
		if key == "model" {
			var model string
			if err := decoder.Decode(&model); err != nil {
				return "", err
			}
			return model, nil
		}
		if err := skipJSONValue(decoder); err != nil {
			return "", err
		}
	}
	return "", nil
}

func skipJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		for decoder.More() {
			if _, err := decoder.Token(); err != nil {
				return err
			}
			if err := skipJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := skipJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return nil
	}
}

type replayReadCloser struct {
	io.Reader
	io.Closer
}
