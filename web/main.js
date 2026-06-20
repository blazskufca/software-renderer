// Main thread.
//
// Spawns the worker loader to fetch + compile the wasm, then instantiates and
// runs the Go/Ebiten program here on the main thread (Ebiten needs the DOM).

const overlay = document.getElementById("overlay");
const statusEl = document.getElementById("status");
const errorEl = document.getElementById("error");

function setStatus(text) {
  statusEl.textContent = text;
}

function fail(message) {
  statusEl.textContent = "Failed to load";
  errorEl.textContent = message;
  console.error(message);
}

function hideOverlay() {
  overlay.classList.add("hidden");
  setTimeout(() => { overlay.style.display = "none"; }, 400);
}

if (typeof Go === "undefined") {
  fail("wasm_exec.js.old did not load (Go is undefined).");
} else if (typeof Worker === "undefined") {
  fail("This browser does not support Web Workers.");
} else {
  const go = new Go();
  const worker = new Worker("worker.js");

  worker.onmessage = async (e) => {
    const msg = e.data || {};
    switch (msg.type) {
      case "status":
        setStatus(msg.text);
        break;

      case "compiled":
        try {
          setStatus("Starting…");
          const instance = await WebAssembly.instantiate(msg.module, go.importObject);
          worker.terminate();
          hideOverlay();
          // go.run resolves when the Go program exits (e.g. user presses Esc).
          await go.run(instance);
        } catch (err) {
          fail(String(err && err.stack || err));
        }
        break;

      case "error":
        fail(msg.error);
        break;
    }
  };

  worker.onerror = (e) => fail("Worker error: " + e.message);

  setStatus("Loading…");
  worker.postMessage({ type: "load", url: "game.wasm" });
}
