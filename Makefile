BIN := sk

.PHONY: build install test run
build:
	go build -o bin/$(BIN) ./cmd/sk

install:
	go install ./cmd/sk

test:
	go vet ./... && go test ./...

run: build
	./bin/$(BIN)
