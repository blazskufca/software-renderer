//go:build js

package assets

import (
	"io"
	"strings"

	"github.com/blazskufca/software-renderer/assets"
)

// On wasm there is no filesystem, so the assets the demo needs are baked into
// the binary. Only the files referenced by setup() are embedded — embedding the
// whole assets/ directory would pull in multi-hundred-MB models.
//

// OpenAsset reads an embedded asset. Paths match the disk layout ("assets/..."),
// so callers are identical on native and wasm builds.
func OpenAsset(path string) (io.ReadCloser, error) {
	// The embed directive lives inside the assets/ package, so embedded files are
	// rooted there ("race.obj"), while callers pass the disk-layout path
	// ("assets/race.obj"). Strip the prefix so both builds use identical paths.
	return assets.AssetsFS.Open(strings.TrimPrefix(path, "assets/"))
}
