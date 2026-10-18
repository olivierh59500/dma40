// Adapted from go-lockbuster3's native Go/DCK renderer for the DMA40 four-panel part.
// Package lockbuster composes the native intro's clocks and artwork through DCK.
package lockbuster

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sprites"
	"github.com/olivierh59500/dma40/internal/exchange"
)

const Width, Height, FPS = 320, 240, 50

type Panel struct {
	files            fs.FS
	images           []*ebiten.Image
	buffers          [2]*ebiten.Image
	background, text *ebiten.Image
	font             [2]*ebiten.Image
	initial          *ebiten.Image
	strips           [2][256][4]*ebiten.Image
	initialStrips    [2][40]*ebiten.Image
	letters          [3]Letter
	history          [2][3]image.Rectangle
	panel            *PanelClock
	clock            *ScrollClock
	scrolls          [2]*scrolling.Scrolling
	blitter          *sprites.ImageSlots
	shader           *ebiten.Shader
	uniforms         map[string]any
	palette, rasters []float32
	volumes          [3]byte
	tick, front      int
	closed           bool
	away             [3]bool
	sharedLetters    [3]*ebiten.Image
	sharedPalette    *composite.IndexedPalette
}

func (g *Panel) read(name string) ([]byte, error) { return fs.ReadFile(g.files, name) }

func New(files fs.FS) (_ *Panel, err error) {
	g := &Panel{front: 1, files: files}
	defer func() {
		if err != nil {
			g.Close()
		}
	}()
	load := func(name string) (*ebiten.Image, image.Image, error) {
		data, err := g.read(name)
		if err != nil {
			return nil, nil, err
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, nil, err
		}
		texture := ebiten.NewImageFromImage(img)
		g.images = append(g.images, texture)
		return texture, img, nil
	}
	_, art, err := load("artwork.png")
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"panels-0.png", "panels-1.png", "panels-2.png", "letter-d.png", "letter-m.png", "letter-a.png"} {
		if _, _, err := load(name); err != nil {
			return nil, err
		}
	}
	for i := range g.font {
		if g.font[i], _, err = load(fmt.Sprintf("font-%d.png", i)); err != nil {
			return nil, err
		}
	}
	g.background = ebiten.NewImage(Width, Height)
	g.background.Fill(color.NRGBA{A: 255})
	g.images = append(g.images, g.background)
	g.blitter, err = sprites.NewImageSlots(sprites.ImageSlotsConfig{Images: g.images, MaxSlots: 8, Blend: ebiten.BlendSourceOver})
	if err != nil {
		return nil, err
	}
	var initial [5]sprites.ImageSlot
	for y := range initial {
		initial[y] = sprites.ImageSlot{Image: 0, Source: image.Rect(0, 35, 320, 75), Y: float64(y * 40)}
	}
	if err := g.blitter.SetSlots(initial[:]); err != nil {
		return nil, err
	}
	g.blitter.Draw(g.background)
	motion, err := g.read("motion.bin")
	if err != nil {
		return nil, err
	}
	g.letters = ReadLetters(motion)
	for i := range g.buffers {
		g.buffers[i] = ebiten.NewImage(Width, Height)
		g.buffers[i].DrawImage(g.background, nil)
		for j := range g.letters {
			g.letters[j].Step()
		}
		if err := g.drawLetters(i); err != nil {
			return nil, err
		}
	}
	order, err := g.read("panel-order.bin")
	if err != nil {
		return nil, err
	}
	g.panel = NewPanelClock(order)
	message, err := g.read("message.bin")
	if err != nil {
		return nil, err
	}
	g.clock = NewScrollClock(message)
	mono := image.NewNRGBA(image.Rect(0, 0, 320, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 320; x++ {
			r, _, _, _ := art.At(x, 35+y).RGBA()
			bit := byte(r>>8) / 17 & 1
			mono.SetNRGBA(x, y, color.NRGBA{R: bit * 255, A: 255})
		}
	}
	g.initial = ebiten.NewImageFromImage(mono)
	for row := range g.initialStrips {
		for slot := range g.initialStrips[row] {
			g.initialStrips[row][slot] = g.initial.SubImage(image.Rect(slot*8, row*20, slot*8+8, row*20+20)).(*ebiten.Image)
		}
	}
	for bank := range g.strips {
		for glyph := range g.strips[bank] {
			for column := range g.strips[bank][glyph] {
				x, y := glyph%16*32+column*8, glyph/16*20
				g.strips[bank][glyph][column] = g.font[bank].SubImage(image.Rect(x, y, x+8, y+20)).(*ebiten.Image)
			}
		}
	}
	g.text = ebiten.NewImage(Width, Height)
	for row := range g.scrolls {
		lane := row
		g.scrolls[row], err = scrolling.New(scrolling.Config{Y: float64(160 + row*20), GlyphWindow: &scrolling.GlyphWindowConfig{Count: 40, Advance: 8,
			Glyph: func(slot int) scrolling.Glyph {
				c := g.clock.Columns[lane][slot]
				var img *ebiten.Image
				if c.Initial != 0 {
					img = g.initialStrips[c.Row][c.Slot]
				} else {
					img = g.strips[c.Font][c.Glyph][c.Column]
				}
				return scrolling.Glyph{Image: img, Advance: 8}
			},
		}})
		if err != nil {
			return nil, err
		}
	}
	g.palette = make([]float32, 16*4)
	pal, err := g.read("palette.bin")
	if err != nil {
		return nil, err
	}
	for i := range 16 {
		putRGB(g.palette[i*4:], binary.BigEndian.Uint16(pal[i*2:]))
	}
	data, err := g.read("rasters.bin")
	if err != nil {
		return nil, err
	}
	g.rasters = make([]float32, 77*4)
	count := 0
	for i := 0; i+3 < len(data); i += 2 {
		if binary.BigEndian.Uint16(data[i:]) == 0 {
			break
		}
		putRGB(g.rasters[count*4:], binary.BigEndian.Uint16(data[i+2:]))
		count++
	}
	g.shader, err = ebiten.NewShader([]byte(nativePaletteShader))
	if err != nil {
		return nil, err
	}
	g.uniforms = map[string]any{"Palette": g.palette, "Rasters": g.rasters, "RasterRange": []float32{159, float32(count)}}
	g.drawText()
	return g, nil
}

func putRGB(dst []float32, word uint16) {
	// The ST uses the even levels of the STE's four-bit DAC: 0,34,...,238.
	dst[0], dst[1], dst[2], dst[3] = float32(word>>8&7)*34/255, float32(word>>4&7)*34/255, float32(word&7)*34/255, 1
}
func (g *Panel) drawLetters(target int) error {
	var slots [3]sprites.ImageSlot
	count := 0
	for i, l := range g.letters {
		x, y := int(l.Words[0]), int(l.Words[1])
		// Keep the original background-restoration trace while a letter is
		// abroad, so rejoining cannot leave stale panel transitions behind.
		g.history[target][i] = image.Rect(x&^15, y, (x&^15)+64, y+35)
		if g.away[i] {
			continue
		}
		slots[count] = sprites.ImageSlot{Image: 4 + i, X: float64(x), Y: float64(y)}
		count++
	}
	if err := g.blitter.SetSlots(slots[:count]); err != nil {
		return err
	}
	g.blitter.Draw(g.buffers[target])
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
	objects := make([]exchange.Object, 3)
	for i := range objects {
		g.sharedLetters[i], err = exchange.Bake(g.images[4+i], g.sharedPalette)
		if err != nil {
			return nil, err
		}
		objects[i] = exchange.Object{Name: []string{"Lockbuster D", "Lockbuster M", "Lockbuster A"}[i],
			SetAway: func(away bool) { g.away[i] = away },
			Draw: func(dst *ebiten.Image, parent ebiten.GeoM) {
				letter := g.letters[i].Words
				exchange.DrawImage(dst, g.sharedLetters[i], float64(letter[0]), float64(letter[1]), parent)
			}}
	}
	return objects, nil
}

func panelRect(frame int) image.Rectangle {
	x, y := frame%5*64, frame/5*40
	return image.Rect(x, y, x+64, y+40)
}
func (g *Panel) Update(levels [3]byte) error {
	g.tick++
	g.volumes = levels
	target := 1 - g.front
	var restore [3]sprites.ImageSlot
	for i, r := range g.history[target] {
		restore[i] = sprites.ImageSlot{Image: len(g.images) - 1, Source: r, X: float64(r.Min.X), Y: float64(r.Min.Y), Hidden: r.Empty()}
	}
	if err := g.blitter.SetSlots(restore[:]); err != nil {
		return err
	}
	g.blitter.Draw(g.buffers[target])
	g.panel.Step()
	if g.panel.Draw {
		p := g.panel.Pose
		x, y := p.Cell%5*64, p.Cell/5*40
		slot := sprites.ImageSlot{Image: 1 + p.Bank, Source: panelRect(p.Frame), X: float64(x), Y: float64(y)}
		if err := g.blitter.SetSlots([]sprites.ImageSlot{slot}); err != nil {
			return err
		}
		g.blitter.Draw(g.buffers[target])
		if g.panel.Commit {
			slot.Image = 1 + g.panel.Base/25600
			slot.Source = panelRect(19)
			if err := g.blitter.SetSlots([]sprites.ImageSlot{slot}); err != nil {
				return err
			}
			g.blitter.Draw(g.background)
		}
	}
	for i := range g.letters {
		g.letters[i].Step()
	}
	if err := g.drawLetters(target); err != nil {
		return err
	}
	g.clock.Step()
	f := kit.Frame{Tick: uint64(g.tick), Time: float64(g.tick) / FPS, Delta: 1.0 / FPS}
	for _, s := range g.scrolls {
		if err := s.Update(f); err != nil {
			return err
		}
	}
	g.drawText()
	var bars [3]sprites.ImageSlot
	for i, v := range g.volumes {
		x, y := int(v)*32, 75
		if v >= 10 {
			x, y = int(v-10)*32, 100
		}
		bars[i] = sprites.ImageSlot{Image: 0, Source: image.Rect(x, y, x+32, y+25), X: float64(80 + i*64), Y: 210}
	}
	if err := g.blitter.SetSlots(bars[:]); err != nil {
		return err
	}
	g.blitter.Draw(g.buffers[target])
	g.front = target
	return nil
}
func (g *Panel) drawText() {
	g.text.Clear()
	for row, s := range g.scrolls {
		state := scrolling.IdentityState()
		state.End, state.Y = 40, float64(160+row*20)
		state.Time = float64(g.tick) / FPS
		s.DrawAt(g.text, state)
	}
}
func (g *Panel) Draw(dst *ebiten.Image) {
	op := ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{g.buffers[g.front], g.text}, Uniforms: g.uniforms}
	dst.DrawRectShader(Width, Height, g.shader, &op)
}
func (*Panel) Layout(int, int) (int, int) { return Width, Height }
func (g *Panel) Tick() int                { return g.tick }
func (g *Panel) Close() {
	if g == nil || g.closed {
		return
	}
	g.closed = true
	for _, image := range g.sharedLetters {
		if image != nil {
			image.Deallocate()
		}
	}
	if g.sharedPalette != nil {
		g.sharedPalette.Close()
	}

	for _, s := range g.scrolls {
		if s != nil {
			s.Close()
		}
	}
	if g.blitter != nil {
		g.blitter.Close()
	}
	if g.shader != nil {
		g.shader.Deallocate()
	}
	for _, img := range g.images {
		img.Deallocate()
	}
	for _, img := range g.buffers {
		if img != nil {
			img.Deallocate()
		}
	}
	if g.initial != nil {
		g.initial.Deallocate()
	}
	if g.text != nil {
		g.text.Deallocate()
	}
}

// Two reserved native color registers change per scanline. Text replaces only
// bitplane zero in its two rows; the panel colors underneath remain intact.
const nativePaletteShader = `//kage:unit pixels
package main
var Palette [16]vec4
var Rasters [77]vec4
var RasterRange vec2
func Fragment(position vec4, source vec2, color vec4) vec4 {
 p:=source-imageSrc0Origin()
 index:=int(clamp(floor(imageSrc0At(source).r*15+0.5),0,15))
 if p.y>=160 && p.y<200 {
  bit:=int(floor(imageSrc1At(source).r+0.5))
  index=index-index%2+bit
 }
 if (index==12 || index==14) && p.y>=RasterRange.x && p.y<RasterRange.x+RasterRange.y {
  return Rasters[int(p.y-RasterRange.x)]*color
 }
 return Palette[index]*color
}
`
