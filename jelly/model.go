package jelly

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
)

const logoWidth, logoDepth = 400.0, 22.0

type logoModel struct {
	mesh    effects.Mesh
	outline [][]int
	height  float64
	crop    image.Rectangle
}

// logoArtwork borrows the supplied pixels while excluding transparent canvas
// margins from the model's coordinates and the still image's placement.
type logoArtwork struct {
	image.Image
	rect image.Rectangle
}

func (a logoArtwork) Bounds() image.Rectangle { return a.rect }

type maskPoint struct {
	point    geometry.Vec2
	distance float64
}

// logoVolume clips a triangular lattice against the supplied logo's alpha
// distance field. Matching cap vertices and boundary walls form a real volume,
// including holes and disconnected pieces. No replacement logo is authored.
func logoVolume(art image.Image) (logoModel, error) {
	if art == nil || art.Bounds().Dx() <= 0 || art.Bounds().Dy() <= 0 {
		return logoModel{}, fmt.Errorf("jelly logo: missing DMA artwork")
	}
	crop := image.Rectangle{}
	for y := art.Bounds().Min.Y; y < art.Bounds().Max.Y; y++ {
		for x := art.Bounds().Min.X; x < art.Bounds().Max.X; x++ {
			if _, _, _, a := art.At(x, y).RGBA(); a >= 0x8000 {
				crop = crop.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if crop.Empty() {
		return logoModel{}, fmt.Errorf("jelly logo: empty alpha silhouette")
	}
	art = logoArtwork{Image: art, rect: crop}
	field, w, h := alphaDistance(art)
	height := logoWidth * float64(art.Bounds().Dy()) / float64(art.Bounds().Dx())
	model := logoModel{height: height, crop: crop}
	// At the orbit's projected logo size, this lattice keeps the alpha-clipped
	// contours and bevel while avoiding subpixel interior facets on mobile.
	const columns = 40
	rows := max(2, int(math.Round(columns*height/logoWidth)))
	slots := make(map[[3]int64]int)
	var boundary [][2]int
	twins := make(map[int]int)
	add := func(p maskPoint, back bool) int {
		// Round the cap into a narrow bevel, retaining a closed wall at the
		// alpha boundary. Facet normals then catch light like soft glass.
		bevel := motion.Smooth(min(1, p.distance*logoWidth/192/8))
		z := -logoDepth / 2 * (.28 + .72*bevel)
		if back {
			z = -z
		}
		key := [3]int64{int64(math.Round(p.point.X * 1e6)), int64(math.Round(p.point.Y * 1e6)), int64(math.Round(z * 1e6))}
		if index, found := slots[key]; found {
			return index
		}
		index := len(model.mesh.Points)
		model.mesh.Points = append(model.mesh.Points, geometry.Vec3{X: p.point.X, Y: p.point.Y, Z: z})
		slots[key] = index
		return index
	}
	face := func(a, b, c int, paint color.NRGBA) {
		p, q, r := model.mesh.Points[a], model.mesh.Points[b], model.mesh.Points[c]
		n := q.Sub(p).Cross(r.Sub(p))
		if n.Dot(n) > 1e-12 {
			model.mesh.Triangles = append(model.mesh.Triangles, effects.Triangle{Indices: [3]int{a, b, c}, Color: paint})
		}
	}
	point := func(x, y int) maskPoint {
		u, v := float64(x)/columns, float64(y)/float64(rows)
		return maskPoint{point: geometry.Vec2{X: (u - .5) * logoWidth, Y: (v - .5) * height},
			distance: sampleDistance(field, w, h, 1.5+u*float64(w-4), 1.5+v*float64(h-4))}
	}
	clip := func(triangle [3]maskPoint) {
		var polygon [4]maskPoint
		n := 0
		previous := triangle[2]
		for _, current := range triangle {
			if (previous.distance > 0) != (current.distance > 0) {
				t := previous.distance / (previous.distance - current.distance)
				polygon[n] = maskPoint{point: geometry.Vec2{
					X: previous.point.X + (current.point.X-previous.point.X)*t,
					Y: previous.point.Y + (current.point.Y-previous.point.Y)*t,
				}}
				n++
			}
			if current.distance > 0 {
				polygon[n] = current
				n++
			}
			previous = current
		}
		if n < 3 {
			return
		}
		var front, back [4]int
		for i, p := range polygon[:n] {
			front[i], back[i] = add(p, false), add(p, true)
			twins[front[i]] = back[i]
		}
		// The gentle vertical tint complements DCK's moving facet lighting.
		shade := max(0, min(1, .5-polygon[0].point.Y/height))
		cap := color.NRGBA{R: byte(65 + 60*shade), G: byte(130 + 85*shade), B: 255, A: 155}
		for i := 1; i+1 < n; i++ {
			face(front[0], front[i+1], front[i], cap)
			face(back[0], back[i], back[i+1], color.NRGBA{R: 75, G: 115, B: 235, A: 120})
		}
		for i, p := range polygon[:n] {
			j := (i + 1) % n
			if p.distance != 0 || polygon[j].distance != 0 {
				continue
			}
			a, b, c, d := front[i], front[j], back[j], back[i]
			side := color.NRGBA{R: 65, G: 155, B: 255, A: 215}
			face(a, b, c, side)
			face(a, c, d, side)
			if a != b {
				boundary = append(boundary, [2]int{a, b})
			}
		}
	}
	for y := -1; y <= rows; y++ {
		for x := -1; x <= columns; x++ {
			a, b, c, d := point(x, y), point(x+1, y), point(x+1, y+1), point(x, y+1)
			clip([3]maskPoint{a, b, c})
			clip([3]maskPoint{a, c, d})
		}
	}
	// Outline complete front/back contours once. Internal triangulation and
	// wall subdivisions stay unmarked, preserving a clean glass rim.
	next := make(map[int]int, len(boundary))
	for _, edge := range boundary {
		next[edge[0]] = edge[1]
	}
	visited := make(map[int]bool, len(boundary))
	for _, edge := range boundary {
		if visited[edge[0]] {
			continue
		}
		var front, back []int
		index := edge[0]
		for len(front) <= len(boundary) && !visited[index] {
			visited[index] = true
			front, back = append(front, index), append(back, twins[index])
			var found bool
			index, found = next[index]
			if !found {
				return logoModel{}, fmt.Errorf("jelly logo: open alpha boundary")
			}
		}
		if index != edge[0] || len(front) < 3 {
			return logoModel{}, fmt.Errorf("jelly logo: invalid alpha contour")
		}
		model.outline = append(model.outline, front, back)
	}
	if len(model.mesh.Triangles) == 0 || len(model.outline) == 0 {
		return logoModel{}, fmt.Errorf("jelly logo: empty alpha silhouette")
	}
	return model, nil
}

// A two-pass chamfer field keeps sub-cell intersections near the artwork's
// actual edge. Its small construction grid is discarded after mesh creation.
func alphaDistance(art image.Image) ([]float64, int, int) {
	const samples = 192
	w, h := samples+4, max(2, int(math.Round(samples*float64(art.Bounds().Dy())/float64(art.Bounds().Dx()))))+4
	mask := make([]bool, w*h)
	bounds := art.Bounds()
	for y := 2; y < h-2; y++ {
		for x := 2; x < w-2; x++ {
			sx := bounds.Min.X + min(bounds.Dx()-1, int((float64(x)-1.5)*float64(bounds.Dx())/samples))
			sy := bounds.Min.Y + min(bounds.Dy()-1, int((float64(y)-1.5)*float64(bounds.Dy())/float64(h-4)))
			_, _, _, a := art.At(sx, sy).RGBA()
			mask[y*w+x] = a >= 0x8000
		}
	}
	distance := func(target bool) []float64 {
		d := make([]float64, len(mask))
		for i, value := range mask {
			if value != target {
				d[i] = float64(w + h)
			}
		}
		for pass := 0; pass < 2; pass++ {
			for step := range len(d) {
				i := step
				if pass == 1 {
					i = len(d) - 1 - step
				}
				x, y := i%w, i/w
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						nx, ny := x+dx, y+dy
						if nx < 0 || nx >= w || ny < 0 || ny >= h || dx == 0 && dy == 0 {
							continue
						}
						cost := 1.0
						if dx != 0 && dy != 0 {
							cost = math.Sqrt2
						}
						d[i] = min(d[i], d[ny*w+nx]+cost)
					}
				}
			}
		}
		return d
	}
	inside, outside := distance(true), distance(false)
	for i, value := range mask {
		if value {
			inside[i] = outside[i] - .5
		} else {
			inside[i] = .5 - inside[i]
		}
	}
	return inside, w, h
}

func sampleDistance(field []float64, w, h int, x, y float64) float64 {
	if x < 0 || y < 0 || x >= float64(w-1) || y >= float64(h-1) {
		return -1
	}
	x0, y0 := int(x), int(y)
	u, v := x-float64(x0), y-float64(y0)
	a := field[y0*w+x0]*(1-u) + field[y0*w+x0+1]*u
	b := field[(y0+1)*w+x0]*(1-u) + field[(y0+1)*w+x0+1]*u
	d := a*(1-v) + b*v
	// Avoid an ambiguous lattice vertex shared by more than one zero edge.
	if math.Abs(d) < 1e-9 {
		return -1e-9
	}
	return d
}
