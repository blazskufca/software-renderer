.PHONY: run build build-pgo profile web serve clean profile-mem view-mem view-cpu render-mem render-cpu test bench bench-wasm

GOROOT := $(shell go env GOROOT)

PGO_PROFILE := profiling

# Portable SIMD (Go 1.27+). `make <target> SIMD=` builds the scalar version.
SIMD ?= GOEXPERIMENT=simd

# Run the native (desktop) build.
run:
	$(SIMD) go run ./cmd/renderer

# Build the native binary.
build:
	$(SIMD) go build -o renderer ./cmd/renderer

profile:
	$(SIMD) go run ./cmd/renderer -cpuprofile "$(PGO_PROFILE)/cpu.prof"
	@echo "Wrote $(PGO_PROFILE) — now run 'make build-pgo'"

profile-mem:
	$(SIMD) go run ./cmd/renderer -memprofile "$(PGO_PROFILE)/mem.prof"

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
	$(SIMD) go build "-pgo=$(PGO_PROFILE)/cpu.prof" -o renderer ./cmd/renderer

# Build the WebAssembly bundle into web/ (game.wasm + wasm_exec.js.old).
web:
	GOOS=js GOARCH=wasm $(SIMD) go build -pgo="$(PGO_PROFILE)/cpu.prof" -o web/game.wasm ./cmd/renderer
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" web/
	@echo "Built web/game.wasm ($$(du -h web/game.wasm | cut -f1))"

# Build the web bundle and serve it. Workers + wasm require http:// (not file://).
serve: web
	@echo "Serving http://localhost:8080  (Ctrl-C to stop)"
	cd web && python3 -m http.server 8080

test:
	go test ./...
	GOEXPERIMENT=simd go test ./...

bench:
	$(SIMD) go test -run '^$$' -bench . -count 5 ./internal/pipeline/

# wasm_exec aborts on a large environment, so run with a minimal one.
bench-wasm:
	env -i PATH="$$PATH" HOME="$$HOME" GOOS=js GOARCH=wasm $(SIMD) go test -exec="$(GOROOT)/lib/wasm/go_js_wasm_exec" -run '^$$' -bench . -count 5 ./internal/pipeline/

clean:
	rm -f renderer web/game.wasm "$(PGO_PROFILE)/cpu.prof" "$(PGO_PROFILE)/mem.prof"
