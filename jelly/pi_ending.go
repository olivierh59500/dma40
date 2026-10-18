package jelly

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/sprites"
	"github.com/olivierh59500/dma40/internal/decipher"
	"github.com/olivierh59500/dma40/internal/pitext"
)

const piAlphabet = decipher.Alphabet + "."

// piStrokes is an authored Matrix-style lowercase pi, drawn as pixel geometry.
var piStrokes = [...]uint16{0, 0b011111110, 0b001000100, 0b001000100, 0b001000100, 0b001000100, 0b010000100, 0b010000110, 0}

type piEnding struct {
	white, texture *ebiten.Image
	batch          *render.Batch
	renderer       *sprites.FieldRenderer
	style          sprites.FieldStyle
	samples        []sprites.FieldSample
	letters        [piCharacters]decipher.Cell
	text           string
	path           motion.Curve
}

func newPiEnding(font image.Image) (_ *piEnding, err error) {
	p := &piEnding{white: ebiten.NewImage(1, 1), texture: ebiten.NewImageFromImage(font), batch: render.NewBatch(1024),
		renderer: sprites.NewFieldRenderer(piCharacters), samples: make([]sprites.FieldSample, 0, piCharacters), text: pitext.Digits(piCharacters - 2)}
	defer func() {
		if err != nil {
			p.close()
		}
	}()
	p.white.Fill(color.White)
	grid, err := presets.BitmapFont("cuddly-loader", p.texture, ebiten.FilterNearest)
	if err != nil {
		return nil, err
	}
	frames := make([]image.Rectangle, 0, len(piAlphabet))
	for _, ch := range piAlphabet {
		r, ok := grid.Region(ch)
		rect := image.Rect(int(r.X), int(r.Y), int(r.X+r.Width), int(r.Y+r.Height))
		if !ok || rect.Empty() || !rect.In(p.texture.Bounds()) {
			return nil, fmt.Errorf("pi page font has no glyph %q", ch)
		}
		frames = append(frames, rect)
	}
	p.style = sprites.FieldStyle{Image: p.texture, Frames: frames, Filter: ebiten.FilterNearest,
		Appearance: sprites.FieldAppearance{Width: piCell, Height: piCell},
		Sample: func(s sprites.FieldSample, a *sprites.FieldAppearance) bool {
			if p.letters[s.Index].Settled {
				a.Tint.Scale(.18, 1, .35, 1)
			} else {
				a.Tint.Scale(.08, .55, .2, 1)
			}
			return true
		},
	}
	p.path = motion.BezierCurve(motion.Point{X: 132, Y: 298}, motion.Point{X: 260, Y: 336}, motion.Point{X: 560, Y: 345}, motion.Point{X: piClickX, Y: piClickY})
	return p, nil
}

func (p *piEnding) draw(dst *ebiten.Image, seconds float64) {
	if seconds >= PiPageAt {
		p.drawPage(dst, seconds)
		return
	}
	p.batch.Begin(dst, p.white)
	alpha := motion.Smooth((seconds - PiAt) / .4)
	pulse := .84 + .16*math.Sin((seconds-PiAt)*11)
	for row, bits := range piStrokes {
		for col := 0; col < 9; col++ {
			if bits&(1<<uint(8-col)) == 0 {
				continue
			}
			x, y := float64(piX+col*2), float64(piY+row*2)
			p.batch.Rect(x-1, y-1, 4, 4, image.Rect(0, 0, 1, 1), color.NRGBA{R: 8, G: 80, B: 30, A: uint8(alpha * 95)})
			p.batch.Rect(x, y, 2, 2, image.Rect(0, 0, 1, 1), color.NRGBA{R: 90, G: 255, B: 135, A: uint8(alpha * pulse * 255)})
		}
	}
	if seconds >= PiCursorAt {
		position := p.path(motion.Smooth((seconds - PiCursorAt) / (PiClickAt - PiCursorAt)))
		press := math.Sin(math.Pi * min(1, max(0, (seconds-PiClickAt)/.2)))
		p.drawCursor(position.X, position.Y+press*1.5)
	}
	if seconds >= PiClickAt {
		u := min(1, (seconds-PiClickAt)/(PiPageAt-PiClickAt))
		var ring [16]geometry.Vec2
		for i := range ring {
			angle := float64(i) * 2 * math.Pi / float64(len(ring))
			ring[i] = geometry.Vec2{X: piClickX + math.Cos(angle)*(3+u*12), Y: piClickY + math.Sin(angle)*(3+u*12)}
		}
		p.batch.StrokePath(ring[:], render.PathStroke{Width: 1.2}, color.NRGBA{R: 100, G: 255, B: 150, A: uint8((1 - u) * 200)})
	}
	p.batch.Flush()
}

func (p *piEnding) drawCursor(x, y float64) {
	triangle := func(points [3][2]float64, paint color.Color) {
		p.batch.Triangle(render.Vertex(x+points[0][0], y+points[0][1], 0, 0, paint), render.Vertex(x+points[1][0], y+points[1][1], 0, 0, paint), render.Vertex(x+points[2][0], y+points[2][1], 0, 0, paint))
	}
	quad := func(points [4][2]float64, paint color.Color) {
		var vertices [4]ebiten.Vertex
		for i, v := range points {
			vertices[i] = render.Vertex(x+v[0], y+v[1], 0, 0, paint)
		}
		p.batch.Quad(vertices)
	}
	triangle([3][2]float64{{0, 0}, {0, 19}, {14, 13}}, color.Black)
	quad([4][2]float64{{4, 13}, {8, 12}, {12, 21}, {8, 23}}, color.Black)
	triangle([3][2]float64{{2, 4}, {2, 15}, {10, 11}}, color.White)
	quad([4][2]float64{{5, 14}, {7, 13}, {10, 20}, {8, 21}}, color.White)
}

func (p *piEnding) drawPage(dst *ebiten.Image, seconds float64) {
	dst.Fill(color.Black)
	p.samples = p.samples[:0]
	for i, ch := range p.text {
		letter := decipher.Sample(piAlphabet, ch, piSeed+i, seconds-PiPageAt-float64(i)*piStagger)
		p.letters[i] = letter
		if letter.Active && letter.Glyph >= 0 {
			p.samples = append(p.samples, sprites.FieldSample{Index: i, Image: letter.Glyph, X: float64(i % piColumns * piCell), Y: float64(i / piColumns * piCell), Scale: 1})
		}
	}
	p.renderer.Draw(dst, p.samples, p.style)
}

func (p *piEnding) close() {
	if p == nil {
		return
	}
	if p.renderer != nil {
		p.renderer.Close()
	}
	if p.texture != nil {
		p.texture.Deallocate()
	}
	if p.white != nil {
		p.white.Deallocate()
	}
}
