package city

import (
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
)

type drawnFace struct {
	points [3]geometry.Vertex
	paint  color.NRGBA
	depth  float64
}

type faceMaterial struct {
	normal geometry.Vec3
	paint  color.NRGBA
}

func cacheMaterials(mesh effects.Mesh) []faceMaterial {
	cache := make([]faceMaterial, len(mesh.Triangles))
	light := geometry.Vec3{X: -.35, Y: -.8, Z: -.5}.Unit()
	for i, face := range mesh.Triangles {
		a, b, c := mesh.Points[face.Indices[0]], mesh.Points[face.Indices[1]], mesh.Points[face.Indices[2]]
		n := b.Sub(a).Cross(c.Sub(a)).Unit()
		factor := .57 + .43*max(0, n.Dot(light))
		paint := face.Color
		paint.R, paint.G, paint.B = uint8(float64(paint.R)*factor), uint8(float64(paint.G)*factor), uint8(float64(paint.B)*factor)
		cache[i] = faceMaterial{normal: n, paint: paint}
	}
	return cache
}

// renderer keeps DCK projection, near clipping and mesh topology. A retained
// triangle stream avoids per-face color interfaces and mobile draw allocations.
type renderer struct {
	white    *ebiten.Image
	faces    []drawnFace
	order    []int
	points   []geometry.Vec3
	vertices []ebiten.Vertex
	indices  []uint32
	view     view
}

func newRenderer() *renderer {
	r := &renderer{white: ebiten.NewImage(1, 1), faces: make([]drawnFace, 0, 18000), order: make([]int, 0, 18000), points: make([]geometry.Vec3, 0, 65536), vertices: make([]ebiten.Vertex, 0, 60000)}
	r.white.Fill(color.White)
	r.indices = make([]uint32, 100000)
	for i := range r.indices {
		r.indices[i] = uint32(i)
	}
	return r
}

func (r *renderer) begin(v view) { r.view, r.faces = v, r.faces[:0] }

// A conservative bounding sphere rejects whole buildings before transforming
// their vertices. Near intersections stay visible and use DCK clipping below.
func (r *renderer) visible(center geometry.Vec3, radius float64) bool {
	p := r.view.point(center)
	if p.Z+radius < r.view.camera.Near || p.Z-radius > 1000 {
		return false
	}
	if p.Z <= radius+r.view.camera.Near {
		return true
	}
	xy, _, _ := r.view.camera.Project(p)
	margin := radius * r.view.camera.Focal / (p.Z - radius)
	return xy.X+margin >= 0 && xy.X-margin <= Width && xy.Y+margin >= 0 && xy.Y-margin <= Height
}

func (r *renderer) mesh(m effects.Mesh, rotation geometry.Rotation, position geometry.Vec3, cull bool, opacity float64, materials []faceMaterial) {
	if opacity <= 0 {
		return
	}
	if cap(r.points) < len(m.Points) {
		r.points = make([]geometry.Vec3, len(m.Points))
	} else {
		r.points = r.points[:len(m.Points)]
	}
	for i, point := range m.Points {
		if materials == nil {
			point = rotation.Apply(point).Add(position)
		}
		r.points[i] = r.view.point(point)
	}
	left, right := r.view.camera.Center.X/r.view.camera.Focal, (Width-r.view.camera.Center.X)/r.view.camera.Focal
	top, bottom := r.view.camera.Center.Y/r.view.camera.Focal, (Height-r.view.camera.Center.Y)/r.view.camera.Focal
	light := geometry.Vec3{X: -.35, Y: -.8, Z: -.5}.Unit()
	for index, face := range m.Triangles {
		if materials != nil && cull && materials[index].normal.Dot(m.Points[face.Indices[0]].Sub(r.view.eye)) >= 0 {
			continue
		}
		a, b, c := r.points[face.Indices[0]], r.points[face.Indices[1]], r.points[face.Indices[2]]
		if max(a.Z, b.Z, c.Z) < r.view.camera.Near || min(a.Z, b.Z, c.Z) > 1000 {
			continue
		}
		// Reject entire distant faces outside the frustum before sorting.
		if min(a.X-a.Z*right, b.X-b.Z*right, c.X-c.Z*right) > 0 || max(a.X+a.Z*left, b.X+b.Z*left, c.X+c.Z*left) < 0 ||
			min(a.Y-a.Z*bottom, b.Y-b.Z*bottom, c.Y-c.Z*bottom) > 0 || max(a.Y+a.Z*top, b.Y+b.Z*top, c.Y+c.Z*top) < 0 {
			continue
		}
		illumination := 1.0
		paint := face.Color
		if materials != nil {
			paint = materials[index].paint
		} else {
			n := b.Sub(a).Cross(c.Sub(a)).Unit()
			if cull && n.Dot(a) >= 0 {
				continue
			}
			worldNormal := geometry.Vec3{X: n.Dot(geometry.Vec3{X: r.view.right.X, Y: r.view.down.X, Z: r.view.forward.X}),
				Y: n.Dot(geometry.Vec3{X: r.view.right.Y, Y: r.view.down.Y, Z: r.view.forward.Y}),
				Z: n.Dot(geometry.Vec3{X: r.view.right.Z, Y: r.view.down.Z, Z: r.view.forward.Z})}
			illumination = .57 + .43*max(0, worldNormal.Dot(light))
		}
		depth := (a.Z + b.Z + c.Z) / 3
		fog := ease(170, 950, depth) * .82
		paint.A = uint8(float64(paint.A) * opacity)
		paint.R = uint8(mix(float64(paint.R)*illumination, 23, fog))
		paint.G = uint8(mix(float64(paint.G)*illumination, 50, fog))
		paint.B = uint8(mix(float64(paint.B)*illumination, 74, fog))
		r.faces = append(r.faces, drawnFace{points: [3]geometry.Vertex{{Position: a}, {Position: b}, {Position: c}}, paint: paint, depth: depth})
	}
}

func (r *renderer) triangle(a, b, c geometry.Vec3, paint color.NRGBA) {
	a, b, c = r.view.point(a), r.view.point(b), r.view.point(c)
	r.faces = append(r.faces, drawnFace{points: [3]geometry.Vertex{{Position: a}, {Position: b}, {Position: c}}, paint: paint, depth: (a.Z + b.Z + c.Z) / 3})
}

func (r *renderer) draw(dst *ebiten.Image) {
	r.order = r.order[:0]
	for i := range r.faces {
		r.order = append(r.order, i)
	}
	// Sort compact indices instead of repeatedly copying 136-byte face records.
	slices.SortFunc(r.order, func(a, b int) int {
		if r.faces[a].depth > r.faces[b].depth {
			return -1
		}
		if r.faces[a].depth < r.faces[b].depth {
			return 1
		}
		return 0
	})
	r.vertices = r.vertices[:0]
	for _, index := range r.order {
		face := r.faces[index]
		var scratch [4]geometry.Vertex
		polygon := geometry.ClipNear(scratch[:0], face.points, r.view.camera.Near)
		red, green, blue, alpha := face.paint.RGBA()
		base := ebiten.Vertex{ColorR: float32(red) / 65535, ColorG: float32(green) / 65535, ColorB: float32(blue) / 65535, ColorA: float32(alpha) / 65535}
		for j := 1; j+1 < len(polygon); j++ {
			for _, corner := range [3]geometry.Vertex{polygon[0], polygon[j], polygon[j+1]} {
				point, _, _ := r.view.camera.Project(corner.Position)
				v := base
				v.DstX, v.DstY = float32(point.X), float32(point.Y)
				r.vertices = append(r.vertices, v)
			}
		}
	}
	if len(r.vertices) > len(r.indices) {
		r.indices = make([]uint32, len(r.vertices))
		for i := range r.indices {
			r.indices[i] = uint32(i)
		}
	}
	dst.DrawTriangles32(r.vertices, r.indices[:len(r.vertices)], r.white, &ebiten.DrawTrianglesOptions{ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha})
}

func (r *renderer) shadow(car carPose) {
	if car.position.Y < -25 {
		return
	}
	center := car.position
	center.Y = -.07
	rotation := geometry.RotateXYZ(geometry.Vec3{Y: car.rotation.Y})
	for i := 0; i < 16; i++ {
		point := func(angle float64) geometry.Vec3 {
			return rotation.Apply(geometry.Vec3{X: 1.05 * math.Cos(angle), Z: 2.30 * math.Sin(angle)}).Add(center)
		}
		r.triangle(center, point(float64(i)*math.Pi/8), point(float64(i+1)*math.Pi/8), color.NRGBA{A: 105})
	}
}

func (r *renderer) thrusters(car carPose, seconds float64) {
	if seconds < LaunchAt-.7 {
		return
	}
	rotation := geometry.RotateXYZ(car.rotation)
	for _, x := range []float64{-thrusterX, thrusterX} {
		length := (2.8 + .7*math.Sin(seconds*39+x)) * ease(LaunchAt-.7, LaunchAt+.6, seconds)
		world := func(p geometry.Vec3) geometry.Vec3 { return rotation.Apply(p).Add(car.position) }
		for axis := 0; axis < 2; axis++ {
			a, b := geometry.Vec3{X: x - .14, Y: thrusterY, Z: thrusterZ}, geometry.Vec3{X: x + .14, Y: thrusterY, Z: thrusterZ}
			if axis == 1 {
				a, b = geometry.Vec3{X: x, Y: thrusterY - .14, Z: thrusterZ}, geometry.Vec3{X: x, Y: thrusterY + .14, Z: thrusterZ}
			}
			r.triangle(world(a), world(b), world(geometry.Vec3{X: x, Y: thrusterY, Z: thrusterZ - length}), color.NRGBA{R: 74, G: 195, B: 255, A: 190})
		}
		for _, z := range []float64{-1.4, 1.4} {
			r.triangle(world(geometry.Vec3{X: x - .13, Y: -.28, Z: z}), world(geometry.Vec3{X: x + .13, Y: -.28, Z: z}), world(geometry.Vec3{X: x, Y: length * 1.3, Z: z - length*.3}), color.NRGBA{R: 153, G: 226, B: 255, A: 170})
		}
	}
}

func (r *renderer) close() { r.white.Deallocate() }
