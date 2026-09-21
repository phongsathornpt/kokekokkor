APP := kokekokkor
FUZZ_TIME ?= 10s
TEMPL ?= $(shell which templ 2>/dev/null || echo $(HOME)/go/bin/templ)
AIR ?= $(shell which air 2>/dev/null || echo $(HOME)/go/bin/air)

.PHONY: run dev test race fuzz modcheck vet build css templ web

css:
	bunx @tailwindcss/cli -i ./web/static/css/input.css -o ./web/static/css/styles.css --minify

templ:
	$(TEMPL) generate ./web/...

web: css templ

dev: web
	@which $(AIR) > /dev/null 2>&1 || (echo "Air is not installed. Install with: go install github.com/air-verse/air@latest" && exit 1)
	$(AIR)

run:
	go run ./cmd/kokekokkor

test: web
	go test ./...

race: web
	go test -race ./...

fuzz:
	go test ./internal/protocol/anthropic -run='^$$' -fuzz='^FuzzDecodeMessagesRequest$$' -fuzztime=$(FUZZ_TIME)
	go test ./internal/protocol/gemini -run='^$$' -fuzz='^FuzzDecodeGenerateContentRequest$$' -fuzztime=$(FUZZ_TIME)
	go test ./internal/protocol/openai -run='^$$' -fuzz='^FuzzDecodeChatRequest$$' -fuzztime=$(FUZZ_TIME)
	go test ./internal/protocol/openai -run='^$$' -fuzz='^FuzzDecodeResponsesRequest$$' -fuzztime=$(FUZZ_TIME)
	go test ./internal/protocol/sse -run='^$$' -fuzz='^FuzzDecode$$' -fuzztime=$(FUZZ_TIME)

modcheck:
	go mod tidy -diff
	go mod verify

vet: web
	go vet ./...

build: web
	mkdir -p bin
	go build -trimpath -o bin/$(APP) ./cmd/kokekokkor
