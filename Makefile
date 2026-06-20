.PHONY: run build web serve clean

GOROOT := $(shell go env GOROOT)

# Run the native (desktop) build.
run:
	go run ./src

# Build the native binary.
build:
	go build -o renderer ./src

# Build the WebAssembly bundle into web/ (game.wasm + wasm_exec.js.old).
web:
	GOOS=js GOARCH=wasm go build -o web/game.wasm ./src
	cp "$(GOROOT)/lib/wasm/wasm_exec.js" web/
	@echo "Built web/game.wasm ($$(du -h web/game.wasm | cut -f1))"

# Build the web bundle and serve it. Workers + wasm require http:// (not file://).
serve: web
	@echo "Serving http://localhost:8080  (Ctrl-C to stop)"
	cd web && python3 -m http.server 8080

clean:
	rm -f renderer web/game.wasm
