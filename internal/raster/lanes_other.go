//go:build goexperiment.simd && !wasm

package raster

import "simd"

func anyLane(m simd.Mask32s, buf []int32) bool { return anyLaneMemory(m, buf) }

func gatherLanes(idx simd.Int32s, pix []uint32, idxBuf []int32, texelBuf []uint32) simd.Uint32s {
	return gatherMemory(idx, pix, idxBuf, texelBuf)
}
