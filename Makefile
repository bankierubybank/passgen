VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64 freebsd/amd64

.PHONY: build test dist release-dist archives manpage sbom checksums clean

# dist and release-dist list their steps in order (clean first, checksums
# last), so never run targets in parallel, even under make -j.
.NOTPARALLEL:

## build: binary for this machine -> ./passgen
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o passgen .

## test: vet + tests with race detector
test:
	go vet ./...
	go test -race ./...

## dist: archives for every platform + checksums -> dist/
dist: clean archives checksums

## release-dist: dist plus an SBOM (downloads the pinned cyclonedx-gomod)
release-dist: clean archives sbom checksums

# The man page with this version and the commit date filled in, made by a
# throwaway build for this machine.
manpage:
	@mkdir -p dist/.man
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/.man/passgen .
	dist/.man/passgen man > dist/.man/passgen.1

archives: manpage
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		name=passgen_$(VERSION)_$${os}_$${arch}; \
		bin=passgen; [ $$os = windows ] && bin=passgen.exe; \
		echo "building $$name"; \
		mkdir -p dist/$$name; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$$name/$$bin . || exit 1; \
		cp README.md LICENSE dist/.man/passgen.1 dist/$$name/; \
		if [ $$os = windows ]; then (cd dist && zip -qr $$name.zip $$name); \
		else tar -C dist -czf dist/$$name.tar.gz $$name; fi; \
		rm -rf dist/$$name; \
	done
	@rm -rf dist/.man

## sbom: CycloneDX SBOM (modules + Go standard library) -> dist/
# One SBOM covers every platform: passgen is pure Go with no dependencies,
# so the module set is the same everywhere.
sbom:
	@mkdir -p dist
	CGO_ENABLED=0 go run github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.12.0 \
		app -json -std -licenses -output dist/passgen_$(VERSION)_sbom.cdx.json .

checksums:
	@cd dist && files=$$(ls *.tar.gz *.zip *.cdx.json 2>/dev/null || true) && \
		if [ -z "$$files" ]; then echo "checksums: nothing in dist/ (run make dist)" >&2; exit 1; fi && \
		(command -v sha256sum >/dev/null && sha256sum $$files || shasum -a 256 $$files) > checksums.txt
	@ls -lh dist

clean:
	rm -rf dist passgen passgen.exe
