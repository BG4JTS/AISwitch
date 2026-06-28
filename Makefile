# AI Switch — Build & Test Targets
# ============================================================

.PHONY: build build-all test vet clean run help

# ── Single-binary builds ─────────────────────────────────────

build:
	go build -ldflags="-s -w" -o bin/ais .

build-core:
	go build -ldflags="-s -w" -o bin/ais-core .

build-webui:
	go build -ldflags="-s -w" -tags webui -o bin/ais-webui .

build-cost:
	go build -ldflags="-s -w" -tags cost -o bin/ais-cost .

build-keymgr:
	go build -ldflags="-s -w" -tags keymgr -o bin/ais-keymgr .

build-full:
	go build -ldflags="-s -w" -tags "cost,keymgr,webui" -o bin/ais-full .

build-all: build build-webui build-cost build-keymgr build-full

# ── Test & Quality ────────────────────────────────────────────

test:
	go test ./internal/convert/ -v -count=1
	go test ./internal/logger/ -v -count=1
	go test ./internal/config/ -v -count=1

test-cover:
	go test ./internal/... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

vet:
	go vet ./...

# ── Utilities ─────────────────────────────────────────────────

run:
	go run . serve

clean:
	rm -rf bin/
	rm -f coverage.out

# ── Cross-compile (linux/windows/darwin, amd64/arm64) ─────────

cross:
	@for GOOS in linux windows darwin; do \
		for GOARCH in amd64 arm64; do \
			name="bin/ais-$${GOOS}-$${GOARCH}"; \
			[ "$$GOOS" = "windows" ] && name="$${name}.exe"; \
			echo "→ $$name"; \
			GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags="-s -w" -o "$$name" .; \
		done; \
	done
	@echo "Checksums:"; cd bin && sha256sum * > checksums.txt

# ── Help ──────────────────────────────────────────────────────

help:
	@echo "Targets:"
	@echo "  build          — default binary"
	@echo "  build-core     — core only (no modules)"
	@echo "  build-webui    — core + webui"
	@echo "  build-cost     — core + cost control"
	@echo "  build-keymgr   — core + key manager"
	@echo "  build-full     — all modules"
	@echo "  build-all      — every variant"
	@echo "  cross          — cross-compile 6 platforms"
	@echo "  test           — unit tests (15)"
	@echo "  test-cover     — coverage report"
	@echo "  vet            — static analysis"
	@echo "  clean          — remove bin/ and coverage.out"
