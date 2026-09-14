package openai

import "testing"

func FuzzDecodeChatRequest(f *testing.F) {
	f.Add([]byte(`{"model":"gpt","messages":[{"role":"user","content":"hello"}]}`))
	f.Add([]byte(`{"model":"gpt","messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"id\":1}"}}]}]}`))
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeChatRequest(data)
	})
}

func FuzzDecodeResponsesRequest(f *testing.F) {
	f.Add([]byte(`{"model":"gpt","input":"hello"}`))
	f.Add([]byte(`{"model":"gpt","input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]}`))
	f.Add([]byte(`[`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeResponsesRequest(data)
	})
}
