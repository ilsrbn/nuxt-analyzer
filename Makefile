.PHONY: build-parser build clean test validate-release

BINARY := bin/impact-map$(if $(filter Windows_NT,$(OS)),.exe,)

build-parser:
	cd node && npm install && npm run build
	mkdir -p assets
	cp node/dist/parser.bundle.js assets/parser.bundle.js

build: build-parser
	go build -o $(BINARY) ./cmd/impact-map

test:
	go test ./...

validate-release:
	bash -n install.sh
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser check; \
	else \
		echo "goreleaser not installed; skipping goreleaser check"; \
	fi

clean:
	rm -rf bin/ node/dist/ node/node_modules/ assets/parser.bundle.js
