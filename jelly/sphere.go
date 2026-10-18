package jelly

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/sprites"
)

type jellySphere struct {
	front, back          *meshBatch
	base, viewed         []geometry.Vec3
	meshCount            int
	field                *sprites.Field
	points               *render.Batch
	rearDots, frontDots  []ebiten.Vertex
	seconds, centerDepth float64
}

func newJellySphere() (_ *jellySphere, err error) {
	p := &jellySphere{}
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	solid, err := effects.SolidSphereModel(effects.SolidSphereConfig{Radius: sphereRadius, Rows: 12, Columns: 24})
	if err != nil {
		return nil, err
	}
	mesh := effects.Mesh{}
	slots := make(map[[3]int64]int)
	pointIndex := func(point geometry.Vec3) int {
		key := [3]int64{int64(math.Round(point.X * 1e9)), int64(math.Round(point.Y * 1e9)), int64(math.Round(point.Z * 1e9))}
		if index, found := slots[key]; found {
			return index
		}
		index := len(mesh.Points)
		mesh.Points = append(mesh.Points, point)
		slots[key] = index
		return index
	}
	for _, group := range solid.Groups {
		for _, quad := range group {
			q := quad.Indices
			for i, index := range q {
				q[i] = pointIndex(solid.Points[index])
			}
			for _, ids := range [][3]int{{q[0], q[1], q[2]}, {q[0], q[2], q[3]}} {
				a, b, c := mesh.Points[ids[0]], mesh.Points[ids[1]], mesh.Points[ids[2]]
				n := b.Sub(a).Cross(c.Sub(a))
				if n.Dot(n) < 1e-10 {
					continue
				}
				if n.Dot(a) < 0 {
					ids[1], ids[2] = ids[2], ids[1]
				}
				mesh.Triangles = append(mesh.Triangles, effects.Triangle{Indices: ids, Color: color.NRGBA{R: 255, G: 150, B: 65}})
			}
		}
	}
	p.front, err = newMeshBatch(mesh, nil, viewAt(0).camera)
	if err != nil {
		return nil, err
	}
	for i := range mesh.Triangles {
		face := &mesh.Triangles[i]
		face.Indices[1], face.Indices[2] = face.Indices[2], face.Indices[1]
	}
	p.back, err = newMeshBatch(mesh, nil, viewAt(0).camera)
	if err != nil {
		return nil, err
	}
	p.meshCount = len(mesh.Points)
	dots, err := geometry.SphereCloud(geometry.SphereCloudConfig{Count: 1024, Radius: sphereRadius, Sampling: geometry.SphereFibonacci})
	if err != nil {
		return nil, err
	}
	p.base = append(append([]geometry.Vec3(nil), mesh.Points...), dots...)
	p.viewed = make([]geometry.Vec3, len(p.base))
	for _, renderer := range []*meshBatch{p.front, p.back} {
		renderer.CullBackFaces = true
		renderer.DepthSort = true
		renderer.Deform = func(index int, _ geometry.Vec3, _ float64) geometry.Vec3 { return p.viewed[index] }
	}
	p.field, err = sprites.NewField(sprites.FieldConfig{Count: len(dots), Spawn: func(index int, _ bool) sprites.Point {
		point := p.viewed[p.meshCount+index]
		return sprites.Point{X: point.X, Y: point.Y, Z: point.Z}
	}})
	if err != nil {
		return nil, err
	}
	p.points = render.NewBatch(len(dots) * 2)
	p.points.Options.Blend = ebiten.BlendLighter
	p.rearDots, p.frontDots = make([]ebiten.Vertex, 0, len(dots)*4), make([]ebiten.Vertex, 0, len(dots)*4)
	return p, nil
}

func (p *jellySphere) update(f kit.Frame, view sceneView, centerDepth float64) error {
	p.seconds, p.centerDepth = f.Time, centerDepth
	skin := ramp(SphereJellyAt, SphereJellyAt+1.5, f.Time)
	for i := range p.front.Mesh.Triangles {
		front, back := &p.front.Mesh.Triangles[i], &p.back.Mesh.Triangles[i]
		ids := front.Indices
		a, b, c := p.viewed[ids[0]], p.viewed[ids[1]], p.viewed[ids[2]]
		normal := b.Sub(a).Cross(c.Sub(a)).Unit()
		facing := math.Abs(normal.Dot(a.Add(b).Add(c).Unit()))
		rim := math.Pow(max(0, 1-facing), 3)
		front.Color.A, back.Color.A = uint8(skin*(6+55*rim)), uint8(skin*(4+20*rim))
	}
	for _, mesh := range []*meshBatch{p.front, p.back} {
		mesh.Camera = view.camera
		if err := mesh.Update(f); err != nil {
			return err
		}
	}
	if err := p.field.ResetCount(len(p.base) - p.meshCount); err != nil {
		return err
	}
	p.field.Sample(sprites.FieldView{Camera: view.camera})
	p.prepareDots()
	return nil
}

// DCK projects the field once. Cache its axis-aligned point skin in Update so
// drawing either hemisphere needs no callback, per-point allocation or trig.
func (p *jellySphere) prepareDots() {
	p.rearDots, p.frontDots = p.rearDots[:0], p.frontDots[:0]
	count := len(p.base) - p.meshCount
	for _, sample := range p.field.Samples() {
		stagger := float64(sample.Index*73%count) / float64(count)
		alpha := ramp(SphereAt+stagger*.8, SphereAt+stagger*.8+.5, p.seconds)
		alpha *= .3 + .7*ramp(-30, 30, p.centerDepth-sample.Z)
		if alpha <= 0 {
			continue
		}
		var tint ebiten.ColorScale
		tint.Scale(1, .66, .29, 1)
		tint.ScaleAlpha(float32(alpha))
		half := max(1.1, 2.9*sample.Scale) / 2
		dots := &p.rearDots
		if sample.Z < p.centerDepth {
			dots = &p.frontDots
		}
		for _, corner := range [4][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
			*dots = append(*dots, ebiten.Vertex{DstX: float32(sample.X + (corner[0]*2-1)*half), DstY: float32(sample.Y + (corner[1]*2-1)*half),
				SrcX: float32(corner[0]), SrcY: float32(corner[1]), ColorR: tint.R(), ColorG: tint.G(), ColorB: tint.B(), ColorA: tint.A()})
		}
	}
}

func (p *jellySphere) drawDots(dst *ebiten.Image, dots []ebiten.Vertex) {
	p.points.Begin(dst, p.front.texture)
	for i := 0; i < len(dots); i += 4 {
		p.points.Quad([4]ebiten.Vertex{dots[i], dots[i+1], dots[i+2], dots[i+3]})
	}
	p.points.Flush()
}

func (p *jellySphere) drawBack(dst *ebiten.Image) {
	if p.seconds < SphereAt {
		return
	}
	p.back.Draw(dst)
	p.drawDots(dst, p.rearDots)
}
func (p *jellySphere) drawFront(dst *ebiten.Image) {
	if p.seconds < SphereAt {
		return
	}
	p.front.Draw(dst)
	p.drawDots(dst, p.frontDots)
}
func (p *jellySphere) Close() error {
	if p.front != nil {
		p.front.Close()
		p.front = nil
	}
	if p.back != nil {
		p.back.Close()
		p.back = nil
	}
	p.points = nil
	return nil
}
