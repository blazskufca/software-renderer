//go:build !js

package assets

import (
	"io"
	"os"
)

// OpenAsset reads an asset from disk on native builds, so the full assets/
// directory (every model and texture) stays available at runtime.
func OpenAsset(path string) (io.ReadCloser, error) {
	return os.Open(path)
}
