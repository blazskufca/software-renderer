.PHONY: run build build-pgo profile web serve clean profile-mem view-mem view-cpu render-mem render-cpu test bench

GOROOT := $(shell go env GOROOT)

PGO_PROFILE := profiling

# The SIMD rasterizers (cmd/renderer/raster_simd.go) need the Go 1.27 `simd`
# package, which lives behind an experiment flag. Everything still builds without
# it — raster_simd_stub.go takes over and the SIMD toolbar buttons disappear — so
# unset this if you want to compare against a plain build.
export GOEXPERIMENT = simd

# Run the native (desktop) build.
run:
	go run ./cmd/renderer

# Build the native binary.
build:
	go build -o renderer ./cmd/renderer

profile:
	go run ./cmd/renderer -cpuprofile "$(PGO_PROFILE)/cpu.prof"
	@echo "Wrote $(PGO_PROFILE) — now run 'make build-pgo'"

profile-mem:
	go run ./cmd/renderer -memprofile "$(PGO_PROFILE)/mem.prof"

test:
	go test ./...

# Compares each rasterizer backend against the others; the SIMD benchmarks skip
# themselves on a build without the experiment.
bench:
	go test ./cmd/renderer -run XXX -bench . -benchtime 2s

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
	go build "-pgo=$(PGO_PROFILE)/cpu.prof" -o renderer ./cmd/renderer

# Build the WebAssembly bundle into web/ (game.wasm + wasm_exec.js.old).
web:
	GOOS=js GOARCH=wasm go build -pgo="$(PGO_PROFILE)/cpu.prof" -o web/game.wasm ./cmd/renderer
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" web/
	@echo "Built web/game.wasm ($$(du -h web/game.wasm | cut -f1))"

# Build the web bundle and serve it. Workers + wasm require http:// (not file://).
serve: web
	@echo "Serving http://localhost:8080  (Ctrl-C to stop)"
	cd web && python3 -m http.server 8080

clean:
	rm -f renderer web/game.wasm "$(PGO_PROFILE)/cpu.prof" "$(PGO_PROFILE)/mem.prof"
