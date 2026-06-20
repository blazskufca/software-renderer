//go:build js

package main

import (
	"embed"
	"io"
)

// On wasm there is no filesystem, so the assets the demo needs are baked into
// the binary. Only the files referenced by setup() are embedded — embedding the
// whole assets/ directory would pull in multi-hundred-MB models.
//
//go:embed assets/f18-superhornet.obj assets/Diffuse.jpg assets/f22.obj assets/f22.png
var assetsFS embed.FS

// openAsset reads an embedded asset. Paths match the disk layout ("assets/..."),
// so callers are identical on native and wasm builds.
func openAsset(path string) (io.ReadCloser, error) {
	return assetsFS.Open(path)
}
