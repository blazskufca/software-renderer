package main

var (
	globalCamera = camera{
		direction: vec3{Z: 1},
	}
)

type camera struct {
	position        vec3
	direction       vec3
	forwardVelocity vec3
	yawAngle        float64
}
