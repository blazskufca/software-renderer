package pipeline

import (
	"github.com/blazskufca/software-renderer/internal/matrix"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

func ViewMatrix[T utils.Numeric](position vector.Vec3[T], yaw, pitch float32) (matrix.Mat4[T], vector.Vec3[T]) {
	rotation := matrix.Mat4MakeRotationY[T](T(yaw)).MulMat4(matrix.Mat4MakeRotationX[T](T(pitch)))
	direction := rotation.MulVec4(vector.Vec3[T]{Z: 1}.ToVec4()).ToVec3()
	return matrix.Mat4LookAt(position, position.Add(direction), vector.Vec3[T]{Y: 1}), direction
}
