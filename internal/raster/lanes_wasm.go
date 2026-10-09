//go:build goexperiment.simd && wasm

package raster

import (
	"simd"
	"simd/archsimd"
)

// wasm can read and write single lanes, so these skip the round trip through memory

func anyLane(m simd.Mask32s, buf []int32) bool {
	i, ok := m.ToInt32s().ToArch().(archsimd.Int32x4)
	if !ok { // emulated vectors
		return anyLaneMemory(m, buf)
	}
	return i.GetElem(0)|i.GetElem(1)|i.GetElem(2)|i.GetElem(3) != 0
}

func gatherLanes(idx simd.Int32s, pix []uint32, idxBuf []int32, texelBuf []uint32) simd.Uint32s {
	i, ok := idx.ToArch().(archsimd.Int32x4)
	if !ok {
		return gatherMemory(idx, pix, idxBuf, texelBuf)
	}
	var t archsimd.Uint32x4
	t = t.SetElem(0, pix[i.GetElem(0)])
	t = t.SetElem(1, pix[i.GetElem(1)])
	t = t.SetElem(2, pix[i.GetElem(2)])
	t = t.SetElem(3, pix[i.GetElem(3)])
	return simd.Uint32sFromArch(t)
}
