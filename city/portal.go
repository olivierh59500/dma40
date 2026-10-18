package city

import "image"

// portalDistance derives an expanding aperture from the supplied DMA alpha.
// Distances are prepared once on the CPU; the shader needs one texture sample
// per pixel. A growing distance threshold fills the logo's gaps without closing
// regions already revealed, as scaling its complex silhouette could do.
func portalDistance(logo image.Image) *image.RGBA {
	const infinity = 3 * (Width + Height)
	distance := make([]int, Width*Height)
	bounds := logo.Bounds()
	scale := 470 / float64(bounds.Dx())
	top := 212 - float64(bounds.Dy())*scale/2
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			i := y*Width + x
			distance[i] = infinity
			px, py := float64(x)+.5-85, float64(y)+.5-top
			if px < 0 || py < 0 {
				continue
			}
			point := image.Pt(bounds.Min.X+int(px/scale), bounds.Min.Y+int(py/scale))
			if point.In(bounds) {
				if _, _, _, alpha := logo.At(point.X, point.Y).RGBA(); alpha >= 0x8000 {
					distance[i] = 0
				}
			}
		}
	}
	// A symmetric 3/4 chamfer pass follows horizontal, vertical and diagonal
	// logo edges in linear time, retaining the pixel-art silhouette.
	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			i := y*Width + x
			if x > 0 {
				distance[i] = min(distance[i], distance[i-1]+3)
			}
			if y > 0 {
				distance[i] = min(distance[i], distance[i-Width]+3)
				if x > 0 {
					distance[i] = min(distance[i], distance[i-Width-1]+4)
				}
				if x+1 < Width {
					distance[i] = min(distance[i], distance[i-Width+1]+4)
				}
			}
		}
	}
	maximum := 1
	for y := Height - 1; y >= 0; y-- {
		for x := Width - 1; x >= 0; x-- {
			i := y*Width + x
			if x+1 < Width {
				distance[i] = min(distance[i], distance[i+1]+3)
			}
			if y+1 < Height {
				distance[i] = min(distance[i], distance[i+Width]+3)
				if x > 0 {
					distance[i] = min(distance[i], distance[i+Width-1]+4)
				}
				if x+1 < Width {
					distance[i] = min(distance[i], distance[i+Width+1]+4)
				}
			}
			maximum = max(maximum, distance[i])
		}
	}
	field := image.NewRGBA(image.Rect(0, 0, Width, Height))
	for i, d := range distance {
		level := uint8((d*255 + maximum/2) / maximum)
		field.Pix[4*i], field.Pix[4*i+1], field.Pix[4*i+2], field.Pix[4*i+3] = level, level, level, 255
	}
	return field
}
