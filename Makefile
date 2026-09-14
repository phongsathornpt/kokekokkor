APP := kokekokkor
FUZZ_TIME ?= 10s

.PHONY: run test race fuzz vet build

run:
	go run ./cmd/kokekokkor

test:
	go test ./...

race:
	go test -race ./...

fuzz:
	go test ./internal/protocol/anthropic -run='^$$' -fuzz='^FuzzDecodeMessagesRequest$$' -fuzztime=$(FUZZ_TIME)
	go test ./internal/protocol/gemini -run='^$$' -fuzz='^FuzzDecodeGenerateContentRequest$$' -fuzztime=$(FUZZ_TIME)
	go test ./internal/protocol/openai -run='^$$' -fuzz='^FuzzDecodeChatRequest$$' -fuzztime=$(FUZZ_TIME)
	go test ./internal/protocol/openai -run='^$$' -fuzz='^FuzzDecodeResponsesRequest$$' -fuzztime=$(FUZZ_TIME)

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -trimpath -o bin/$(APP) ./cmd/kokekokkor
