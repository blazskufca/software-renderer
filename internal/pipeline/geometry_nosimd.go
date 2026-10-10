//go:build simd && !goexperiment.simd

package pipeline

import (
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/matrix"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

func transformVerticesSIMD[T utils.Numeric](*vertexData[T], []vector.Vec3[T], *matrix.Mat4[T], *Params[T]) int {
	return 0
}

func shadeFacesSIMD[T utils.Numeric](*faceData[T], []mesh.Face[T], *vertexData[T], *matrix.Mat4[T], *Params[T]) int {
	return 0
}
