package camera

import (
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

type Camera[T utils.Numeric] struct {
	Position        vector.Vec3[T]
	Direction       vector.Vec3[T]
	ForwardVelocity vector.Vec3[T]
	RightVelocity   vector.Vec3[T]
	PitchAngle      float32 // radians; positive tilts the view down
	YawAngle        float32
}
