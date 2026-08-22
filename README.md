# Software Renderer

[![Live Demo](https://img.shields.io/badge/Live%20Demo-blazskufca.com-blue)](https://blazskufca.com/projects/software_renderer/)

A 3D software renderer written in Go from scratch. Every pixel is computed on the
CPU into a flat color buffer; no GPU rasterization is used. [Ebiten](https://ebiten.org/)
is used only to open a window, blit the finished color buffer to the screen, and
handle input, so the renderer itself stays a pure software pipeline that also
compiles to WebAssembly and runs in the browser.

This project follows the [Pikuma "3D Computer Graphics Programming" course](https://pikuma.com/courses/learn-3d-computer-graphics-programming),
reimplemented in idiomatic, generic Go rather than the course's C.
<img width="960" height="480" alt="screenrecording-2026-06-23_10-25-52" src="https://github.com/user-attachments/assets/f2ac3244-18ac-4e74-857b-e7414a949c4a" />


## Features

- Wavefront `.obj` mesh loading with PNG/JPG diffuse textures
- Perspective projection with a configurable field of view
- Matrix transform pipeline (scale, rotate, translate, view/look-at)
- Back-face culling
- Frustum clipping against all six planes
- Flat shading from a directional light
- Z-buffering for correct depth ordering
- Three rasterizer backends, selectable at runtime:
  - Flat-bottom/flat-top scanline rasterization
  - Pineda (edge-function / barycentric) rasterization
  - SIMD Pineda, shading eight pixels at a time (see [SIMD rasterizers](#simd-rasterizers))
- Perspective-correct texture mapping
- A free-look FPS camera (keyboard, mouse, and on-screen touch controls)
- Native desktop and WebAssembly browser targets from a single codebase
- Generic numeric core (`float32`/`float64`) via Go generics

## SIMD rasterizers

The two Pineda rasterizers have vector rewrites in `cmd/renderer/raster_simd.go`,
built on Go 1.27's experimental [`simd`](https://pkg.go.dev/simd) package. They
shade a whole vector of pixels per iteration — eight on AVX2, four or sixteen
elsewhere — and produce output that is bit-identical to the scalar versions on
the shipped model.

Two changes were needed to make vectorizing pay off, and both are worth more than
the vector arithmetic itself:

- **Exact span clipping.** A scalar rasterizer rejects an uncovered pixel with
  three compares, which is cheap enough that a vector loop walking the same
  bounding box loses outright. Each scanline is instead narrowed to the exact
  interval where all three edge functions are non-negative, so every lane the
  loop touches is covered.
- **No scalar floating point in the inner loop.** Go compiles scalar float code
  to legacy SSE, and a single legacy SSE instruction inside a 256-bit loop
  triggers the AVX/SSE transition penalty — measured at 15x on Zen 3. Each
  rasterizer is therefore split into a scalar setup function and a pure-vector
  one, and the interpolated attributes are carried as vector accumulators.

Vectorizing trades a fixed per-triangle setup cost for cheaper per-pixel work, so
it only pays above a triangle size. Below that the rasterizers hand the triangle
back to the scalar path, which costs a few percent for the dispatch check — worth
it, because without the cutoff the textured vector path was 1.6x *slower* than
the scalar one on `race.obj`. Measured on a Ryzen 5 5600X (`make bench`), same
total pixel coverage at every triangle size:

| Workload | Pineda | SIMD Pineda | Speedup |
| --- | --- | --- | --- |
| Fill, 96px triangles | 0.17 ms | 0.08 ms | 2.2x |
| Fill, 48px triangles | 0.18 ms | 0.11 ms | 1.7x |
| Fill, 16px triangles | 0.26 ms | 0.26 ms | — (below cutoff) |
| Textured, 96px triangles | 0.37 ms | 0.16 ms | 2.3x |
| Textured, 48px triangles | 0.35 ms | 0.23 ms | 1.5x |
| Textured, 24px triangles | 0.40 ms | 0.40 ms | — (below cutoff) |
| Fill, one frame of `race.obj` | 0.38 ms | 0.33 ms | 1.13x |
| Textured, one frame of `race.obj` | 0.58 ms | 0.53 ms | 1.10x |

A whole `race.obj` frame gains least because most of its triangles are small; the
win grows with triangle size, so it is largest exactly when the frame is most
expensive. `BenchmarkTriangleSizeSweep` produces the size table and is the tool
for retuning the cutoffs on other hardware.

Building without `GOEXPERIMENT=simd` is fully supported: `raster_simd_stub.go`
takes over, the SIMD toolbar buttons and key bindings disappear, and everything
else is unchanged.

## Requirements

- Go 1.26 or newer (Go 1.27 with `GOEXPERIMENT=simd`, which the Makefile sets,
  for the SIMD rasterizers)
- Native builds need the platform dependencies for Ebiten (on Linux, the usual
  X11/OpenGL development headers). See the
  [Ebiten install guide](https://ebiten.org/documents/install.html).
- WebAssembly build/serve needs Python 3 (for the bundled dev server)

## Running (desktop)

```sh
make run
```

or directly:

```sh
go run ./cmd/renderer
```

To build a standalone binary:

```sh
make build      # produces ./renderer
```

## Running (WebAssembly)

Build the wasm bundle and serve it (workers and wasm require `http://`, not
`file://`):

```sh
make serve
```

Then open http://localhost:8080.

To only build the bundle into `web/` without serving:

```sh
make web
```

## Controls

Camera:

| Key            | Action          |
| -------------- | --------------- |
| `W` / `S`      | Move forward / back |
| `A` / `D`      | Strafe left / right |
| Up / Down      | Move up / down  |
| Mouse          | Look (click to capture the cursor, `Esc` to release) |

On-screen buttons provide the same movement on touch devices.

Render mode:

| Key | Mode                          |
| --- | ----------------------------- |
| `1` | Wireframe with vertices       |
| `2` | Wireframe                     |
| `3` | Filled triangles              |
| `4` | Filled triangles + wireframe  |
| `5` | Textured                      |
| `6` | Textured + wireframe          |

Rasterizer / culling:

| Key | Action                          |
| --- | ------------------------------- |
| `7` | Fill rasterizer: scanline       |
| `8` | Fill rasterizer: Pineda         |
| `9` | Texture rasterizer: scanline    |
| `0` | Texture rasterizer: Pineda      |
| `-` | Fill rasterizer: Pineda + SIMD  |
| `=` | Texture rasterizer: Pineda + SIMD |
| `C` | Enable back-face culling        |
| `X` | Disable back-face culling       |
| `F` | Flat shading                    |
| `G` | Gouraud shading                 |

The two SIMD entries only appear on a build that has them (see above). Note that
the fill and texture rasterizers are selected independently, and only one of them
runs: modes `3`/`4` use the fill rasterizer, modes `5`/`6` use the texture one.

The render-mode toolbar at the top of the window exposes the same toggles.

## Project layout

```
cmd/renderer/      Entry point, main loop, rasterizers, UI, display
internal/
  vector/          Vec2/Vec3/Vec4 math
  matrix/          Mat4 transforms and projection
  camera/          FPS camera state
  light/           Directional light and intensity
  clipping/        Frustum plane setup and polygon clipping
  texture/          Texture loading and texel sampling
  face/            Mesh and .obj parsing
  config/          Render/cull/rasterizer mode enums
  assets/          Asset loading (disk on native, embedded on wasm)
  utils/           Numeric generic constraint and helpers
assets/            Models (.obj) and textures
web/               WebAssembly host page, worker, and JS glue
```

## Credits

- Based on the [Pikuma "3D Computer Graphics Programming" course](https://pikuma.com/courses/learn-3d-computer-graphics-programming) by Gustavo Pezzi.
- Some 3D models are from [Kenney](https://kenney.nl) (CC0).
</content>
</invoke>
