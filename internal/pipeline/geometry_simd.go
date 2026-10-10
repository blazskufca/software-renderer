//go:build simd && goexperiment.simd

package pipeline

import (
	"simd"
	"slices"

	"github.com/blazskufca/software-renderer/internal/config"
	mesh "github.com/blazskufca/software-renderer/internal/face"
	"github.com/blazskufca/software-renderer/internal/matrix"
	"github.com/blazskufca/software-renderer/internal/utils"
	"github.com/blazskufca/software-renderer/internal/vector"
)

// transformVerticesSIMD does the per-vertex stage for float32 meshes a vector
// of vertices at a time and returns how many vertices it handled; the scalar
// loop finishes the rest. Results are bit-identical to the scalar loop: same
// operations in the same order, and no fused multiply-add.
func transformVerticesSIMD[T utils.Numeric](vd *vertexData[T], in []vector.Vec3[T], world *matrix.Mat4[T], p *Params[T]) int {
	v, ok := any(vd).(*vertexData[float32])
	if !ok || simd.Emulated() || scalarOnly {
		return 0
	}
	// Inside this generic function the simd calls compiled to real function
	// calls instead of instructions (10x slower), so the loop lives in a plain
	// float32 function.
	return transformVertices32(v, any(in).([]vector.Vec3[float32]), any(world).(*matrix.Mat4[float32]), any(p).(*Params[float32]))
}

// Indexes into vertexData.consts, one vector per constant.
const (
	cWorld  = 0            // 12: world rows 0-2
	cProj   = cWorld + 12  // 12: projection rows 0, 1 and 3
	cPlanes = cProj + 12   // 36: per plane, point then normal
	cHalf   = cPlanes + 36 // 2: half width, half height
	cZero   = cHalf + 2    // 1
	nConsts = cZero + 1
)

func transformVertices32(v *vertexData[float32], verts []vector.Vec3[float32], world *matrix.Mat4[float32], p *Params[float32]) int {
	var probe simd.Float32s
	n := probe.Len()
	count := len(verts) - len(verts)%n

	// All scalar work happens before the first vector op: scalar float code
	// after it would run legacy SSE instructions against dirty upper halves
	// (see clearUpperBits). That includes the constants, which are broadcast
	// into memory here and loaded as vectors in the loop.
	for _, s := range []*[]float32{&v.mx, &v.my, &v.mz} {
		*s = slices.Grow((*s)[:0], count)[:count]
	}
	for i := range count {
		v.mx[i], v.my[i], v.mz[i] = verts[i].X, verts[i].Y, verts[i].Z
	}
	v.consts = slices.Grow(v.consts[:0], nConsts*n)[:nConsts*n]
	set := func(k int, f float32) {
		for i := range n {
			v.consts[k*n+i] = f
		}
	}
	for r := range 3 {
		for c := range 4 {
			set(cWorld+r*4+c, world.At(r, c))
			set(cProj+r*4+c, p.Projection.At([3]int{0, 1, 3}[r], c))
		}
	}
	for k := range p.Frustum {
		pt, nm := p.Frustum[k].Point(), p.Frustum[k].Normal()
		for j, f := range [6]float32{pt.X, pt.Y, pt.Z, nm.X, nm.Y, nm.Z} {
			set(cPlanes+k*6+j, f)
		}
	}
	set(cHalf, float32(p.Width/2))
	set(cHalf+1, float32(p.Height/2))
	set(cZero, 0)
	c := func(k int) simd.Float32s { return simd.LoadFloat32s(v.consts[k*n : k*n+n]) }

	for i := 0; i < count; i += n {
		mx, my, mz := simd.LoadFloat32s(v.mx[i:i+n]), simd.LoadFloat32s(v.my[i:i+n]), simd.LoadFloat32s(v.mz[i:i+n])
		x := c(cWorld + 0).Mul(mx).Add(c(cWorld + 1).Mul(my)).Add(c(cWorld + 2).Mul(mz)).Add(c(cWorld + 3))
		y := c(cWorld + 4).Mul(mx).Add(c(cWorld + 5).Mul(my)).Add(c(cWorld + 6).Mul(mz)).Add(c(cWorld + 7))
		z := c(cWorld + 8).Mul(mx).Add(c(cWorld + 9).Mul(my)).Add(c(cWorld + 10).Mul(mz)).Add(c(cWorld + 11))
		x.Store(v.x[i : i+n])
		y.Store(v.y[i : i+n])
		z.Store(v.z[i : i+n])

		zero := c(cZero)
		inside := zero.Equal(zero)
		for k := range 6 {
			b := cPlanes + k*6
			d := x.Sub(c(b + 0)).Mul(c(b + 3)).
				Add(y.Sub(c(b + 1)).Mul(c(b + 4))).
				Add(z.Sub(c(b + 2)).Mul(c(b + 5)))
			inside = inside.And(d.Greater(zero))
		}
		inside.ToInt32s().Store(v.inside[i : i+n])

		// project: Mat4MulVec4Project with W = 1, then the viewport transform
		px := c(cProj + 0).Mul(x).Add(c(cProj + 1).Mul(y)).Add(c(cProj + 2).Mul(z)).Add(c(cProj + 3))
		py := c(cProj + 4).Mul(x).Add(c(cProj + 5).Mul(y)).Add(c(cProj + 6).Mul(z)).Add(c(cProj + 7))
		pw := c(cProj + 8).Mul(x).Add(c(cProj + 9).Mul(y)).Add(c(cProj + 10).Mul(z)).Add(c(cProj + 11))
		nonZero := pw.NotEqual(zero)
		px = px.Div(pw).IfElse(nonZero, px)
		py = py.Div(pw).IfElse(nonZero, py)
		hw, hh := c(cHalf), c(cHalf+1)
		px.Mul(hw).Add(hw).Store(v.sx[i : i+n])
		py.Mul(hh).Neg().Add(hh).Store(v.sy[i : i+n])
		pw.Store(v.sw[i : i+n])
	}
	clearUpperBits()
	return count
}

// shadeFacesSIMD is shadeFace for float32 meshes, a vector of faces at a time.
// It returns how many faces it handled; shadeFace does the rest.
func shadeFacesSIMD[T utils.Numeric](fd *faceData[T], faces []mesh.Face[T], vd *vertexData[T], world *matrix.Mat4[T], p *Params[T]) int {
	f, ok := any(fd).(*faceData[float32])
	if !ok || simd.Emulated() || scalarOnly {
		return 0
	}
	return shadeFaces32(f, any(faces).([]mesh.Face[float32]), any(vd).(*vertexData[float32]), any(world).(*matrix.Mat4[float32]), any(p).(*Params[float32]))
}

// Indexes into faceData.gather.
const (
	gA      = 0 // corner A x, y, z; B and C follow
	gNormal = 9 // vertex normal A x, y, z; B and C follow
)

// Indexes into vertexData.consts, which shadeFaces32 reuses.
const (
	cRot        = 0        // 9: world rows 0-2, columns 0-2
	cLight      = cRot + 9 // 3
	cFZero      = cLight + 3
	nFaceConsts = cFZero + 1
)

func shadeFaces32(fd *faceData[float32], faces []mesh.Face[float32], vd *vertexData[float32], world *matrix.Mat4[float32], p *Params[float32]) int {
	var probe simd.Float32s
	n := probe.Len()
	count := len(faces) - len(faces)%n
	cull := p.Cull == config.CULL_BACKFACE
	gouraud := p.Shading == config.SHADING_GOURAUD

	// Scalar gather first, before any vector op (see transformVertices32).
	// Unrolled, with count capped so the block indexes need no bounds checks:
	// this loop costs more than the vector math.
	g := &fd.gather
	x, y, z := vd.x, vd.y, vd.z
	count = min(count, faceBlock)
	for j := range count {
		f := &faces[j]
		a, b, c := f.A, f.B, f.C
		g[gA+0][j], g[gA+1][j], g[gA+2][j] = x[a], y[a], z[a]
		g[gA+3][j], g[gA+4][j], g[gA+5][j] = x[b], y[b], z[b]
		g[gA+6][j], g[gA+7][j], g[gA+8][j] = x[c], y[c], z[c]
	}
	if gouraud {
		for j := range count {
			f := &faces[j]
			g[gNormal+0][j], g[gNormal+1][j], g[gNormal+2][j] = f.ANormal.X, f.ANormal.Y, f.ANormal.Z
			g[gNormal+3][j], g[gNormal+4][j], g[gNormal+5][j] = f.BNormal.X, f.BNormal.Y, f.BNormal.Z
			g[gNormal+6][j], g[gNormal+7][j], g[gNormal+8][j] = f.CNormal.X, f.CNormal.Y, f.CNormal.Z
		}
	}
	vd.consts = slices.Grow(vd.consts[:0], nFaceConsts*n)[:nFaceConsts*n]
	set := func(k int, f float32) {
		for i := range n {
			vd.consts[k*n+i] = f
		}
	}
	for r := range 3 {
		for c := range 3 {
			set(cRot+r*3+c, world.At(r, c))
		}
	}
	set(cLight, p.Light.Direction.X)
	set(cLight+1, p.Light.Direction.Y)
	set(cLight+2, p.Light.Direction.Z)
	set(cFZero, 0)
	c := func(k int) simd.Float32s { return simd.LoadFloat32s(vd.consts[k*n : k*n+n]) }
	ld := func(k, j int) simd.Float32s { return simd.LoadFloat32s(g[k][j : j+n]) }

	for j := 0; j < count; j += n {
		zero := c(cFZero)
		ax, ay, az := ld(gA, j), ld(gA+1, j), ld(gA+2, j)
		ex, ey, ez := ld(gA+3, j).Sub(ax), ld(gA+4, j).Sub(ay), ld(gA+5, j).Sub(az)
		fx, fy, fz := ld(gA+6, j).Sub(ax), ld(gA+7, j).Sub(ay), ld(gA+8, j).Sub(az)
		nx := ey.Mul(fz).Sub(ez.Mul(fy))
		ny := ez.Mul(fx).Sub(ex.Mul(fz))
		nz := ex.Mul(fy).Sub(ey.Mul(fx))
		keep := zero.Equal(zero)
		if cull {
			facing := nx.Mul(zero.Sub(ax)).Add(ny.Mul(zero.Sub(ay))).Add(nz.Mul(zero.Sub(az)))
			keep = facing.GreaterEqual(zero).Or(facing.NotEqual(facing)) // !(facing < 0), NaN included
		}
		keep.ToInt32s().Store(fd.keep[j : j+n])

		l := nx.Mul(nx).Add(ny.Mul(ny)).Add(nz.Mul(nz)).Sqrt()
		flat := nx.Div(l).Mul(c(cLight)).Add(ny.Div(l).Mul(c(cLight + 1))).Add(nz.Div(l).Mul(c(cLight + 2))).Neg()
		if !gouraud {
			for k := range 3 {
				flat.Store(fd.light[k][j : j+n])
			}
			continue
		}
		for k := range 3 {
			vx, vy, vz := ld(gNormal+k*3, j), ld(gNormal+k*3+1, j), ld(gNormal+k*3+2, j)
			tx := c(cRot + 0).Mul(vx).Add(c(cRot + 1).Mul(vy)).Add(c(cRot + 2).Mul(vz))
			ty := c(cRot + 3).Mul(vx).Add(c(cRot + 4).Mul(vy)).Add(c(cRot + 5).Mul(vz))
			tz := c(cRot + 6).Mul(vx).Add(c(cRot + 7).Mul(vy)).Add(c(cRot + 8).Mul(vz))
			tl := tx.Mul(tx).Add(ty.Mul(ty)).Add(tz.Mul(tz)).Sqrt()
			lit := tx.Mul(c(cLight)).Add(ty.Mul(c(cLight + 1))).Add(tz.Mul(c(cLight + 2))).Neg().Div(tl)
			lit.IfElse(tl.NotEqual(zero), flat).Store(fd.light[k][j : j+n])
		}
	}
	clearUpperBits()
	return count
}
