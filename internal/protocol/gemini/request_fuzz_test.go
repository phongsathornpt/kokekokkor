package gemini

import "testing"

func FuzzDecodeGenerateContentRequest(f *testing.F) {
	f.Add([]byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`))
	f.Add([]byte(`{"contents":[{"role":"model","parts":[{"functionCall":{"id":"call_1","name":"lookup","args":{"id":1}}}]}]}`))
	f.Add([]byte(`{"contents":`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeGenerateContentRequest(data)
	})
}
