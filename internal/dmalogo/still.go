// Package dmalogo shares the original logo's crop and placement across parts.
package dmalogo

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/composite"
)

const Width, CenterX, CenterY = 400.0, 320.0, 150.0

type cropped struct {
	image.Image
	rect image.Rectangle
}

func (c cropped) Bounds() image.Rectangle { return c.rect }

// Crop uses the same alpha boundary as the glass volume. Pixels remain borrowed.
func Crop(art image.Image) (image.Image, error) {
	if art == nil || art.Bounds().Empty() {
		return nil, fmt.Errorf("DMA logo: missing original artwork")
	}
	var bounds image.Rectangle
	for y := art.Bounds().Min.Y; y < art.Bounds().Max.Y; y++ {
		for x := art.Bounds().Min.X; x < art.Bounds().Max.X; x++ {
			if _, _, _, alpha := art.At(x, y).RGBA(); alpha >= 0x8000 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if bounds.Empty() {
		return nil, fmt.Errorf("DMA logo: empty alpha silhouette")
	}
	return cropped{Image: art, rect: bounds}, nil
}

// Draw presents the same still at the end of Noname and the start of Jelly.
func Draw(dst, source *ebiten.Image, width, centerY, alpha float64) {
	var options ebiten.DrawImageOptions
	options.Filter = ebiten.FilterLinear
	scale := width / float64(source.Bounds().Dx())
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(CenterX-width/2, centerY-float64(source.Bounds().Dy())*scale/2)
	options.ColorScale.ScaleAlpha(float32(alpha))
	composite.Instance{Image: source, Options: options}.Draw(dst)
}
