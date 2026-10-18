package spoon

import (
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/sprites"
)

// spoonMesh is an authored, closed bowl and bevelled handle. The broad triangular
// facets remain visible under the moving silver environment reflection.
func spoonMesh() effects.Mesh {
	m := effects.Mesh{}
	add := func(p geometry.Vec3) int { m.Points = append(m.Points, p); return len(m.Points) - 1 }
	tri := func(a, b, c int) {
		m.Triangles = append(m.Triangles, effects.Triangle{Indices: [3]int{a, b, c}, Color: color.NRGBA{255, 255, 255, 255}})
	}
	const rings, segments = 5, 12
	for side := 0; side < 2; side++ {
		start := len(m.Points)
		add(geometry.Vec3{Y: -99, Z: 22 + float64(side)*4})
		for ring := 1; ring <= rings; ring++ {
			r := float64(ring) / rings
			for j := 0; j < segments; j++ {
				a := float64(j) * 2 * math.Pi / segments
				add(geometry.Vec3{X: 53 * r * math.Cos(a), Y: -99 + 68*r*math.Sin(a), Z: 22*(1-r*r) + float64(side)*4})
			}
		}
		face := func(a, b, c int) {
			if side == 1 {
				b, c = c, b
			}
			tri(a, b, c)
		}
		for j := 0; j < segments; j++ {
			face(start, start+1+(j+1)%segments, start+1+j)
		}
		for ring := 1; ring < rings; ring++ {
			for j := 0; j < segments; j++ {
				a := start + 1 + (ring-1)*segments + j
				b := start + 1 + ring*segments + j
				c := start + 1 + ring*segments + (j+1)%segments
				d := start + 1 + (ring-1)*segments + (j+1)%segments
				face(a, c, b)
				face(a, d, c)
			}
		}
	}
	const perSide = 1 + rings*segments
	for j := 0; j < segments; j++ {
		a := 1 + (rings-1)*segments + j
		b := 1 + (rings-1)*segments + (j+1)%segments
		tri(a, b, a+perSide)
		tri(b, b+perSide, a+perSide)
	}
	rows := []struct{ y, w float64 }{
		{-39, 5}, {-33, 5}, {-27, 5}, {-21, 5.3}, {-15, 5.7}, {-9, 6.1}, {-3, 6.5}, {3, 6.8},
		{9, 7}, {33, 8}, {58, 9.5}, {83, 11}, {108, 13}, {132, 14}, {147, 12}, {156, 7}, {160, 1},
	}
	start := len(m.Points)
	const bevel = 8
	for _, r := range rows {
		for j := 0; j < bevel; j++ {
			a := float64(j) * 2 * math.Pi / bevel
			add(geometry.Vec3{X: r.w * math.Cos(a), Y: r.y, Z: 2 + 3*math.Sin(a)})
		}
	}
	for row := 0; row < len(rows)-1; row++ {
		for j := 0; j < bevel; j++ {
			a := start + row*bevel + j
			b := start + (row+1)*bevel + j
			c := start + (row+1)*bevel + (j+1)%bevel
			d := start + row*bevel + (j+1)%bevel
			tri(a, b, c)
			tri(a, c, d)
		}
	}
	for j := 1; j < bevel-1; j++ {
		tri(start, start+j+1, start+j)
		end := start + (len(rows)-1)*bevel
		tri(end, end+j, end+j+1)
	}
	return m
}

func smooth(from, to, t float64) float64 { return motion.Smooth(max(0, min(1, (t-from)/(to-from)))) }

// spoonMaterial advances from bowl to handle only after the entire cloud has
// settled. The same front fades each ball out as the matching facets appear.
func spoonMaterial(seconds, y float64) float64 {
	position := max(0, min(1, (y+168)/328))
	const fade = .55
	start := SpoonAt + position*(SpoonSolidAt-SpoonAt-fade)
	return smooth(start, start+fade, seconds)
}

// bendSpoon integrates a circular neck arc, then carries the rigid bowl along
// its tangent. The handle stays fixed and the metal never changes thickness.
func bendSpoon(p geometry.Vec3, t float64) geometry.Vec3 {
	angle := (5 * math.Pi / 6) * smooth(BendAt, BendAt+6, t)
	const pivot, span = 5.0, 43.0
	s := pivot - p.Y
	if angle < .000001 || s <= 0 {
		return p
	}
	radius := span / angle
	theta := min(angle, s/radius)
	y := pivot - radius*math.Sin(theta)
	z := radius * (1 - math.Cos(theta))
	if s > span {
		y -= (s - span) * math.Cos(angle)
		z += (s - span) * math.Sin(angle)
	}
	return geometry.Vec3{X: p.X, Y: y + p.Z*math.Sin(theta), Z: z + p.Z*math.Cos(theta)}
}

func spoonPose(t float64) effects.Transform {
	pose := effects.Transform{Scale: 1, Position: geometry.Vec3{Y: 8 * math.Sin(t*.4), Z: 190 + 510*smooth(RevealAt, SpoonAt, t)}, Rotation: geometry.Vec3{X: -.32 + .11*math.Sin(t*.31), Y: .42 * math.Sin(t*.22), Z: -.58 + .11*math.Sin(t*.27)}}
	// Once the metal is complete, turn towards a near-profile view before the
	// neck bends. Hold that view so its depth curve reads across the screen and
	// the fixed handle remains a stable reference throughout the fold.
	profile := smooth(SpoonSolidAt, BendAt, t)
	pose.Position = geometry.Lerp(pose.Position, geometry.Vec3{X: 28, Z: 700}, profile)
	pose.Rotation = geometry.Lerp(pose.Rotation, geometry.Vec3{X: .06, Y: -78 * math.Pi / 180, Z: -.14}, profile)
	return pose
}

type metal struct {
	mesh        *effects.MeshEffect
	material    *ebiten.Image
	transformed []geometry.Vec3
}

func newMetal() (*metal, error) {
	im := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			nx, ny := float64(x)/63.5-1, float64(y)/63.5-1
			// A compact environment map supplies cold shadows, a broad softbox and
			// narrow white reflection bands without an external generated texture.
			v := .14 + .24*(ny+1)/2 + .5*math.Exp(-math.Pow((ny+.28)/.14, 2)) + .32*math.Exp(-math.Pow((ny-.53)/.055, 2))
			v += .16 * math.Exp(-math.Pow((nx+.4)/.23, 2))
			v = min(1, max(0, v))
			im.SetNRGBA(x, y, color.NRGBA{R: byte(10 + 228*v), G: byte(17 + 230*v), B: byte(30 + 225*v), A: 255})
		}
	}
	p := &metal{material: ebiten.NewImageFromImage(im)}
	mesh, err := effects.NewMesh(spoonMesh(), p.material, geometry.Camera{Center: geometry.Vec2{X: 320, Y: 190}, Focal: 650, Near: 24})
	if err != nil {
		p.material.Deallocate()
		return nil, err
	}
	p.mesh = mesh
	p.transformed = make([]geometry.Vec3, len(mesh.Mesh.Points))
	mesh.CullBackFaces = true
	mesh.Deform = func(_ int, p geometry.Vec3, t float64) geometry.Vec3 { return bendSpoon(p, t) }
	mesh.Animate = spoonPose
	return p, nil
}

func (p *metal) Update(f kit.Frame) error {
	pose := spoonPose(f.Time)
	rot := geometry.RotateXYZ(pose.Rotation)
	for i, v := range p.mesh.Mesh.Points {
		p.transformed[i] = rot.Apply(bendSpoon(v, f.Time))
	}
	for i := range p.mesh.Mesh.Triangles {
		face := &p.mesh.Mesh.Triangles[i]
		ids := face.Indices
		a, b, c := p.transformed[ids[0]], p.transformed[ids[1]], p.transformed[ids[2]]
		n := b.Sub(a).Cross(c.Sub(a)).Unit()
		uv := geometry.Vec2{X: 63.5 * (n.X + 1), Y: 63.5 * (n.Y + 1)}
		face.UV = [3]geometry.Vec2{uv, uv, uv}
		y := (p.mesh.Mesh.Points[ids[0]].Y + p.mesh.Mesh.Points[ids[1]].Y + p.mesh.Mesh.Points[ids[2]].Y) / 3
		face.Color.A = uint8(math.Round(255 * spoonMaterial(f.Time, y)))
	}
	return p.mesh.Update(f)
}
func (p *metal) Draw(dst *ebiten.Image) { p.mesh.Draw(dst) }
func (p *metal) Close() error           { p.mesh.Close(); p.material.Deallocate(); return nil }

const ballCount = 1600

type ballCloud struct {
	field                   *sprites.Field
	renderer                *sprites.FieldRenderer
	style                   sprites.FieldStyle
	target                  []geometry.Vec3
	seconds                 float64
	frame                   ballFrame
	fades                   []float64
	preparedAt, liveSeconds float64
	prepared, dirty, live   bool
}

// hash is deterministic across architectures and independent of frame rate.
func hash(n uint32) uint32 {
	n ^= n >> 16
	n *= 0x7feb352d
	n ^= n >> 15
	n *= 0x846ca68b
	return n ^ (n >> 16)
}
func noise(n int) float64 { return float64(hash(uint32(n))) / 4294967296 }

func newBallCloud(art image.Image) (*ballCloud, error) {
	c := &ballCloud{renderer: sprites.NewFieldRenderer(ballCount)}
	texture := ebiten.NewImageFromImage(art)
	frames := make([]image.Rectangle, 16)
	for i := range frames {
		bounds := image.Rectangle{}
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				_, _, _, a := art.At(i*16+x, y).RGBA()
				if a != 0 {
					bounds = bounds.Union(image.Rect(i*16+x, y, i*16+x+1, y+1))
				}
			}
		}
		frames[i] = bounds
	}
	c.style = sprites.FieldStyle{Image: texture, Frames: frames, Filter: ebiten.FilterLinear, Appearance: sprites.FieldAppearance{AnchorX: .5, AnchorY: .5}, Sample: func(s sprites.FieldSample, a *sprites.FieldAppearance) bool {
		d := max(1.3, min(40, 7*s.Scale))
		a.Width, a.Height = d, d
		fog := float32(max(.28, min(1, 1.3-s.Z/1500)))
		a.Tint.Scale(fog, fog, fog, 1)
		a.Tint.ScaleAlpha(float32((1 - spoonMaterial(c.preparedAt, c.target[s.Index].Y)) * c.fades[s.Index]))
		return s.X >= -40 && s.X <= Width+40 && s.Y >= -40 && s.Y <= Height+40
	}}
	m := spoonMesh()
	areas := make([]float64, len(m.Triangles))
	total := 0.0
	for i, f := range m.Triangles {
		a, b, d := m.Points[f.Indices[0]], m.Points[f.Indices[1]], m.Points[f.Indices[2]]
		v := b.Sub(a).Cross(d.Sub(a))
		total += math.Sqrt(v.Dot(v)) / 2
		areas[i] = total
	}
	for i := 0; i < ballCount; i++ {
		index := sort.SearchFloat64s(areas, noise(i*7+11)*total)
		f := m.Triangles[index]
		u := math.Sqrt(noise(i*7 + 12))
		v := noise(i*7 + 13)
		p := m.Points[f.Indices[0]].Scale(1 - u).Add(m.Points[f.Indices[1]].Scale(u * (1 - v))).Add(m.Points[f.Indices[2]].Scale(u * v))
		c.target = append(c.target, p)
	}
	var err error
	c.frame = ballFrameAt(0)
	c.fades = make([]float64, ballCount)
	c.field, err = sprites.NewField(sprites.FieldConfig{Count: ballCount, Spawn: func(index int, _ bool) sprites.Point {
		p, fade := c.frame.point(index, c.target[index])
		c.fades[index] = fade
		scale := 650 / p.Z
		frame := max(0, min(15, int(16-7*scale)))
		return sprites.Point{X: p.X, Y: p.Y, Z: p.Z, Image: frame}
	}})
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}
func (c *ballCloud) Update(f kit.Frame) error {
	c.seconds = f.Time
	c.dirty = true
	return nil
}
func (c *ballCloud) sampleAt(seconds float64) error {
	if c.prepared && c.preparedAt == seconds {
		return nil
	}
	c.prepared, c.preparedAt = true, seconds
	c.frame = ballFrameAt(seconds)
	if err := c.field.ResetCount(len(c.target)); err != nil {
		return err
	}
	c.field.Sample(sprites.FieldView{Camera: geometry.Camera{Center: geometry.Vec2{X: 320, Y: 190}, Focal: 650, Near: 24}, SortDepth: true})
	return nil
}
func (c *ballCloud) Draw(dst *ebiten.Image) {
	if c.live {
		_ = c.sampleAt(c.liveSeconds)
	} else if c.dirty || !c.prepared {
		_ = c.sampleAt(c.seconds)
		c.dirty = false
	}
	c.renderer.Draw(dst, c.field.Samples(), c.style)
}
func (c *ballCloud) Close() error { c.renderer.Close(); c.style.Image.Deallocate(); return nil }
