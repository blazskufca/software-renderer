.PHONY: run build build-pgo profile web serve clean profile-mem view-mem view-cpu render-mem render-cpu test test-wasm cover lint bench bench-wasm bench-compare

GOROOT := $(shell go env GOROOT)

PGO_PROFILE := profiling

# The PR #1 renderer (internal/pipeline + internal/raster) is opt-in:
# `make <target> SIMD=1` builds with `-tags simd` and GOEXPERIMENT=simd (Go 1.27+).
# Without it you get the original renderer. `go build -tags simd` alone gives the
# new renderer with scalar raster kernels.
ifdef SIMD
GO_ENV := GOEXPERIMENT=simd
GO_TAGS := -tags simd
endif

# Run the native (desktop) build.
run:
	$(GO_ENV) go run $(GO_TAGS) ./cmd/renderer

# Build the native binary.
build:
	$(GO_ENV) go build $(GO_TAGS) -o renderer ./cmd/renderer

profile:
	$(GO_ENV) go run $(GO_TAGS) ./cmd/renderer -cpuprofile "$(PGO_PROFILE)/cpu.prof"
	@echo "Wrote $(PGO_PROFILE) — now run 'make build-pgo'"

profile-mem:
	$(GO_ENV) go run $(GO_TAGS) ./cmd/renderer -memprofile "$(PGO_PROFILE)/mem.prof"

view-mem:
	go tool pprof -http=:6061 "$(PGO_PROFILE)/mem.prof"

view-cpu:
	go tool pprof -http=:6061 "$(PGO_PROFILE)/cpu.prof"

render-mem:
	go tool pprof -svg -output "$(PGO_PROFILE)/mem.svg" "$(PGO_PROFILE)/mem.prof"
	go tool pprof -png -output "$(PGO_PROFILE)/mem.png" "$(PGO_PROFILE)/mem.prof"
	@echo "wrote $(PGO_PROFILE)/mem.png and $(PGO_PROFILE)/mem.svg"

render-cpu:
	go tool pprof -svg -output "$(PGO_PROFILE)/cpu.svg" "$(PGO_PROFILE)/cpu.prof"
	go tool pprof -png -output "$(PGO_PROFILE)/cpu.png" "$(PGO_PROFILE)/cpu.prof"
	@echo "wrote $(PGO_PROFILE)/cpu.png $(PGO_PROFILE)/cpu.svg"

build-pgo:
	@test -f "$(PGO_PROFILE)/cpu.prof" || { echo "missing "$(PGO_PROFILE)/cpu.prof" — run 'make profile' first"; exit 1; }
	$(GO_ENV) go build $(GO_TAGS) "-pgo=$(PGO_PROFILE)/cpu.prof" -o renderer ./cmd/renderer

# Build the WebAssembly bundle into web/ (game.wasm + wasm_exec.js.old).
web:
	GOOS=js GOARCH=wasm $(GO_ENV) go build $(GO_TAGS) -pgo="$(PGO_PROFILE)/cpu.prof" -o web/game.wasm ./cmd/renderer
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" web/
	@echo "Built web/game.wasm ($$(du -h web/game.wasm | cut -f1))"

# Build the web bundle and serve it. Workers + wasm require http:// (not file://).
serve: web
	@echo "Serving http://localhost:8081  (Ctrl-C to stop)"
	cd web && python3 -m http.server 8081

# Every renderer/kernel combination: original, PR #1 scalar, PR #1 SIMD.
test:
	go test ./...
	go test -tags simd ./...
	GOEXPERIMENT=simd go test -tags simd ./...

# internal/... under js/wasm in Node: original geometry with scalar raster
# kernels, then (-tags simd) the per-vertex geometry with SIMD128 kernels.
# cmd/renderer is left out: ebiten's js backend needs a browser DOM.
# wasm_exec aborts on a large environment, so run with a minimal one.
test-wasm:
	env -i PATH="$$PATH" HOME="$$HOME" GOOS=js GOARCH=wasm go test -exec="$(GOROOT)/lib/wasm/go_js_wasm_exec" ./internal/...
	env -i PATH="$$PATH" HOME="$$HOME" GOOS=js GOARCH=wasm GOEXPERIMENT=simd go test -tags simd -exec="$(GOROOT)/lib/wasm/go_js_wasm_exec" ./internal/...

# Statement coverage of the three `make test` builds, merged into
# coverage/coverage.out (view with `go tool cover -html=coverage/coverage.out`).
# -coverpkg counts code run by any package's tests. Only packages with tests are
# run; they import every other package. Running -cover on a package with no tests
# makes go build covdata under GOEXPERIMENT=simd, which fails on some distro
# toolchains.
COVER_DIR := coverage
COVER_PKGS = $(shell go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./...)
COVER_ARGS = -cover -coverpkg=./... $(COVER_PKGS) -args -test.gocoverdir

cover:
	rm -rf $(COVER_DIR)
	mkdir -p $(COVER_DIR)/legacy $(COVER_DIR)/scalar $(COVER_DIR)/simd
	go test $(COVER_ARGS)=$(CURDIR)/$(COVER_DIR)/legacy
	go test -tags simd $(COVER_ARGS)=$(CURDIR)/$(COVER_DIR)/scalar
	GOEXPERIMENT=simd go test -tags simd $(COVER_ARGS)=$(CURDIR)/$(COVER_DIR)/simd
	go tool covdata textfmt -i=$(COVER_DIR)/legacy,$(COVER_DIR)/scalar,$(COVER_DIR)/simd -o $(COVER_DIR)/coverage.out
	go tool cover -func=$(COVER_DIR)/coverage.out | tail -n 1

# gofmt, then vet every build: original, PR #1 scalar, PR #1 SIMD, and wasm.
# -composites=false: the original code uses unkeyed struct literals.
lint:
	@unformatted="$$(gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	go vet -composites=false ./...
	go vet -composites=false -tags simd ./...
	GOEXPERIMENT=simd go vet -composites=false -tags simd ./...
	GOOS=js GOARCH=wasm go vet -composites=false ./...
	GOOS=js GOARCH=wasm GOEXPERIMENT=simd go vet -composites=false -tags simd ./...

# internal/pipeline always uses the new rasterizer; SIMD=1 picks the SIMD kernels
# and the per-vertex geometry (otherwise it's master's per-face loop).
bench:
	$(GO_ENV) go test $(GO_TAGS) -run '^$$' -bench . -count 5 ./internal/pipeline/

# wasm_exec aborts on a large environment, so run with a minimal one.
bench-wasm:
	env -i PATH="$$PATH" HOME="$$HOME" GOOS=js GOARCH=wasm $(GO_ENV) go test $(GO_TAGS) -exec="$(GOROOT)/lib/wasm/go_js_wasm_exec" -run '^$$' -bench . -count 5 ./internal/pipeline/

# The default build vs -tags simd vs -tags simd with SIMD kernels, on the
# cmd/renderer frame benchmarks: interleaved, pinned to one CPU, compared with
# benchstat (~12 minutes). Settings: see scripts/bench-compare.sh.
bench-compare:
	scripts/bench-compare.sh

clean:
	rm -f renderer web/game.wasm "$(PGO_PROFILE)/cpu.prof" "$(PGO_PROFILE)/mem.prof"
