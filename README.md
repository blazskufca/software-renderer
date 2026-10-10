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
- Two rasterizer backends, selectable at runtime:
  - Flat-bottom/flat-top scanline rasterization
  - Pineda (edge-function / barycentric) rasterization
- Perspective-correct texture mapping
- A free-look FPS camera (keyboard, mouse, and on-screen touch controls)
- Native desktop and WebAssembly browser targets from a single codebase
- Generic numeric core (`float32`/`float64`) via Go generics

## Requirements

- Go 1.26 or newer
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

The SIMD span rasterizer (`internal/raster`, `internal/pipeline`) is opt-in.
`SIMD=1` works with every make target above and below (needs Go 1.27+):

```sh
make run SIMD=1                                     # -tags simd + GOEXPERIMENT=simd
GOEXPERIMENT=simd go run -tags simd ./cmd/renderer  # same, directly
go run -tags simd ./cmd/renderer                    # new rasterizer, scalar kernels
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

## Testing

```sh
make test       # original renderer, new rasterizer with scalar kernels, with SIMD kernels
make test-wasm  # internal/... under js/wasm in Node.js, scalar and SIMD128 kernels
make lint       # gofmt, then go vet for every native and wasm build
make cover      # statement coverage of all three `make test` builds, merged
```

These targets cover every build mode themselves, so `SIMD=1` is not needed.
`make cover` writes `coverage/coverage.out`; view it with
`go tool cover -html=coverage/coverage.out`.

CI (`.github/workflows/ci.yml`) runs them, plus the native and wasm builds, on
every pull request and push to `master`.

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
| `C` | Enable back-face culling        |
| `X` | Disable back-face culling       |

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
