BIN := speckit

.PHONY: build install test run
build:
	go build -o bin/$(BIN) .

install:
	go build -o $(shell go env GOPATH)/bin/$(BIN) .

test:
	go vet ./... && go test ./...

run: build
	./bin/$(BIN)
