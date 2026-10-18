package city

import (
	"image"
	"image/color"
	"math"

	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/dma40/internal/dmalogo"
)

type builder struct{ effects.Mesh }

func (b *builder) quad(a, c, d, e geometry.Vec3, paint color.NRGBA) {
	i := len(b.Points)
	b.Points = append(b.Points, a, c, d, e)
	b.Triangles = append(b.Triangles,
		effects.Triangle{Indices: [3]int{i, i + 1, i + 2}, Color: paint},
		effects.Triangle{Indices: [3]int{i, i + 2, i + 3}, Color: paint})
}

func (b *builder) box(center, size geometry.Vec3, paint color.NRGBA) {
	m := effects.Cube(1, geometry.Vec2{}, paint)
	offset := len(b.Points)
	for _, p := range m.Points {
		b.Points = append(b.Points, geometry.Vec3{X: center.X + p.X*size.X, Y: center.Y + p.Y*size.Y, Z: center.Z + p.Z*size.Z})
	}
	for _, face := range m.Triangles {
		for j := range face.Indices {
			face.Indices[j] += offset
		}
		b.Triangles = append(b.Triangles, face)
	}
}

func random(n int) float64 {
	x := uint32(n)*747796405 + 2891336453
	x = ((x >> ((x >> 28) + 4)) ^ x) * 277803737
	return float64((x>>22)^x) / 4294967296
}

func (r route) clearance(x, z float64) float64 {
	best := math.Inf(1)
	for d := 0.0; d <= r.path.Length(); d += 8 {
		p, _ := r.path.At(d)
		best = min(best, math.Hypot(x-p.X, z-p.Y))
	}
	return best
}

type building struct {
	mesh      effects.Mesh
	center    geometry.Vec3
	radius    float64
	materials []faceMaterial
}

type district struct {
	ground, campus                   effects.Mesh
	buildings                        []building
	groundMaterials, campusMaterials []faceMaterial
}

func newDistrict(r route, logo image.Image) (district, error) {
	var b builder
	b.quad(geometry.Vec3{X: -1400, Z: -1400}, geometry.Vec3{X: 1400, Z: -1400}, geometry.Vec3{X: 1400, Z: 2100}, geometry.Vec3{X: -1400, Z: 2100}, color.NRGBA{R: 19, G: 31, B: 45, A: 255})
	// Extend the entrance behind the following camera so the forming car is
	// already on a continuous road rather than at the edge of a polygon.
	start, direction := r.path.At(0)
	var approach [4]geometry.Vec3
	for i, corner := range [4][2]float64{{-100, -15}, {-100, 15}, {0, 15}, {0, -15}} {
		approach[i] = geometry.Vec3{X: start.X + direction.X*corner[0] + direction.Y*corner[1], Y: -.025, Z: start.Y + direction.Y*corner[0] - direction.X*corner[1]}
	}
	b.quad(approach[0], approach[1], approach[2], approach[3], color.NRGBA{R: 34, G: 44, B: 60, A: 255})
	// Lay the road on the route itself; the car never moves over a backdrop.
	for d := 0.0; d < r.path.Length(); d += 7 {
		a, ta := r.path.At(d)
		c, tc := r.path.At(d + 7.2)
		point := func(p, t motion.Point, side, height float64) geometry.Vec3 {
			return geometry.Vec3{X: p.X + t.Y*side, Y: height, Z: p.Y - t.X*side}
		}
		b.quad(point(a, ta, -15, -.025), point(a, ta, 15, -.025), point(c, tc, 15, -.025), point(c, tc, -15, -.025), color.NRGBA{R: 34, G: 44, B: 60, A: 255})
		for _, side := range []float64{-14, 14} {
			b.quad(point(a, ta, side-.14, -.045), point(a, ta, side+.14, -.045), point(c, tc, side+.14, -.045), point(c, tc, side-.14, -.045), color.NRGBA{R: 43, G: 191, B: 213, A: 255})
		}
		if int(d/7)%3 != 0 {
			b.quad(point(a, ta, -.13, -.05), point(a, ta, .13, -.05), point(c, tc, .13, -.05), point(c, tc, -.13, -.05), color.NRGBA{R: 208, G: 205, B: 147, A: 255})
		}
	}
	ground := b.Mesh
	b = builder{}
	var buildings []building
	for zi := -8; zi <= 18; zi++ {
		for xi := -11; xi <= 11; xi++ {
			id := (zi+10)*61 + xi + 20
			x, z := float64(xi)*64+12*random(id), float64(zi)*64+12*random(id+1)
			if math.Abs(x) < 275 && math.Abs(z-900) < 150 || r.clearance(x, z) < 43 {
				continue
			}
			w, depth, h := 22+random(id+2)*16, 23+random(id+3)*13, 12+random(id+4)*62
			paint := color.NRGBA{R: byte(40 + 45*random(id+5)), G: byte(72 + 39*random(id+6)), B: byte(111 + 50*random(id+7)), A: 255}
			b.box(geometry.Vec3{X: x, Y: -h / 2, Z: z}, geometry.Vec3{X: w, Y: h, Z: depth}, paint)
			b.box(geometry.Vec3{X: x, Y: -h - .4, Z: z}, geometry.Vec3{X: w + .5, Y: .8, Z: depth + .5}, color.NRGBA{R: 115, G: 154, B: 179, A: 255})
			// Sparse window strips keep recognizable facades without thousands
			// of separately shaded window meshes on mobile.
			for y := 5.0; y < h-2; y += 8 {
				window := color.NRGBA{R: 114, G: 202, B: 218, A: 255}
				if (id+int(y))%5 == 0 {
					window = color.NRGBA{R: 244, G: 184, B: 109, A: 255}
				}
				b.quad(geometry.Vec3{X: x - w*.38, Y: -y - 1.3, Z: z - depth/2 - .025}, geometry.Vec3{X: x - w*.38, Y: -y, Z: z - depth/2 - .025}, geometry.Vec3{X: x + w*.38, Y: -y, Z: z - depth/2 - .025}, geometry.Vec3{X: x + w*.38, Y: -y - 1.3, Z: z - depth/2 - .025}, window)
				b.quad(geometry.Vec3{X: x + w/2 + .025, Y: -y - 1.3, Z: z - depth*.38}, geometry.Vec3{X: x + w/2 + .025, Y: -y, Z: z - depth*.38}, geometry.Vec3{X: x + w/2 + .025, Y: -y, Z: z + depth*.38}, geometry.Vec3{X: x + w/2 + .025, Y: -y - 1.3, Z: z + depth*.38}, window)
			}
			buildings = append(buildings, building{mesh: b.Mesh, center: geometry.Vec3{X: x, Y: -h / 2, Z: z}, radius: math.Sqrt(w*w+depth*depth+h*h)/2 + 1, materials: cacheMaterials(b.Mesh)})
			b = builder{}
		}
	}
	b = builder{}
	art, err := dmalogo.Crop(logo)
	if err != nil {
		return district{}, err
	}
	const columns = 66
	rows := max(1, int(math.Round(columns*float64(art.Bounds().Dy())/float64(art.Bounds().Dx()))))
	unit := 500.0 / columns
	// The original alpha silhouette supplies the actual rooftop street plan.
	for y := 0; y < rows; y++ {
		for x := 0; x < columns; x++ {
			sx, sy := art.Bounds().Min.X+(2*x+1)*art.Bounds().Dx()/(2*columns), art.Bounds().Min.Y+(2*y+1)*art.Bounds().Dy()/(2*rows)
			red, green, blue, alpha := art.At(sx, sy).RGBA()
			if alpha < 32768 {
				continue
			}
			paint := color.NRGBA{R: uint8(red * 255 / alpha), G: uint8(green * 255 / alpha), B: uint8(blue * 255 / alpha), A: 255}
			b.box(geometry.Vec3{X: (float64(x) + .5 - columns/2.0) * unit, Y: -14, Z: 900 - (float64(y)+.5-float64(rows)/2)*unit}, geometry.Vec3{X: unit * .98, Y: 28, Z: unit * .98}, paint)
		}
	}
	return district{ground: ground, buildings: buildings, campus: b.Mesh, groundMaterials: cacheMaterials(ground), campusMaterials: cacheMaterials(b.Mesh)}, nil
}
