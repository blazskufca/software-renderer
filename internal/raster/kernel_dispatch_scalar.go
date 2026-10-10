//go:build !goexperiment.simd

package raster

import mesh "github.com/blazskufca/software-renderer/internal/face"

func fillRows(t *Target, s *triSetup, color uint32) { fillRowsScalar(t, s, color) }

func textureRows(t *Target, s *triSetup, tex *mesh.TextureData) { textureRowsScalar(t, s, tex) }
