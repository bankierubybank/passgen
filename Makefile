VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64 freebsd/amd64

.PHONY: build test dist clean

## build: binary for this machine -> ./passgen
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o passgen .

## test: vet + tests with race detector
test:
	go vet ./...
	go test -race ./...

## dist: archives for every platform + checksums -> dist/
dist: clean
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		name=passgen_$(VERSION)_$${os}_$${arch}; \
		bin=passgen; [ $$os = windows ] && bin=passgen.exe; \
		echo "building $$name"; \
		mkdir -p dist/$$name; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$name/$$bin . || exit 1; \
		cp README.md LICENSE dist/$$name/; \
		if [ $$os = windows ]; then (cd dist && zip -qr $$name.zip $$name); \
		else tar -C dist -czf dist/$$name.tar.gz $$name; fi; \
		rm -rf dist/$$name; \
	done
	@cd dist && (command -v sha256sum >/dev/null && sha256sum *.tar.gz *.zip || shasum -a 256 *.tar.gz *.zip) > checksums.txt
	@ls -lh dist

clean:
	rm -rf dist passgen passgen.exe
