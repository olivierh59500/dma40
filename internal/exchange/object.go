// Package exchange exposes original demo objects to the shared composition.
package exchange

import (
	"encoding/binary"
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/render"
)

// Object transfers one original object between its native painter and the
// shared foreground. Draw receives native-to-composition coordinates and never
// advances the source clock. SetAway is called before the native update.
type Object struct {
	Name    string
	SetAway func(bool)
	Draw    func(*ebiten.Image, ebiten.GeoM)
}

type Provider interface {
	ExchangeObjects() ([]Object, error)
}

// Colors expands the original ST palette and keys the source background index.
func Colors(raw []byte) ([]color.NRGBA, error) {
	if len(raw) < 32 {
		return nil, fmt.Errorf("exchange: incomplete source palette")
	}
	colors := make([]color.NRGBA, 16)
	for i := range colors {
		word := binary.BigEndian.Uint16(raw[i*2:])
		colors[i] = color.NRGBA{R: byte(word>>8&7) * 34, G: byte(word>>4&7) * 34, B: byte(word&7) * 34, A: 255}
	}
	colors[0].A = 0
	return colors, nil
}

func Palette(raw []byte) (*composite.IndexedPalette, error) {
	colors, err := Colors(raw)
	if err != nil {
		return nil, err
	}
	return composite.NewIndexedPalette(composite.IndexedPaletteConfig{Palette: colors, Channel: composite.BitplaneRed, SourceAlpha: true, Blend: ebiten.BlendCopy})
}

// Bake caches original indexed pixels through DCK once, retaining transparent
// cutouts. The caller owns the result and the supplied palette remains borrowed.
func Bake(source *ebiten.Image, palette *composite.IndexedPalette) (*ebiten.Image, error) {
	image := render.NewSurface(source.Bounds().Dx(), source.Bounds().Dy())
	if err := palette.Draw(image, source); err != nil {
		image.Deallocate()
		return nil, err
	}
	return image, nil
}

func DrawImage(dst, source *ebiten.Image, x, y float64, parent ebiten.GeoM) {
	var options ebiten.DrawImageOptions
	options.Filter = ebiten.FilterLinear
	options.GeoM.Translate(x, y)
	options.GeoM.Concat(parent)
	dst.DrawImage(source, &options)
}
