PACKAGE := ./cmd/switchlet
BINARY := switchlet
VERSION ?= dev
GO_LDFLAGS := -X main.buildVersion=$(VERSION)
DIST_DIR := dist/$(VERSION)
CHECKSUM_FILE := $(DIST_DIR)/checksums.txt
RELEASE_PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: build install verify clean release-binaries release-checksums release-check

build:
	go build -ldflags "$(GO_LDFLAGS)" -o $(BINARY) $(PACKAGE)

install:
	go install -ldflags "$(GO_LDFLAGS)" $(PACKAGE)

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
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "$(GO_LDFLAGS)" -o "$$output" $(PACKAGE) || exit 1; \
	done

release-checksums: release-binaries
	rm -f $(CHECKSUM_FILE)
	cd $(DIST_DIR) && sha256sum * > checksums.txt

release-check: verify release-checksums
