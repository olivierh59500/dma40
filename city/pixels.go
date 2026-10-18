package city

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/geometry"
)

const tile = 4
const pixelTiles = Width * Height / (tile * tile)

// Every source pixel stays in a textured four-by-four tile. Morphing the tiles
// covers the complete incoming frame without CPU readback or a replacement image.
type pixelMorph struct {
	from, to, sampled []geometry.Vec3
	lift              []float64
	vertices          []ebiten.Vertex
	indices           []uint32
	shader            *ebiten.Shader
	options           ebiten.DrawTrianglesShaderOptions
}

func newPixelMorph(car *carModel, r route) (*pixelMorph, error) {
	p := &pixelMorph{from: make([]geometry.Vec3, pixelTiles), to: make([]geometry.Vec3, pixelTiles), sampled: make([]geometry.Vec3, pixelTiles), lift: make([]float64, pixelTiles), vertices: make([]ebiten.Vertex, pixelTiles*4), indices: make([]uint32, pixelTiles*6)}
	var err error
	p.shader, err = ebiten.NewShader([]byte(pixelShader))
	if err != nil {
		return nil, err
	}
	p.options.Uniforms = map[string]any{"Material": float32(0), "Opacity": float32(1)}
	v, pose := r.cameraAt(FormedAt), r.pose(FormedAt)
	rot := geometry.RotateXYZ(pose.rotation)
	type surface struct {
		points [3]geometry.Vec3
		paint  color.NRGBA
	}
	var faces []surface
	var ends []float64
	total := 0.0
	for _, face := range car.Triangles {
		var points [3]geometry.Vec3
		var flat [3]geometry.Vec2
		valid := true
		for j, id := range face.Indices {
			points[j] = v.point(rot.Apply(car.Points[id]).Add(pose.position))
			var ok bool
			flat[j], _, ok = v.camera.Project(points[j])
			valid = valid && ok
		}
		if !valid {
			continue
		}
		a, b, c := flat[0], flat[1], flat[2]
		area := math.Abs((b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X)) / 2
		if area < .01 {
			continue
		}
		total += area
		ends = append(ends, total)
		faces = append(faces, surface{points: points, paint: face.Color})
	}
	for i := range p.from {
		x, y := i%(Width/tile), i/(Width/tile)
		p.from[i] = geometry.Vec3{X: float64(x*tile + tile/2), Y: float64(y*tile + tile/2)}
		face := faces[sort.SearchFloat64s(ends, random(i*7+1901)*total)]
		u, w := math.Sqrt(random(i*7+1902)), random(i*7+1903)
		world := face.points[0].Scale(1 - u).Add(face.points[1].Scale(u * (1 - w))).Add(face.points[2].Scale(u * w))
		xy, _, _ := v.camera.Project(world)
		p.to[i] = geometry.Vec3{X: xy.X, Y: xy.Y, Z: world.Z}
		p.lift[i] = (random(i+412) - .5) * 65
		for j, delta := range [4][2]int{{0, 0}, {tile, 0}, {tile, tile}, {0, tile}} {
			p.vertices[i*4+j].SrcX, p.vertices[i*4+j].SrcY = float32(x*tile+delta[0]), float32(y*tile+delta[1])
			v := &p.vertices[i*4+j]
			v.ColorR, v.ColorG, v.ColorB, v.ColorA = float32(face.paint.R)/255, float32(face.paint.G)/255, float32(face.paint.B)/255, 1
		}
		base := uint32(i * 4)
		copy(p.indices[i*6:], []uint32{base, base + 1, base + 2, base, base + 2, base + 3})
	}
	return p, nil
}

func (p *pixelMorph) draw(dst, source *ebiten.Image, seconds float64) {
	u := ease(.12, FormedAt-.35, seconds)
	geometry.Morph(p.sampled, p.from, p.to, u)
	size := mix(tile, 1.3, u) / 2
	lift := math.Sin(u * math.Pi)
	for i, point := range p.sampled {
		point.Y -= lift * p.lift[i]
		for j, corner := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			v := &p.vertices[i*4+j]
			v.DstX, v.DstY = float32(point.X+corner[0]*size), float32(point.Y+corner[1]*size)
		}
	}
	p.options.Images[0] = source
	p.options.Uniforms["Material"] = float32(ease(1.9, FormedAt-.3, seconds))
	p.options.Uniforms["Opacity"] = float32(1 - ease(FormedAt-.35, FormedAt, seconds))
	dst.DrawTrianglesShader32(p.vertices, p.indices, p.shader, &p.options)
}

func (p *pixelMorph) close() { p.shader.Deallocate() }

const pixelShader = `//kage:unit pixels
package main
var Material, Opacity float
func Fragment(position vec4, uv vec2, color vec4) vec4 {
 source := imageSrc0At(uv)
 bright := max(source.r,max(source.g,source.b))
 alpha := mix(smoothstep(0,.025,bright),1,Material)
 // Keep the complete original frame before the tiles separate.
 alpha = mix(1,alpha,smoothstep(0,.03,Material))
 return vec4(mix(source.rgb,color.rgb,Material)*alpha,alpha)*Opacity
}
`
