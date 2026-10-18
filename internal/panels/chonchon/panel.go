// Adapted from go-chonchon's native Go/DCK renderer for the DMA40 four-panel part.
// Package chonchon composes Chonchon's native bitmap motion with DCK.
package chonchon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/png"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/sprites"
	"github.com/olivierh59500/dma40/internal/exchange"
)

const Width, Height, FPS = 320, 200, 50

type Panel struct {
	files              fs.FS
	clock              *Clock
	image, stage, logo *ebiten.Image
	lookup             *composite.IndexedPalette
	slots              *sprites.ImageSlots
	pixels             []byte
	volumes            [3]byte
	shader             *ebiten.Shader
	rasters            [28 * 4]float32
	uniforms           map[string]any
	closed             bool
	curve, meterRaw    []byte
	logoAway           bool
	sharedAtlas        *ebiten.Image
	sharedPalette      *composite.IndexedPalette
	sharedBatch        *render.Batch
}

func (g *Panel) read(n string) ([]byte, error) { return fs.ReadFile(g.files, n) }
func New(files fs.FS) (_ *Panel, err error) {
	g := &Panel{files: files, pixels: make([]byte, Width*Height*4)}
	defer func() {
		if err != nil {
			g.Close()
		}
	}()
	read := func(n string) []byte {
		if err != nil {
			return nil
		}
		var b []byte
		b, err = g.read(n)
		return b
	}
	bg, font, path, ys, wave, msg, stars, index := read("background.bin"), read("font-source.bin"), read("logo-path.bin"), read("logo-y.bin"), read("scroll-y.bin"), read("message.bin"), read("star-paths.bin"), read("star-index.bin")
	if err != nil {
		return nil, err
	}
	g.clock, err = NewClock(bg, font, path, ys, wave, msg, stars, index)
	if err != nil {
		return nil, err
	}
	load := func(n string) (*ebiten.Image, error) {
		b, e := g.read(n)
		if e != nil {
			return nil, e
		}
		im, _, e := image.Decode(bytes.NewReader(b))
		if e != nil {
			return nil, e
		}
		return ebiten.NewImageFromImage(im), nil
	}
	g.logo, err = load("logo-bank.png")
	if err != nil {
		return nil, err
	}
	g.image = ebiten.NewImage(Width, Height)
	g.stage = ebiten.NewImage(Width, Height)
	g.slots, err = sprites.NewImageSlots(sprites.ImageSlotsConfig{Images: []*ebiten.Image{g.logo}, MaxSlots: 63})
	if err != nil {
		return nil, err
	}
	pal := read("palette.bin")
	if err != nil {
		return nil, err
	}
	colors := make([]color.NRGBA, 16)
	for i := range colors {
		w := binary.BigEndian.Uint16(pal[i*2:])
		colors[i] = color.NRGBA{R: byte(w>>8&7) * 34, G: byte(w>>4&7) * 34, B: byte(w&7) * 34, A: 255}
	}
	g.lookup, err = composite.NewIndexedPalette(composite.IndexedPaletteConfig{Palette: colors, Channel: composite.BitplaneRed, Blend: ebiten.BlendCopy})
	if err != nil {
		return nil, err
	}
	g.curve = read("volume-curve.bin")
	g.meterRaw = read("meter-raw.bin")
	raster := read("raster.bin")
	if err != nil {
		return nil, err
	}
	for i := 0; i < 28; i++ {
		w := binary.BigEndian.Uint16(raster[i*2:])
		g.rasters[i*4], g.rasters[i*4+1], g.rasters[i*4+2], g.rasters[i*4+3] = float32(w>>8&7)*34/255, float32(w>>4&7)*34/255, float32(w&7)*34/255, 1
	}
	g.shader, err = ebiten.NewShader([]byte(meterShader))
	if err != nil {
		return nil, err
	}
	g.uniforms = map[string]any{"Rasters": g.rasters[:]}
	if err = g.Update([3]byte{}); err != nil {
		return nil, err
	}
	return g, nil
}
func (g *Panel) Update(levels [3]byte) error {
	g.clock.Step()
	g.volumes = levels
	curve := g.curve
	for channel := 0; channel < 3; channel++ {
		height := int(curve[g.volumes[channel]])
		if height == 0 {
			continue
		}
		x := 96
		if channel == 1 {
			x = 144
		} else if channel == 2 {
			x = 176
		}
		width := 48
		if channel == 1 {
			width = 32
		}
		sourceX := x - 96
		for row := 0; row <= height && row < 52; row++ {
			for dx := 0; dx < width; dx++ {
				p := (147+row)*160 + (x+dx)/16*8 + 6
				mask := uint16(1 << uint(15-(x+dx)%16))
				w := binary.BigEndian.Uint16(g.clock.Screen[p:])
				q := sourceX + dx
				art := g.meterRaw
				v := binary.BigEndian.Uint16(art[row*160+q/16*8:])
				if v>>uint(15-q%16)&1 != 0 {
					w |= mask
				}
				binary.BigEndian.PutUint16(g.clock.Screen[p:], w)
			}
		}
	}
	g.clock.Pixels(g.pixels)
	g.image.WritePixels(g.pixels)
	g.stage.DrawImage(g.image, &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy})
	var slots [63]sprites.ImageSlot
	for row := range slots {
		x, phase, y := g.clock.LogoRow(row)
		slots[row] = sprites.ImageSlot{Image: 0, Source: image.Rect(0, row*17+phase, 96, row*17+phase+1), X: float64(x), Y: float64(y)}
	}
	if e := g.slots.SetSlots(slots[:]); e != nil {
		return e
	}
	if !g.logoAway {
		g.slots.Draw(g.stage)
	}
	return nil
}

func (g *Panel) ExchangeObjects() ([]exchange.Object, error) {
	raw, err := g.read("palette.bin")
	if err != nil {
		return nil, err
	}
	g.sharedPalette, err = exchange.Palette(raw)
	if err != nil {
		return nil, err
	}
	g.sharedAtlas, err = exchange.Bake(g.logo, g.sharedPalette)
	if err != nil {
		return nil, err
	}
	g.sharedBatch = render.NewBatch(126)
	return []exchange.Object{{Name: "Chonchon wave logo", SetAway: func(away bool) { g.logoAway = away }, Draw: g.drawSharedLogo}}, nil
}

func (g *Panel) drawSharedLogo(dst *ebiten.Image, parent ebiten.GeoM) {
	g.sharedBatch.Begin(dst, g.sharedAtlas)
	for row := 0; row < 63; row++ {
		x, phase, y := g.clock.LogoRow(row)
		sourceY := row*17 + phase
		var vertices [4]ebiten.Vertex
		for i, corner := range [4][2]int{{0, 0}, {96, 0}, {96, 1}, {0, 1}} {
			xx, yy := parent.Apply(float64(x+corner[0]), float64(y+corner[1]))
			vertices[i] = ebiten.Vertex{DstX: float32(xx), DstY: float32(yy), SrcX: float32(corner[0]), SrcY: float32(sourceY + corner[1]), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}
		}
		g.sharedBatch.Quad(vertices)
	}
	g.sharedBatch.Flush()
}
func (g *Panel) Draw(dst *ebiten.Image) {
	_ = g.lookup.Draw(g.image, g.stage)
	op := ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{g.stage, g.image}, Uniforms: g.uniforms, Blend: ebiten.BlendCopy}
	dst.DrawRectShader(Width, Height, g.shader, &op)
}
func (*Panel) Layout(int, int) (int, int) { return Width, Height }
func (g *Panel) Tick() int                { return g.clock.Tick }
func (g *Panel) Close() {
	if g == nil || g.closed {
		return
	}
	g.closed = true
	if g.sharedAtlas != nil {
		g.sharedAtlas.Deallocate()
	}
	if g.sharedPalette != nil {
		g.sharedPalette.Close()
	}

	if g.slots != nil {
		g.slots.Close()
	}
	if g.lookup != nil {
		g.lookup.Close()
	}
	if g.shader != nil {
		g.shader.Deallocate()
	}
	for _, im := range []*ebiten.Image{g.image, g.stage, g.logo} {
		if im != nil {
			im.Deallocate()
		}
	}
}

const meterShader = `//kage:unit pixels
package main
var Rasters [28]vec4
func Fragment(position vec4,source vec2,color vec4)vec4{p:=source-imageSrc0Origin();i:=int(clamp(floor(imageSrc0At(source).r*15+0.5),0,15));if i>=8 && p.y>=144 && p.y<200{return Rasters[int(clamp(floor((p.y-144)/2),0,27))]*color};return imageSrc1At(source)*color}
`
