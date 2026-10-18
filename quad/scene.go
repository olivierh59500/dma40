// Package quad keeps four complete demo renderers visible in fixed quadrants.
package quad

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"io/fs"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/sound"
	"github.com/olivierh59500/dma40/internal/displayclock"
	"github.com/olivierh59500/dma40/internal/exchange"
	"github.com/olivierh59500/dma40/internal/panels/chonchon"
	"github.com/olivierh59500/dma40/internal/panels/dma"
	"github.com/olivierh59500/dma40/internal/panels/lockbuster"
	"github.com/olivierh59500/dma40/internal/panels/rastermaster"
	"github.com/olivierh59500/dma40/internal/partassets"
)

const Width, Height, FPS = 640, 400, 50
const ID = "four"
const Title = "FOUR DEMOS"

// Regions is deliberately static: every panel occupies its full quarter at
// entry, during playback and on return. There is no tour or camera controller.
func Regions() [4]image.Rectangle {
	return [4]image.Rectangle{image.Rect(0, 0, 320, 200), image.Rect(320, 0, 640, 200), image.Rect(0, 200, 320, 400), image.Rect(320, 200, 640, 400)}
}

type nativePanel interface {
	Update([3]byte) error
	Draw(*ebiten.Image)
	Close()
}
type adapter struct {
	nativePanel
	rate, tick int
	levels     *[3]byte
}

func (p *adapter) Update(frame kit.Frame) error {
	target := int(frame.Tick) * p.rate / FPS
	for p.tick < target {
		if err := p.nativePanel.Update(*p.levels); err != nil {
			return err
		}
		p.tick++
	}
	return nil
}
func (p *adapter) Close() error { p.nativePanel.Close(); return nil }

type Scene struct {
	group    kit.Group
	visual   *sound.Stream
	pcm      [960 * 8]byte
	levels   [3]byte
	tick     uint64
	visitors []visitor
	clock    displayclock.Clock
}

// New uses the native source clocks and artwork; DCK owns normalized surfaces,
// viewport composition, bitmap text, palettes, sprites and cube deformation.
func New(media *partassets.Set) (_ *Scene, err error) {
	return newScene(media, true)
}

func newScene(media *partassets.Set, crossings bool) (_ *Scene, err error) {
	s := &Scene{}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	s.visual, err = sound.Open("music.ym", media.Music, sound.Options{SampleRate: 48000, BlockFrames: 960, Loop: true})
	if err != nil {
		return nil, err
	}
	factories := []struct {
		prefix              string
		width, height, rate int
		new                 func(fs.FS) (nativePanel, error)
	}{
		{"lockbuster", lockbuster.Width, lockbuster.Height, lockbuster.FPS, func(f fs.FS) (nativePanel, error) { return lockbuster.New(f) }},
		{"rastermaster", rastermaster.Width, rastermaster.Height, rastermaster.FPS, func(f fs.FS) (nativePanel, error) { return rastermaster.New(f) }},
		{"chonchon", chonchon.Width, chonchon.Height, chonchon.FPS, func(f fs.FS) (nativePanel, error) { return chonchon.New(f) }},
		{"dma", dma.Width, dma.Height, dma.FPS, func(f fs.FS) (nativePanel, error) { return dma.New(f) }},
	}
	regions := Regions()
	for index, factory := range factories {
		files, e := fs.Sub(media.Files, factory.prefix)
		if e != nil {
			return nil, e
		}
		panel, e := factory.new(files)
		if e != nil {
			return nil, fmt.Errorf("%s panel: %w", factory.prefix, e)
		}
		controller := &adapter{nativePanel: panel, rate: factory.rate, levels: &s.levels}
		normalized, e := effects.NewWarp(controller, factory.width, factory.height, 1, 1)
		if e != nil {
			panel.Close()
			return nil, e
		}
		width, height := float64(factory.width), float64(factory.height)
		// Normalize each full source canvas to the common 320x200 quadrant.
		// This deliberately adapts Lockbuster's 320x240 canvas without cropping.
		normalized.Map = func(x, y, _ float64) geometry.Vec2 { return geometry.Vec2{X: x * 320 / width, Y: y * 200 / height} }
		viewport, e := kit.NewViewport(normalized, 320, 200, regions[index])
		if e != nil {
			normalized.Close()
			return nil, e
		}
		viewport.Background = color.Black
		s.group = append(s.group, viewport)
		if crossings {
			provider, ok := panel.(exchange.Provider)
			if !ok {
				return nil, fmt.Errorf("%s panel has no exchange objects", factory.prefix)
			}
			objects, e := provider.ExchangeObjects()
			if e != nil {
				return nil, e
			}
			for _, object := range objects {
				s.visitors = append(s.visitors, newVisitor(object, index, factory.width, factory.height, len(s.visitors)))
			}
		}
	}
	if err = s.group.Update(kit.Frame{}); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Scene) Update() error {
	if _, err := io.ReadFull(s.visual, s.pcm[:]); err != nil {
		return err
	}
	if registers, ok := s.visual.YMRegisters(); ok {
		for i := range s.levels {
			s.levels[i] = registers[8+i] & 15
		}
	}
	s.tick++
	for i := range s.visitors {
		s.visitors[i].update(float64(s.tick) / FPS)
	}
	return s.group.Update(kit.Frame{Tick: s.tick, Time: float64(s.tick) / FPS, Delta: 1.0 / FPS})
}
func (s *Scene) Draw(dst *ebiten.Image) { s.draw(dst, float64(s.tick)/FPS) }

func (s *Scene) DrawLive(dst *ebiten.Image, now time.Time, paused bool) {
	fixed := float64(s.tick) / FPS
	var seconds float64
	if paused {
		seconds = s.clock.Hold(now, fixed)
	} else {
		seconds = s.clock.Sample(now, fixed)
	}
	s.draw(dst, seconds)
}

func (s *Scene) draw(dst *ebiten.Image, seconds float64) {
	dst.Fill(color.Black)
	s.group.Draw(dst)
	for i := range s.visitors {
		s.visitors[i].draw(dst, seconds)
	}
}
func (s *Scene) Tick() uint64 { return s.tick }
func (s *Scene) Close() {
	s.group.Close()
	s.group = nil
	s.visitors = nil
	if s.visual != nil {
		s.visual.Close()
		s.visual = nil
	}
}
