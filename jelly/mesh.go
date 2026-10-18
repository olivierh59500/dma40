package jelly

import (
	"fmt"
	"image/color"
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
)

// meshBatch specializes DCK's mesh data for this part's fixed topology. Shared
// points project once in Update; Draw reuses prepared, premultiplied vertices.
// Additive logo faces need no depth sort. The two
// translucent sphere hemispheres retain a stable sort of compact face records.
// DCK owns camera projection, near clipping and outlines. Ebitengine receives
// the prepared triangle stream directly, avoiding a second geometry copy.
type meshBatch struct {
	Mesh                     effects.Mesh
	Camera                   geometry.Camera
	Deform                   func(int, geometry.Vec3, float64) geometry.Vec3
	CullBackFaces, DepthSort bool
	Light                    geometry.Vec3
	Ambient                  float64
	Blend                    ebiten.Blend
	points                   []geometry.Vec3
	projected                []geometry.Vec2
	visible                  []bool
	order                    []meshFace
	vertices                 []ebiten.Vertex
	indices                  []uint32
	texture                  *ebiten.Image
	owned                    bool
	outline                  *effects.MeshEffect
}

type meshFace struct {
	index int
	depth float64
	paint color.NRGBA
}

func newMeshBatch(mesh effects.Mesh, texture *ebiten.Image, camera geometry.Camera) (*meshBatch, error) {
	if camera.Near <= 0 || camera.Focal <= 0 || len(mesh.Points) == 0 {
		return nil, fmt.Errorf("jelly mesh: invalid camera or points")
	}
	for _, face := range mesh.Triangles {
		for _, index := range face.Indices {
			if index < 0 || index >= len(mesh.Points) {
				return nil, fmt.Errorf("jelly mesh: invalid point index")
			}
		}
	}
	m := &meshBatch{Mesh: effects.Mesh{Points: slices.Clone(mesh.Points), Triangles: slices.Clone(mesh.Triangles)}, Camera: camera,
		Ambient: 1, points: make([]geometry.Vec3, len(mesh.Points)), projected: make([]geometry.Vec2, len(mesh.Points)),
		visible: make([]bool, len(mesh.Points)), order: make([]meshFace, 0, len(mesh.Triangles)),
		vertices: make([]ebiten.Vertex, 0, len(mesh.Triangles)*6), texture: texture}
	m.indices = make([]uint32, cap(m.vertices))
	for i := range m.indices {
		m.indices[i] = uint32(i)
	}
	if texture == nil {
		m.texture = ebiten.NewImage(1, 1)
		m.texture.Fill(color.White)
		m.owned = true
	}
	return m, nil
}

func (m *meshBatch) Update(f kit.Frame) error {
	for i, point := range m.Mesh.Points {
		if m.Deform != nil {
			point = m.Deform(i, point, f.Time)
		}
		m.points[i] = point
		m.projected[i], _, m.visible[i] = m.Camera.Project(point)
	}
	m.order = m.order[:0]
	light := m.Light.Unit()
	for index, face := range m.Mesh.Triangles {
		if face.Color.A == 0 {
			continue
		}
		ids := face.Indices
		a, b, c := m.points[ids[0]], m.points[ids[1]], m.points[ids[2]]
		paint := face.Color
		if m.CullBackFaces || m.Ambient != 1 {
			normal := b.Sub(a).Cross(c.Sub(a)).Unit()
			if m.CullBackFaces && normal.Dot(a) >= 0 {
				continue
			}
			intensity := math.Max(0, math.Min(1, m.Ambient+(1-m.Ambient)*math.Max(0, normal.Dot(light))))
			paint.R, paint.G, paint.B = uint8(float64(paint.R)*intensity), uint8(float64(paint.G)*intensity), uint8(float64(paint.B)*intensity)
		}
		m.order = append(m.order, meshFace{index: index, depth: (a.Z + b.Z + c.Z) / 3, paint: paint})
	}
	if m.DepthSort {
		slices.SortStableFunc(m.order, func(a, b meshFace) int {
			if a.depth > b.depth {
				return -1
			}
			if a.depth < b.depth {
				return 1
			}
			return 0
		})
	}
	m.vertices = m.vertices[:0]
	for _, sample := range m.order {
		face := m.Mesh.Triangles[sample.index]
		ids := face.Indices
		// Calling the concrete color method avoids boxing a color interface
		// three times per triangle, which otherwise allocates on every Draw.
		r, g, b, a := sample.paint.RGBA()
		paint := ebiten.Vertex{ColorR: float32(r) / 65535, ColorG: float32(g) / 65535, ColorB: float32(b) / 65535, ColorA: float32(a) / 65535}
		if m.visible[ids[0]] && m.visible[ids[1]] && m.visible[ids[2]] {
			for corner, index := range ids {
				v := paint
				p, uv := m.projected[index], face.UV[corner]
				v.DstX, v.DstY, v.SrcX, v.SrcY = float32(p.X), float32(p.Y), float32(uv.X), float32(uv.Y)
				m.vertices = append(m.vertices, v)
			}
			continue
		}
		var scratch [4]geometry.Vertex
		triangle := [3]geometry.Vertex{}
		for corner, index := range ids {
			triangle[corner] = geometry.Vertex{Position: m.points[index], UV: face.UV[corner]}
		}
		polygon := geometry.ClipNear(scratch[:0], triangle, m.Camera.Near)
		for j := 1; j+1 < len(polygon); j++ {
			for _, corner := range [3]geometry.Vertex{polygon[0], polygon[j], polygon[j+1]} {
				p, _, _ := m.Camera.Project(corner.Position)
				v := paint
				v.DstX, v.DstY, v.SrcX, v.SrcY = float32(p.X), float32(p.Y), float32(corner.UV.X), float32(corner.UV.Y)
				m.vertices = append(m.vertices, v)
			}
		}
	}
	if m.outline != nil {
		m.outline.Camera, m.outline.CullBackFaces = m.Camera, m.CullBackFaces
		return m.outline.Update(f)
	}
	return nil
}

func (m *meshBatch) Draw(dst *ebiten.Image) {
	if len(m.vertices) == 0 {
		return
	}
	// Fixed topology already supplies a prepared triangle stream. Sending its
	// cached 32-bit indices avoids copying it through a second transient batch.
	dst.DrawTriangles32(m.vertices, m.indices[:len(m.vertices)], m.texture, &ebiten.DrawTrianglesOptions{Blend: m.Blend, ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha})
}

func (m *meshBatch) SetOutline(config effects.MeshOutlineConfig) error {
	{
		if m.outline != nil {
			m.outline.Close()
			m.outline = nil
		}
		points := make([]geometry.Vec3, 0)
		source := make([]int, 0)
		remap := make(map[int]int)
		faces := make([][]int, len(config.Faces))
		for i, face := range config.Faces {
			for _, old := range face {
				if old < 0 || old >= len(m.Mesh.Points) {
					return fmt.Errorf("jelly outline: invalid point")
				}
				index, ok := remap[old]
				if !ok {
					index = len(points)
					remap[old] = index
					points = append(points, m.Mesh.Points[old])
					source = append(source, old)
				}
				faces[i] = append(faces[i], index)
			}
		}
		config.Faces = faces
		var err error
		m.outline, err = effects.NewMesh(effects.Mesh{Points: points}, m.texture, m.Camera)
		if err != nil {
			return err
		}
		m.outline.Deform = func(index int, _ geometry.Vec3, _ float64) geometry.Vec3 { return m.points[source[index]] }
	}
	if config.White == nil && m.owned {
		config.White = m.texture
	}
	return m.outline.SetOutline(config)
}
func (m *meshBatch) DrawOutline(dst *ebiten.Image) {
	if m.outline != nil {
		m.outline.DrawOutline(dst)
	}
}
func (m *meshBatch) Close() error {
	if m.outline != nil {
		m.outline.Close()
		m.outline = nil
	}
	if m.owned && m.texture != nil {
		m.texture.Deallocate()
		m.texture = nil
	}
	return nil
}
