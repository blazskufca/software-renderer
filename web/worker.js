// Worker loader.
//
// The Web Worker downloads and compiles the .wasm module off the main thread,
// then hands the compiled WebAssembly.Module back so the page stays responsive
// (spinner keeps animating) during the multi-megabyte fetch + compile.
//
// Why only *compile* here and not run the game: Ebiten's wasm backend creates
// its <canvas> via `document.createElement` and binds input to `window`. Neither
// `document` nor `window` exists inside a Worker, so the game itself must run on
// the main thread. The worker does the heavy loading; the main thread runs it.

self.onmessage = async (e) => {
  if (e.data && e.data.type !== "load") return;
  const url = (e.data && e.data.url) || "game.wasm";
  try {
    self.postMessage({ type: "status", text: "Fetching " + url + "…" });
    const resp = await fetch(url);
    if (!resp.ok) {
      throw new Error("HTTP " + resp.status + " fetching " + url);
    }

    let module;
    if (typeof WebAssembly.compileStreaming === "function") {
      self.postMessage({ type: "status", text: "Compiling…" });
      module = await WebAssembly.compileStreaming(resp.clone());
    } else {
      // Safari fallback: no streaming compile.
      self.postMessage({ type: "status", text: "Compiling…" });
      const bytes = await resp.arrayBuffer();
      module = await WebAssembly.compile(bytes);
    }

    // WebAssembly.Module is structured-cloneable, so it transfers to the main
    // thread without re-compiling.
    self.postMessage({ type: "compiled", module });
  } catch (err) {
    self.postMessage({ type: "error", error: String(err && err.stack || err) });
  }
};
