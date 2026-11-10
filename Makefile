BINARY  := dstat
DIST    := dist
GO      ?= go
LDFLAGS := -s -w
PLATFORMS := linux/amd64 linux/arm64 linux/386 linux/arm linux/ppc64le linux/s390x
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all build build-all package clean vet

all: build

## build: native linux/amd64 binary
build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

## build-all: one binary per release platform under dist/
build-all:
	@mkdir -p $(DIST)
	@set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out="$(DIST)/$(BINARY)-$(VERSION)-$$os-$$arch"; \
		mkdir -p "$$out"; \
		if [ "$$arch" = "arm" ]; then export GOARM=7; else unset GOARM; fi; \
		echo "building $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "$$out/$(BINARY)" .; \
	done

## package: build-all + tar.gz archives (binary, READMEs, LICENSE, docs)
package: build-all
	@set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out="$(DIST)/$(BINARY)-$(VERSION)-$$os-$$arch"; \
		cp README.md README.zh-CN.md LICENSE "$$out/"; \
		cp -r docs "$$out/"; \
		tar -C $(DIST) -czf "$$out.tar.gz" "$$(basename $$out)"; \
		echo "packaged $$out.tar.gz"; \
	done

vet:
	$(GO) vet ./...

clean:
	rm -rf $(DIST) $(BINARY)
