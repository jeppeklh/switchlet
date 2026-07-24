PACKAGE := ./cmd/switchlet
BINARY := switchlet
VERSION ?= dev
DIST_DIR := dist/$(VERSION)
RELEASE_PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: build install verify clean release-binaries

build:
	go build -o $(BINARY) $(PACKAGE)

install:
	go install $(PACKAGE)

verify:
	gofmt -w .
	go test ./...
	go vet ./...

clean:
	rm -rf dist $(BINARY) $(BINARY).exe

release-binaries:
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)
	@for platform in $(RELEASE_PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output="$(DIST_DIR)/$(BINARY)_$${os}_$${arch}"; \
		if [ "$$os" = "windows" ]; then output="$$output.exe"; fi; \
		echo "Building $$output"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o "$$output" $(PACKAGE) || exit 1; \
	done
