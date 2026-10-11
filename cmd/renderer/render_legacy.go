//go:build !simd

// Default backend: master's generic scanline and Pineda rasterizers and the
// per-face geometry loop. `-tags simd` swaps in render_simd.go instead.

package main

import (
	"encoding/binary"
	"math"

	"github.com/blazskufca/software-renderer/internal/clipping"
	"github.com/blazskufca/software-renderer/internal/config"
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/light"
	"github.com/blazskufca/software-renderer/internal/matrix"
	"github.com/blazskufca/software-renderer/internal/texture"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
	"github.com/hajimehoshi/ebiten/v2"
)

// rasterState is the backend-specific part of Renderer.
type rasterState[T utils.Numeric] struct {
	colorBuffer       []byte
	zBuffer           []T
	trianglesToDraw   []triangle[T]
	textureRasterFunc config.TexturizedRasterizer[T]
	fillRasterFunc    config.FillRasterizer[T]
}

// setFillRaster selects the triangle fill rasterizer and records the choice so
// the UI can reflect which backend is active (func values aren't comparable, so
// we track the mode enum alongside the func).
func (r *Renderer[T]) setFillRaster(mode config.FillRasterMode) {
	r.fillRasterMode = mode
	switch mode {
	case config.FILL_PINEDA:
		r.fillRasterFunc = r.drawFilledTrianglePineda
	default:
		r.fillRasterFunc = r.drawFilledTriangle
	}
}

// setTextureRaster selects the texture-mapping rasterizer and records the choice
// so the UI can reflect which backend is active.
func (r *Renderer[T]) setTextureRaster(mode config.TextureRasterizer) {
	r.textureRasterMode = mode
	switch mode {
	case config.TEXTURE_PINEDA:
		r.textureRasterFunc = r.drawTexturedTrianglePineda
	default:
		r.textureRasterFunc = r.drawTexturedTriangle
	}
}

func (r *Renderer[T]) initBuffers() error {
	r.colorBuffer = make([]byte, r.config.WindowWidth*r.config.WindowHeight*4)
	r.zBuffer = make([]T, r.config.WindowWidth*r.config.WindowHeight)
	return nil
}

func (r *Renderer[T]) drawTriangles() {
	for i := 0; i < len(r.trianglesToDraw); i++ {
		triangle := r.trianglesToDraw[i]
		if r.renderMethod == config.RENDER_FILL_TRIANGLE || r.renderMethod == config.RENDER_FILL_TRIANGLE_WIRE {
			r.fillRasterFunc(
				triangle.points[0].X, triangle.points[0].Y, triangle.points[0].Z, triangle.points[0].W,
				triangle.points[1].X, triangle.points[1].Y, triangle.points[1].Z, triangle.points[1].W,
				triangle.points[2].X, triangle.points[2].Y, triangle.points[2].Z, triangle.points[2].W,
				triangle.color,
				triangle.lightIntensity[0], triangle.lightIntensity[1], triangle.lightIntensity[2])
		}

		if r.renderMethod == config.RENDER_TEXTURED || r.renderMethod == config.RENDER_TEXTURED_WIRE {
			r.textureRasterFunc(
				triangle.points[0].X, triangle.points[0].Y, triangle.points[0].Z, triangle.points[0].W, triangle.textCoords[0].U, triangle.textCoords[0].V,
				triangle.points[1].X, triangle.points[1].Y, triangle.points[1].Z, triangle.points[1].W, triangle.textCoords[1].U, triangle.textCoords[1].V,
				triangle.points[2].X, triangle.points[2].Y, triangle.points[2].Z, triangle.points[2].W, triangle.textCoords[2].U, triangle.textCoords[2].V,
				triangle.lightIntensity[0], triangle.lightIntensity[1], triangle.lightIntensity[2], triangle.texture)
		}

		if r.renderMethod == config.RENDER_WIRE || r.renderMethod == config.RENDER_WIRE_VERTEX || r.renderMethod == config.RENDER_FILL_TRIANGLE_WIRE || r.renderMethod == config.RENDER_TEXTURED_WIRE {
			r.drawTriangle(int(triangle.points[0].X), int(triangle.points[0].Y),
				int(triangle.points[1].X), int(triangle.points[1].Y),
				int(triangle.points[2].X), int(triangle.points[2].Y), 0xFFFFFFFF)
		}
		if r.renderMethod == config.RENDER_WIRE_VERTEX {
			r.drawRectangle(int(triangle.points[0].X)-3, int(triangle.points[0].Y)-3, 6, 6, 0xFFFFFF00)
			r.drawRectangle(int(triangle.points[1].X)-3, int(triangle.points[1].Y)-3, 6, 6, 0xFFFFFF00)
			r.drawRectangle(int(triangle.points[2].X)-3, int(triangle.points[2].Y)-3, 6, 6, 0xFFFFFF00)
		}

	}
}

func (r *Renderer[T]) clearBuffers() {
	// Clear color buffer
	r.clearColorBuffer(skyColor)
	// Clear Z-buffer
	r.clearZBuffer(1)
}

// Pixels are stored as four bytes in R,G,B,A order (matching ebiten's
// WritePixels). Our colors are packed 0xAABBGGRR, so a little-endian write lays
// the bytes down as R,G,B,A.
//
// We seed the first pixel and then replicate it across the buffer by repeatedly
// copying the already-filled prefix onto the remainder. Each copy doubles the
// filled region, so the runtime turns the clear into a handful of bulk memmoves
// instead of one PutUint32 per pixel.
func (r *Renderer[T]) clearColorBuffer(newColor uint32) {
	if len(r.colorBuffer) < 4 {
		return
	}
	binary.LittleEndian.PutUint32(r.colorBuffer[:4], newColor)
	for filled := 4; filled < len(r.colorBuffer); filled *= 2 {
		copy(r.colorBuffer[filled:], r.colorBuffer[:filled])
	}
}

// clearZBuffer resets every depth sample to newZ using the same copy-doubling
// trick as clearColorBuffer: seed element 0, then replicate the filled prefix.
func (r *Renderer[T]) clearZBuffer(newZ T) {
	if len(r.zBuffer) == 0 {
		return
	}
	r.zBuffer[0] = newZ
	for filled := 1; filled < len(r.zBuffer); filled *= 2 {
		copy(r.zBuffer[filled:], r.zBuffer[:filled])
	}
}

// renderColorBuffer hands the color buffer straight to ebiten. The buffer is
// already a flat R,G,B,A byte slice, so there is nothing to convert or copy.
func (r *Renderer[T]) renderColorBuffer(screen *ebiten.Image) {
	screen.WritePixels(r.colorBuffer)
}

func (r *Renderer[T]) drawPixel(x, y int, color uint32) {
	if x >= 0 && y >= 0 && x < r.config.WindowWidth && y < r.config.WindowHeight {
		idx := ((y * r.config.WindowWidth) + x) * 4
		binary.LittleEndian.PutUint32(r.colorBuffer[idx:], color)
	}
}

func (r *Renderer[T]) update() {
	// ebiten drives the frame pacing (fixed TPS), so we no longer sleep here;
	// deltaTime is measured in Update, before input is processed.
	r.trianglesToDraw = r.trianglesToDraw[:0]

	for _, currentMesh := range r.meshes {
		if currentMesh == nil {
			panic("nil currentMesh")
		}
		for _, mutation := range currentMesh.PerFrameMutations {
			err := mutation(currentMesh)
			if err != nil {
				panic(err)
			}
		}
		//currentMesh.rotation.X += math.Pi / 2 * deltaTime
		//currentMesh.rotation.Y += math.Pi / 2 * deltaTime
		//currentMesh.rotation.Y += math.Pi / 2 * deltaTime
		//currentMesh.rotation.Z += math.Pi / 2 * deltaTime
		//currentMesh.translation.Z = 5
		//globalCamera.position.X += 2.0 * deltaTime
		//globalCamera.position.Y += 2.0 * deltaTime

		//currentMesh.scale.X += 0.002
		//currentMesh.scale.Y += 0.001
		//currentMesh.translation.X += 0.01

		scaleMatrix := matrix.Mat4MakeScale[T](currentMesh.Scale.X, currentMesh.Scale.Y, currentMesh.Scale.Z)
		translationMatrix := matrix.Mat4MakeTranslation[T](currentMesh.Translation.X, currentMesh.Translation.Y, currentMesh.Translation.Z)
		rotationMatrixX, rotationMatrixY, rotationMatrixZ := matrix.Mat4MakeRotationX[T](currentMesh.Rotation.X), matrix.Mat4MakeRotationY[T](currentMesh.Rotation.Y), matrix.Mat4MakeRotationZ[T](currentMesh.Rotation.Z)
		// Create a view matrix
		// Start the target as normalized Z axis
		target := vector.Vec3[T]{Z: 1}

		cameraYaw := matrix.Mat4MakeRotationY[T](T(r.camera.YawAngle))
		cameraPitch := matrix.Mat4MakeRotationX[T](T(r.camera.PitchAngle))
		cameraRotation := cameraYaw.MulMat4(cameraPitch)
		r.camera.Direction = cameraRotation.MulVec4(target.ToVec4()).ToVec3()

		target = r.camera.Position.Add(r.camera.Direction)

		viewMatrix := matrix.Mat4LookAt(r.camera.Position, target, vector.Vec3[T]{Y: 1})

		// order matters
		// Scale -> rotate -> translate
		worldMatrix := matrix.Mat4Identity[T]()
		worldMatrix = scaleMatrix.MulMat4(worldMatrix)
		worldMatrix = rotationMatrixZ.MulMat4(worldMatrix)
		worldMatrix = rotationMatrixY.MulMat4(worldMatrix)
		worldMatrix = rotationMatrixX.MulMat4(worldMatrix)
		worldMatrix = translationMatrix.MulMat4(worldMatrix)
		worldMatrix = viewMatrix.MulMat4(worldMatrix)

		for i := 0; i < len(currentMesh.Faces); i++ {
			// Face index into the vertices array
			//if i != 4 {
			//	continue
			//}
			meshFace := currentMesh.Faces[i]
			facesVertices := [3]vector.Vec3[T]{}

			facesVertices[0] = currentMesh.Vertices[meshFace.A]
			facesVertices[1] = currentMesh.Vertices[meshFace.B]
			facesVertices[2] = currentMesh.Vertices[meshFace.C]
			var transformedVertexArr [3]vector.Vec4[T]
			for j := 0; j < 3; j++ {
				transformedVertex := facesVertices[j].ToVec4()
				transformedVertex = worldMatrix.MulVec4(transformedVertex)
				transformedVertexArr[j] = transformedVertex // Save current vertex in arr of transformedVertex
			}
			vectorA, vectorB, vectorC := transformedVertexArr[0].ToVec3(), transformedVertexArr[1].ToVec3(), transformedVertexArr[2].ToVec3()
			BAVector, CAVector := vectorB.Sub(vectorA), vectorC.Sub(vectorA)
			// For vectors BA and CA we also only care about the direction and not the magnitude
			BAVector.Normalize()
			CAVector.Normalize()

			// We are using a left-handed coordinate system (i.e Z grows inside the monitor)
			// Face normal vector can be normalized
			faceNormal := BAVector.Cross(CAVector)
			faceNormal.Normalize()

			flatShadePercent := -faceNormal.Dot(r.light.Direction)
			faceIntensities := [3]T{flatShadePercent, flatShadePercent, flatShadePercent}

			// Per-vertex light intensities for Gouraud shading.
			if r.shadingMode == config.SHADING_GOURAUD {
				vertexNormals := [3]vector.Vec3[T]{meshFace.ANormal, meshFace.BNormal, meshFace.CNormal}
				var vertexIntensities [3]T
				for j := 0; j < 3; j++ {
					n := vertexNormals[j]
					transformedNormal := worldMatrix.MulVec4(vector.Vec4[T]{X: n.X, Y: n.Y, Z: n.Z}).ToVec3()
					if transformedNormal.Length() == 0 {
						transformedNormal = faceNormal
					} else {
						transformedNormal.Normalize()
					}
					vertexIntensities[j] = -transformedNormal.Dot(r.light.Direction)
				}
				faceIntensities = vertexIntensities
			}

			if r.cullMode == config.CULL_BACKFACE {
				// Backface culling
				origin := vector.Vec3[T]{} // Camera position can change so we need a real 0, 0, 0
				cameraRayVec := origin.Sub(vectorA)

				cameraRayFaceNormalDot := faceNormal.Dot(cameraRayVec)

				// If dot product is less than 0 we should hide the face
				if cameraRayFaceNormalDot < 0 {
					continue
				}
			}

			// Clipping stage
			poly := clipping.NewPolyFromTriangle(&transformedVertexArr, &[3]texture.Tex2[T]{meshFace.AUV, meshFace.BUV, meshFace.CUV}, &faceIntensities)
			poly.ClipPolygon(&r.frustumPlanes)
			// Break the clipped polygon apart into triangles (fan triangulation)
			var clippedTriangles [10]triangle[T]
			var numberOfClippedTriangles int = poly.NumberOfVertices - 2
			for t := 0; t < numberOfClippedTriangles; t++ {
				tr := triangle[T]{}
				tr.points = [3]vector.Vec4[T]{poly.Vertices[0].ToVec4(), poly.Vertices[t+1].ToVec4(), poly.Vertices[t+2].ToVec4()}
				tr.textCoords = [3]texture.Tex2[T]{poly.TexCoords[0], poly.TexCoords[t+1], poly.TexCoords[t+2]}
				tr.lightIntensity = [3]T{poly.LightIntensities[0], poly.LightIntensities[t+1], poly.LightIntensities[t+2]}
				clippedTriangles[t] = tr
			}
			for i := 0; i < numberOfClippedTriangles; i++ {
				clippedTriangle := clippedTriangles[i]

				var projectedPoints [3]vector.Vec4[T]
				for j := 0; j < len(transformedVertexArr); j++ {
					projectedPoints[j] = r.projectionMatrix.Mat4MulVec4Project(clippedTriangle.points[j])

					// Scale into the view
					projectedPoints[j].X *= T(r.config.WindowWidth / 2)
					projectedPoints[j].Y *= T(r.config.WindowHeight / 2)

					// Invert Y values to account for difference between Y0 of color buffer (top left) and obj files
					projectedPoints[j].Y *= -1
					// Translate the projected point into the middle of the screen
					projectedPoints[j].X += T(r.config.WindowWidth / 2)
					projectedPoints[j].Y += T(r.config.WindowHeight / 2)
				}
				var projectedTriangle triangle[T]
				projectedTriangle.color = meshFace.Color
				projectedTriangle.lightIntensity = clippedTriangle.lightIntensity
				projectedTriangle.points[0] = projectedPoints[0]
				projectedTriangle.points[1] = projectedPoints[1]
				projectedTriangle.points[2] = projectedPoints[2]
				projectedTriangle.textCoords[0] = clippedTriangle.textCoords[0]
				projectedTriangle.textCoords[1] = clippedTriangle.textCoords[1]
				projectedTriangle.textCoords[2] = clippedTriangle.textCoords[2]
				projectedTriangle.avgDepth = (transformedVertexArr[0].Z + transformedVertexArr[1].Z + transformedVertexArr[2].Z) / 3
				projectedTriangle.texture = &currentMesh.Texture
				r.trianglesToDraw = append(r.trianglesToDraw, projectedTriangle)
			}
		}
	}

}

// triangle stores actual vec2 points of the triangle projected onto the screen
type triangle[T utils.Numeric] struct {
	points     [3]vector.Vec4[T]
	textCoords [3]texture.Tex2[T]
	color      uint32
	avgDepth   T
	// lightIntensity is the light intensity at each of the three vertices. For
	// flat shading all three are equal (the face-normal intensity); for Gouraud
	// shading each holds its vertex's own intensity and the rasterizer
	// interpolates between them.
	lightIntensity [3]T
	texture        *mesh.TextureData
}

func (r *Renderer[T]) drawFilledTriangle(x0f, y0f, z0, w0, x1f, y1f, z1, w1, x2f, y2f, z2, w2 T, color uint32, i0, i1, i2 T) {
	// Snap screen coords to integers, like the C reference (keep z/w as float for depth)
	x0, y0 := int(x0f), int(y0f)
	x1, y1 := int(x1f), int(y1f)
	x2, y2 := int(x2f), int(y2f)

	if y0 > y1 {
		utils.SwapNumericPtr(&x0, &x1)
		utils.SwapNumericPtr(&y0, &y1)
		utils.SwapNumericPtr(&z0, &z1)
		utils.SwapNumericPtr(&w0, &w1)
		utils.SwapNumericPtr(&i0, &i1)
	}
	if y1 > y2 {
		utils.SwapNumericPtr(&x1, &x2)
		utils.SwapNumericPtr(&y1, &y2)
		utils.SwapNumericPtr(&z1, &z2)
		utils.SwapNumericPtr(&w1, &w2)
		utils.SwapNumericPtr(&i1, &i2)
	}
	if y0 > y1 {
		utils.SwapNumericPtr(&x0, &x1)
		utils.SwapNumericPtr(&y0, &y1)
		utils.SwapNumericPtr(&z0, &z1)
		utils.SwapNumericPtr(&w0, &w1)
		utils.SwapNumericPtr(&i0, &i1)
	}

	pointA := vector.Vec4[T]{T(x0), T(y0), T(z0), T(w0)}
	pointB := vector.Vec4[T]{T(x1), T(y1), T(z1), T(w1)}
	pointC := vector.Vec4[T]{T(x2), T(y2), T(z2), T(w2)}

	// top half
	var invSlope1, invSlope2 T
	if y1-y0 != 0 {
		invSlope1 = T(x1-x0) / T(math.Abs(float64(y1-y0)))
	}
	if y2-y0 != 0 {
		invSlope2 = T(x2-x0) / T(math.Abs(float64(y2-y0)))
	}
	if y1-y0 != 0 {
		for y := y0; y <= y1; y++ {
			xStart := int(T(x1) + T(y-y1)*invSlope1) // single truncation, no Round
			xEnd := int(T(x0) + T(y-y0)*invSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTrianglePixel(x, y, color, pointA, pointB, pointC, i0, i1, i2)
			}
		}
	}
	// bottom half
	invSlope1, invSlope2 = 0, 0
	if y2-y1 != 0 {
		invSlope1 = T(x2-x1) / T(math.Abs(float64(y2-y1)))
	}
	if y2-y0 != 0 {
		invSlope2 = T(x2-x0) / T(math.Abs(float64(y2-y0)))
	}
	if y2-y1 != 0 {
		for y := y1; y <= y2; y++ {
			xStart := int(T(x1) + T(y-y1)*invSlope1)
			xEnd := int(T(x0) + T(y-y0)*invSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTrianglePixel(x, y, color, pointA, pointB, pointC, i0, i1, i2)
			}
		}
	}
}

func (r *Renderer[T]) drawTrianglePixel(x, y int, color uint32, a, b, c vector.Vec4[T], i0, i1, i2 T) {
	if x < 0 || x >= r.config.WindowWidth || y < 0 || y >= r.config.WindowHeight {
		return
	}
	p := vector.Vec2[T]{T(x), T(y)}
	aVec2 := a.ToVec2()
	bVec2 := b.ToVec2()
	cVec2 := c.ToVec2()

	weights := barycentricWeights(aVec2, bVec2, cVec2, p)
	alpha, beta, gamma := weights.X, weights.Y, weights.Z
	interpolatedReciprocalW := (1/a.W)*alpha + (1/b.W)*beta + (1/c.W)*gamma
	interpolatedReciprocalW = 1 - interpolatedReciprocalW

	if interpolatedReciprocalW < r.zBuffer[(r.config.WindowWidth*y)+x] {
		// Interpolate the per-vertex light intensity across the face (Gouraud);
		// for flat shading all three are equal so this is a constant.
		intensity := i0*alpha + i1*beta + i2*gamma
		r.drawPixel(x, y, light.LightApplyIntensity(color, float32(intensity)))
		// Update the Z-buffer with 1/w for current pixel
		r.zBuffer[(r.config.WindowWidth*y)+x] = interpolatedReciprocalW
	}

}

func (r *Renderer[T]) drawTexturedTriangle(x0, y0, z0, w0, u0, v0, x1, y1, z1, w1, u1, v1, x2, y2, z2, w2, u2, v2, i0, i1, i2 T, t *mesh.TextureData) {
	ix0, iy0 := int(x0), int(y0)
	ix1, iy1 := int(x1), int(y1)
	ix2, iy2 := int(x2), int(y2)

	// Triangles vertices need to be sorted by the Y component here too
	// But when we swap not only do we need X and Y, we also have to swap the UV coordinates
	if iy0 > iy1 {
		utils.SwapNumericPtr(&iy0, &iy1)
		utils.SwapNumericPtr(&ix0, &ix1)
		utils.SwapNumericPtr(&u0, &u1)
		utils.SwapNumericPtr(&v0, &v1)
		utils.SwapNumericPtr(&z0, &z1)
		utils.SwapNumericPtr(&w0, &w1)
		utils.SwapNumericPtr(&i0, &i1)

	}
	if iy1 > iy2 {
		utils.SwapNumericPtr(&iy1, &iy2)
		utils.SwapNumericPtr(&ix1, &ix2)
		utils.SwapNumericPtr(&u1, &u2)
		utils.SwapNumericPtr(&v1, &v2)
		utils.SwapNumericPtr(&z1, &z2)
		utils.SwapNumericPtr(&w1, &w2)
		utils.SwapNumericPtr(&i1, &i2)
	}
	if iy0 > iy1 {
		utils.SwapNumericPtr(&iy0, &iy1)
		utils.SwapNumericPtr(&ix0, &ix1)
		utils.SwapNumericPtr(&u0, &u1)
		utils.SwapNumericPtr(&v0, &v1)
		utils.SwapNumericPtr(&z0, &z1)
		utils.SwapNumericPtr(&w0, &w1)
		utils.SwapNumericPtr(&i0, &i1)

	}

	// Flip the V to account for inverted UV coordinates, V0 is top left for us
	v0 = 1 - v0
	v1 = 1 - v1
	v2 = 1 - v2

	// Vector points
	pointA := vector.Vec4[T]{T(ix0), T(iy0), T(z0), T(w0)}
	pointB := vector.Vec4[T]{T(ix1), T(iy1), T(z1), T(w1)}
	pointC := vector.Vec4[T]{T(ix2), T(iy2), T(z2), T(w2)}
	AUV := texture.Tex2[T]{u0, v0}
	BUV := texture.Tex2[T]{u1, v1}
	CUV := texture.Tex2[T]{u2, v2}

	// Render the flat-bottom triangle
	// Inverse slope is dX/dy
	var inverseSlope1 T
	var inverseSlope2 T
	if (iy1 - iy0) != 0 {
		inverseSlope1 = T(ix1-ix0) / T(math.Abs(float64(iy1-iy0))) // "Left" leg of the triangle
	}
	if (iy2 - iy0) != 0 {
		inverseSlope2 = T(ix2-ix0) / T(math.Abs(float64(iy2-iy0))) // "Right" leg of the triangle
	}
	// For every scanline
	if iy1-iy0 != 0 {
		for y := iy0; y <= iy1; y++ {
			// The xStart and xEnd come from point-slope formula
			// y-y1 = m(x-x1)
			// (y-y1)/m = x-x1
			// x =  x1 + (y-y1)/m
			xStart := int(T(ix1) + T(y-iy1)*inverseSlope1)
			xEnd := int(T(ix0) + T(y-iy0)*inverseSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTexel(x, y, t, pointA, pointB, pointC, AUV, BUV, CUV, i0, i1, i2)
				//drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
	// Render the flat top triangle
	inverseSlope1, inverseSlope2 = 0.0, 0.0
	if (iy2 - iy1) != 0 {
		inverseSlope1 = T(ix2-ix1) / T(math.Abs(float64(iy2-iy1))) // "Left" leg of the triangle
	}
	if (iy2 - iy0) != 0 {
		inverseSlope2 = T(ix2-ix0) / T(math.Abs(float64(iy2-iy0))) // "Right" leg of the triangle
	}
	// For every scanline
	if iy2-iy1 != 0 {
		for y := iy1; y <= iy2; y++ {
			xStart := int(T(ix1) + T(y-iy1)*inverseSlope1)
			xEnd := int(T(ix0) + T(y-iy0)*inverseSlope2)
			if xEnd < xStart {
				utils.SwapNumericPtr(&xStart, &xEnd)
			}
			for x := xStart; x < xEnd; x++ {
				r.drawTexel(x, y, t, pointA, pointB, pointC, AUV, BUV, CUV, i0, i1, i2)
				//drawPixel(x, y, 0xFFFF00FF)
			}

		}
	}
}

// edgeCross is the edge function: the Z component of the cross product of edge AB
// and AP. It equals twice the signed area of triangle ABP and is positive when P
// lies to the left of the directed edge A->B (for a positive-area winding). It is
// used both as the inside test and as the (unnormalized) barycentric weight.
func edgeCross[T utils.Numeric](a, b, p vector.Vec2[T]) T {
	ab := b.Sub(a)
	ap := p.Sub(a)
	return ab.X*ap.Y - ab.Y*ap.X
}

// isTopLeft reports whether the directed edge start->end is a top edge or a left
// edge. A top edge is perfectly horizontal and points to the right; a left edge
// points upward (its Y delta is negative). This drives the top-left fill rule:
// pixels exactly on a top or left edge belong to this triangle, pixels on the
// other edges belong to the neighbour, so a shared edge is drawn exactly once.
func isTopLeft[T utils.Numeric](start, end vector.Vec2[T]) bool {
	edge := end.Sub(start)
	isTopEdge := edge.Y == 0 && edge.X > 0
	isLeftEdge := edge.Y < 0
	return isTopEdge || isLeftEdge
}

// drawFilledTrianglePineda rasterizes a flat-colored triangle using Juan Pineda's
// algorithm instead of the flat-top/flat-bottom scanline split. We walk the
// triangle's axis aligned bounding box and use three integer edge functions to
// test which pixels fall inside. The edge functions are evaluated once at the
// bounding box corner and then stepped incrementally (the "iteration by w0/w1/w2"):
// a constant delta is added per column and per row, so there are no multiplies in
// the inner loop. A top-left fill rule biases the edges so shared edges between
// adjacent triangles are drawn exactly once.
// Signature matches drawFilledTriangle so it can be swapped in directly.
func (r *Renderer[T]) drawFilledTrianglePineda(x0f, y0f, z0, w0f, x1f, y1f, z1, w1f, x2f, y2f, z2, w2f T, color uint32, i0, i1, i2 T) {
	ix0, iy0 := int(x0f), int(y0f)
	ix1, iy1 := int(x1f), int(y1f)
	ix2, iy2 := int(x2f), int(y2f)

	pointA := vector.Vec4[T]{T(ix0), T(iy0), z0, w0f}
	pointB := vector.Vec4[T]{T(ix1), T(iy1), z1, w1f}
	pointC := vector.Vec4[T]{T(ix2), T(iy2), z2, w2f}

	aVec2 := vector.Vec2[int]{X: ix0, Y: iy0}
	bVec2 := vector.Vec2[int]{X: ix1, Y: iy1}
	cVec2 := vector.Vec2[int]{X: ix2, Y: iy2}

	// Signed area can be used at rasterization step to perform backface culling
	area := edgeCross(aVec2, bVec2, cVec2)
	if area <= 0 {
		return
	}

	// Axis aligned bounding box of the triangle, clamped to the screen bounds
	xMin := max(0, min(ix0, ix1, ix2))
	yMin := max(0, min(iy0, iy1, iy2))
	xMax := min(r.config.WindowWidth-1, max(ix0, ix1, ix2))
	yMax := min(r.config.WindowHeight-1, max(iy0, iy1, iy2))

	// Per-column (moving in +X) and per-row (moving in +Y) deltas for each edge
	// function. These are constant, which is what lets us step instead of recompute.
	deltaW0Col := bVec2.Y - cVec2.Y
	deltaW1Col := cVec2.Y - aVec2.Y
	deltaW2Col := aVec2.Y - bVec2.Y
	deltaW0Row := cVec2.X - bVec2.X
	deltaW1Row := aVec2.X - cVec2.X
	deltaW2Row := bVec2.X - aVec2.X

	// Top-left fill rule: 0 bias keeps edge pixels, -1 bias drops them
	bias0 := -1
	if isTopLeft(bVec2, cVec2) {
		bias0 = 0
	}
	bias1 := -1
	if isTopLeft(cVec2, aVec2) {
		bias1 = 0
	}
	bias2 := -1
	if isTopLeft(aVec2, bVec2) {
		bias2 = 0
	}

	// Seed the edge functions at the top-left corner of the bounding box
	p0 := vector.Vec2[int]{X: xMin, Y: yMin}
	w0Row := edgeCross(bVec2, cVec2, p0) + bias0
	w1Row := edgeCross(cVec2, aVec2, p0) + bias1
	w2Row := edgeCross(aVec2, bVec2, p0) + bias2

	invArea := 1.0 / float64(area)

	for y := yMin; y <= yMax; y++ {
		w0, w1, w2 := w0Row, w1Row, w2Row
		for x := xMin; x <= xMax; x++ {
			// Pixel is inside the triangle when all three edge functions are >= 0
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				// Remove the fill-rule bias before normalizing: the bias is a
				// coverage-only offset, so the barycentric weights used for
				// interpolation must come from the unbiased edge values, otherwise
				// they no longer sum to 1 and depth (especially on small/far
				// triangles, where the bias dominates) comes out wrong.
				alpha := float64(w0-bias0) * invArea
				beta := float64(w1-bias1) * invArea
				gamma := float64(w2-bias2) * invArea

				// Interpolate 1/w to get the per pixel depth, same as the scanline path
				interpolatedReciprocalW := (1/float64(pointA.W))*alpha + (1/float64(pointB.W))*beta + (1/float64(pointC.W))*gamma
				interpolatedReciprocalW = 1 - interpolatedReciprocalW

				if T(interpolatedReciprocalW) < r.zBuffer[(r.config.WindowWidth*y)+x] {
					// Interpolate the per-vertex light intensity (Gouraud); equal
					// vertex values collapse to a constant for flat shading.
					intensity := float64(i0)*alpha + float64(i1)*beta + float64(i2)*gamma
					r.drawPixel(x, y, light.LightApplyIntensity(color, float32(intensity)))
					// Update the Z-buffer with 1/w for current pixel
					r.zBuffer[(r.config.WindowWidth*y)+x] = T(interpolatedReciprocalW)
				}
			}
			// Step the edge functions one column to the right
			w0 += deltaW0Col
			w1 += deltaW1Col
			w2 += deltaW2Col
		}
		// Step the edge functions one row down
		w0Row += deltaW0Row
		w1Row += deltaW1Row
		w2Row += deltaW2Row
	}
}

// drawTexturedTrianglePineda is the texture-mapped counterpart of
// drawFilledTrianglePineda: same bounding-box + incremental edge function
// rasterization with the top-left fill rule, but for every inside pixel it
// perspective-correct samples the texture. Signature matches drawTexturedTriangle
// so it can be swapped in directly.
func (r *Renderer[T]) drawTexturedTrianglePineda(x0, y0, z0, w0f, u0, v0, x1, y1, z1, w1f, u1, v1, x2, y2, z2, w2f, u2, v2, i0, i1, i2 T, t *mesh.TextureData) {
	// Flip the V to account for inverted UV coordinates, V0 is top left for us
	v0 = 1 - v0
	v1 = 1 - v1
	v2 = 1 - v2

	ix0, iy0 := int(x0), int(y0)
	ix1, iy1 := int(x1), int(y1)
	ix2, iy2 := int(x2), int(y2)

	pointA := vector.Vec4[T]{T(ix0), T(iy0), z0, w0f}
	pointB := vector.Vec4[T]{T(ix1), T(iy1), z1, w1f}
	pointC := vector.Vec4[T]{T(ix2), T(iy2), z2, w2f}
	AUV := texture.Tex2[T]{u0, v0}
	BUV := texture.Tex2[T]{u1, v1}
	CUV := texture.Tex2[T]{u2, v2}

	aVec2 := vector.Vec2[int]{X: ix0, Y: iy0}
	bVec2 := vector.Vec2[int]{X: ix1, Y: iy1}
	cVec2 := vector.Vec2[int]{X: ix2, Y: iy2}

	// Signed area can be used to cull the triangles
	area := edgeCross(aVec2, bVec2, cVec2)
	if area <= 0 {
		return // degenerate triangle, nothing to rasterize
	}

	// Axis aligned bounding box of the triangle, clamped to the screen bounds
	xMin := max(0, min(ix0, ix1, ix2))
	yMin := max(0, min(iy0, iy1, iy2))
	xMax := min(r.config.WindowWidth-1, max(ix0, ix1, ix2))
	yMax := min(r.config.WindowHeight-1, max(iy0, iy1, iy2))

	// Constant per-column and per-row deltas for the incremental edge functions
	deltaW0Col := bVec2.Y - cVec2.Y
	deltaW1Col := cVec2.Y - aVec2.Y
	deltaW2Col := aVec2.Y - bVec2.Y
	deltaW0Row := cVec2.X - bVec2.X
	deltaW1Row := aVec2.X - cVec2.X
	deltaW2Row := bVec2.X - aVec2.X

	// Top-left fill rule biases
	bias0 := -1
	if isTopLeft(bVec2, cVec2) {
		bias0 = 0
	}
	bias1 := -1
	if isTopLeft(cVec2, aVec2) {
		bias1 = 0
	}
	bias2 := -1
	if isTopLeft(aVec2, bVec2) {
		bias2 = 0
	}

	// Seed the edge functions at the top-left corner of the bounding box
	p0 := vector.Vec2[int]{X: xMin, Y: yMin}
	w0Row := edgeCross(bVec2, cVec2, p0) + bias0
	w1Row := edgeCross(cVec2, aVec2, p0) + bias1
	w2Row := edgeCross(aVec2, bVec2, p0) + bias2

	invArea := 1.0 / float64(area)

	for y := yMin; y <= yMax; y++ {
		w0, w1, w2 := w0Row, w1Row, w2Row
		for x := xMin; x <= xMax; x++ {
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				// Use the unbiased edge values for interpolation (the fill-rule
				// bias is for coverage only). Biased weights would not sum to 1 and
				// would corrupt depth/UV on small far triangles.
				alpha := float64(w0-bias0) * invArea
				beta := float64(w1-bias1) * invArea
				gamma := float64(w2-bias2) * invArea

				// Perspective correct interpolation of U/w and V/w
				interpolatedU := float64(AUV.U)/float64(pointA.W)*alpha + float64(BUV.U)/float64(pointB.W)*beta + float64(CUV.U)/float64(pointC.W)*gamma
				interpolatedV := float64(AUV.V)/float64(pointA.W)*alpha + float64(BUV.V)/float64(pointB.W)*beta + float64(CUV.V)/float64(pointC.W)*gamma
				interpolatedReciprocalW := 1/float64(pointA.W)*alpha + 1/float64(pointB.W)*beta + 1/float64(pointC.W)*gamma

				// Divide back both U and V by 1/w
				interpolatedU /= interpolatedReciprocalW
				interpolatedV /= interpolatedReciprocalW

				// UV is always between 0 and 1, scale to the texture resolution.
				// Modulo guards against rounding that lands a border pixel just outside.
				texX := int(math.Abs(interpolatedU*float64(t.Width))) % t.Width
				texY := int(math.Abs(interpolatedV*float64(t.Height))) % t.Height

				// Pixels closer to the viewer need to be bigger than the ones further away
				interpolatedReciprocalW = 1 - interpolatedReciprocalW

				if T(interpolatedReciprocalW) < r.zBuffer[(r.config.WindowWidth*y)+x] {
					// Gouraud: interpolate the per-vertex light intensity across the
					// face (affine, in screen space). Flat shading feeds equal values.
					intensity := float64(i0)*alpha + float64(i1)*beta + float64(i2)*gamma
					r.drawPixel(x, y, light.LightApplyIntensity(t.PixelData[(texY*t.Width)+texX], float32(intensity)))
					// Update the Z-buffer with 1/w for current pixel
					r.zBuffer[(r.config.WindowWidth*y)+x] = T(interpolatedReciprocalW)
				}
			}
			// Step the edge functions one column to the right
			w0 += deltaW0Col
			w1 += deltaW1Col
			w2 += deltaW2Col
		}
		// Step the edge functions one row down
		w0Row += deltaW0Row
		w1Row += deltaW1Row
		w2Row += deltaW2Row
	}
}

func (r *Renderer[T]) drawTexel(x, y int, t *mesh.TextureData, a, b, c vector.Vec4[T], AUV, BUV, CUV texture.Tex2[T], i0, i1, i2 T) {
	if x < 0 || x >= r.config.WindowWidth || y < 0 || y >= r.config.WindowHeight {
		return
	}
	p := vector.Vec2[T]{X: T(x), Y: T(y)}
	aVec2 := a.ToVec2()
	bVec2 := b.ToVec2()
	cVec2 := c.ToVec2()

	weights := barycentricWeights(aVec2, bVec2, cVec2, p)
	alpha, beta, gamma := weights.X, weights.Y, weights.Z
	// Perform the interpolation of all the U/w and V/w values using barycentric weights and a factor of 1/w
	interpolatedU := (AUV.U/a.W)*alpha + (BUV.U/b.W)*beta + (CUV.U/c.W)*gamma
	interpolatedV := (AUV.V/a.W)*alpha + (BUV.V/b.W)*beta + (CUV.V/c.W)*gamma
	// This is 1/w because W (original Z) is not linear
	interpolatedReciprocalW := (1/a.W)*alpha + (1/b.W)*beta + (1/c.W)*gamma

	// Divide back both U and V by 1/w
	interpolatedU /= interpolatedReciprocalW
	interpolatedV /= interpolatedReciprocalW
	// UV is always between 0 and 1 so we need to scale them to texture resolution
	// The overflows here come from the fact that we truncate to (integer) pixels
	// On the border of a triangle a pixel might be outside a triangle which throws the barycentric weights (become bigger than 1)
	// Which then tries to fetch texel outside of bounds of texture array
	texX := int(math.Abs(float64(interpolatedU)*float64(t.Width))) % t.Width
	texY := int(math.Abs(float64(interpolatedV)*float64(t.Height))) % t.Height

	// Pixels closer to the viewer need to bigger than the ones further away
	interpolatedReciprocalW = 1 - interpolatedReciprocalW

	// Only draw pixels if depth value if less than the one previously stored in the Z-buffer
	if interpolatedReciprocalW < r.zBuffer[(r.config.WindowWidth*y)+x] {
		// Interpolate the per-vertex light intensity across the face (Gouraud);
		// flat shading passes equal vertex values so this is a constant.
		intensity := i0*alpha + i1*beta + i2*gamma
		r.drawPixel(x, y, light.LightApplyIntensity(t.PixelData[(texY*t.Width)+texX], float32(intensity)))
		// Update the Z-buffer with 1/w for current pixel
		r.zBuffer[(r.config.WindowWidth*y)+x] = interpolatedReciprocalW

	}
}
