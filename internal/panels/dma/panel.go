// Package dma adapts DMA is Back's active DCK scene without its separate intro.
package dma

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/dma40/internal/exchange"
)

const Width, Height, FPS = 640, 400, 60
const text = "          WELCOME TO THE \"DMA-IS-BACK\" DEMO BY BILIZIR, WRITTEN IN GOLANG + EBITEN.     GREETINGS TO ALL DEMOSCENE LOVERS AND ATARI ST FANS!     LET'S WRAP...          "

type Panel struct {
	cube        *effects.JellyCube
	scroll      *scrolling.Scrolling
	logos       composite.ImageGrid
	path        *motion.TrajectoryClock
	back, font  *ebiten.Image
	canvas      *ebiten.Image
	tick        int
	cubeAway    bool
	sharedWhite *ebiten.Image
	sharedBatch *render.Batch
}

func New(files fs.FS) (_ *Panel, err error) {
	p := &Panel{}
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	load := func(name string) (*ebiten.Image, error) {
		data, e := fs.ReadFile(files, name)
		if e != nil {
			return nil, e
		}
		im, _, e := image.Decode(bytes.NewReader(data))
		if e != nil {
			return nil, e
		}
		return ebiten.NewImageFromImage(im), nil
	}
	p.back, err = load("back.png")
	if err != nil {
		return nil, err
	}
	p.font, err = load("font.png")
	if err != nil {
		return nil, err
	}
	atlas, err := presets.FontAtlas("dma-is-back", p.font)
	if err != nil {
		return nil, err
	}
	config, err := presets.DMAScanlineScroll(atlas, text)
	if err != nil {
		return nil, err
	}
	p.scroll, err = scrolling.New(scrolling.Config{Scanline: &config})
	if err != nil {
		return nil, err
	}
	cube := effects.DMAJellyCubeConfig()
	cube.Delay, cube.ZoomDuration = 0, 0
	p.cube, err = effects.NewJellyCube(cube)
	if err != nil {
		return nil, err
	}
	p.logos = presets.DMALogoGrid()
	orbit := motion.DefaultNestedOrbit(motion.Point{X: Width / 2, Y: Height / 2}, motion.Point{X: Width / 4, Y: Height / 2.7})
	p.path, err = motion.NewTrajectoryClock(motion.TrajectoryClockConfig{Sample: orbit.At, Step: .014})
	if err != nil {
		return nil, err
	}
	p.canvas = ebiten.NewImage(Width, Height)
	p.canvas.Fill(color.Black)
	if err = p.scroll.Update(kit.Frame{}); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *Panel) Update(_ [3]byte) error {
	p.tick++
	if err := p.path.Step(); err != nil {
		return err
	}
	if err := p.scroll.Update(kit.Frame{Tick: uint64(p.tick)}); err != nil {
		return err
	}
	return p.cube.Update(kit.Frame{Time: float64(p.tick) / FPS, Tick: uint64(p.tick)})
}
func (p *Panel) Draw(dst *ebiten.Image) {
	// The native main scene retains its text surface between frames.
	p.scroll.Draw(p.canvas)
	point := p.path.At()
	p.logos.DrawCentered(p.canvas, p.back, point.X, point.Y)
	if !p.cubeAway {
		p.cube.Draw(p.canvas)
	}
	dst.DrawImage(p.canvas, nil)
}

func (p *Panel) ExchangeObjects() ([]exchange.Object, error) {
	p.sharedWhite = ebiten.NewImage(1, 1)
	p.sharedWhite.Fill(color.White)
	p.sharedBatch = render.NewBatch(12)
	return []exchange.Object{{Name: "DMA jelly cube", SetAway: func(away bool) { p.cubeAway = away }, Draw: p.drawSharedCube}}, nil
}

func (p *Panel) drawSharedCube(dst *ebiten.Image, parent ebiten.GeoM) {
	// DCK retains deformation, perspective, depth order and original colors.
	// Only its prepared screen-space vertices move into the common foreground.
	vertices, indices := p.cube.Geometry()
	var transformed [24]ebiten.Vertex
	for i, vertex := range vertices {
		x, y := parent.Apply(float64(vertex.DstX), float64(vertex.DstY))
		vertex.DstX, vertex.DstY = float32(x), float32(y)
		transformed[i] = vertex
	}
	p.sharedBatch.Begin(dst, p.sharedWhite)
	for i := 0; i < len(indices); i += 3 {
		p.sharedBatch.Triangle(transformed[indices[i]], transformed[indices[i+1]], transformed[indices[i+2]])
	}
	p.sharedBatch.Flush()
}
func (p *Panel) Close() {
	if p.sharedWhite != nil {
		p.sharedWhite.Deallocate()
		p.sharedWhite = nil
	}
	if p.cube != nil {
		p.cube.Close()
	}
	if p.scroll != nil {
		p.scroll.Close()
	}
	for _, im := range []*ebiten.Image{p.back, p.font, p.canvas} {
		if im != nil {
			im.Deallocate()
		}
	}
}
