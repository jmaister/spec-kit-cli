BIN    := sk
PREFIX ?= $(HOME)/.local/bin

.PHONY: build install uninstall test run
build:
	go build -o bin/$(BIN) ./cmd/sk

# Installs to ~/.local/bin by default (override: make install PREFIX=/usr/local/bin).
install: build
	@mkdir -p $(PREFIX)
	install -m 0755 bin/$(BIN) $(PREFIX)/$(BIN)
	@echo "installed $(PREFIX)/$(BIN)"
	@case ":$$PATH:" in *":$(PREFIX):"*) ;; *) echo "warning: $(PREFIX) is not in your PATH — add: export PATH=\"$(PREFIX):\$$PATH\"";; esac

uninstall:
	rm -f $(PREFIX)/$(BIN)

test:
	go vet ./... && go test ./...

run: build
	./bin/$(BIN)
