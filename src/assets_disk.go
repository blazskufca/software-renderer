//go:build !js

package main

import (
	"io"
	"os"
)

// openAsset reads an asset from disk on native builds, so the full assets/
// directory (every model and texture) stays available at runtime.
func openAsset(path string) (io.ReadCloser, error) {
	return os.Open(path)
}
