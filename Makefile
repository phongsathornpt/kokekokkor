APP := kokekokkor

.PHONY: run test race vet build

run:
	go run ./cmd/kokekokkor

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -trimpath -o bin/$(APP) ./cmd/kokekokkor
