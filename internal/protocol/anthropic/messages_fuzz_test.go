package anthropic

import "testing"

func FuzzDecodeMessagesRequest(f *testing.F) {
	f.Add([]byte(`{"model":"claude","max_tokens":32,"messages":[{"role":"user","content":"hello"}]}`))
	f.Add([]byte(`{"model":"claude","max_tokens":32,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"done"}]}]}`))
	f.Add([]byte(`{`))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeMessagesRequest(data)
	})
}
