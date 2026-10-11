//go:build js

package assets

import "embed"

//go:embed race.obj colormap.png forest roads cars camp pets
var AssetsFS embed.FS
