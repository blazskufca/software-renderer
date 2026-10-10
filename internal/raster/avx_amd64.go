//go:build goexperiment.simd && amd64

package raster

import "simd/archsimd"

// VZEROUPPER: Go's scalar float code is legacy SSE; dirty YMM upper halves make it stall.
func clearUpperBits() { archsimd.ClearAVXUpperBits() }
