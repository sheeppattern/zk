VERSION ?= dev
LDFLAGS = -X github.com/sheeppattern/zk/cmd.Version=$(VERSION)
BINARY = zk

.PHONY: build test lint clean release-local install

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

install: build
	@mkdir -p $(HOME)/bin
	cp $(BINARY)$(if $(filter windows,$(shell go env GOOS)),.exe,) $(HOME)/bin/
	@echo "Installed to $(HOME)/bin/$(BINARY)"

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/

release-local:
	@mkdir -p dist
	@for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
		IFS='/' read -r GOOS GOARCH <<< "$$platform"; \
		output="dist/zk_$${GOOS}_$${GOARCH}"; \
		if [ "$$GOOS" = "windows" ]; then output="$${output}.exe"; fi; \
		echo "Building $${output}..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags="$(LDFLAGS)" -o "$${output}" .; \
	done
	@echo "Done. Binaries in dist/"
