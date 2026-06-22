//go:build js

package assets

import "embed"

//go:embed race.obj colormap.png
var AssetsFS embed.FS
