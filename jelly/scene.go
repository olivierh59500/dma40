// Package jelly composes the DMA glass logo and Second Reality floor entrance.
package jelly

import (
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/timeline"
	"github.com/olivierh59500/dma40/internal/displayclock"
	"github.com/olivierh59500/dma40/internal/dmalogo"
)

const Width, Height, FPS = 640, 400, 50
const ID, Title = "jelly", "DMA JELLY"

// EntryTick starts on the fully visible still inherited from Noname.
const EntryTick = int((FlashAt - .2) * FPS)
const LogoAt, FlashAt, FloorAt = 1.0, 4.0, 7.0
const PullbackAt, SphereAt, SphereJellyAt, DropAt, OrbitAt = 6.5, 9.0, 11.0, 14.0, 18.0

type Scene struct {
	layers *kit.Layers
	world  *jellyWorld
	still  *ebiten.Image
	canvas *ebiten.Image
	outro  *portraitOutro
	clock  displayclock.Clock
	frozen bool
	dirty  bool
	tick   uint64
}

func New(files fs.FS, logo, portrait, font image.Image) (_ *Scene, err error) {
	s := &Scene{}
	var owned kit.Group
	defer func() {
		if err != nil {
			if s.layers == nil {
				owned.Close()
			}
			s.Close()
		}
	}()
	raw, err := fs.ReadFile(files, "floor-colors.bin")
	if err != nil {
		return nil, err
	}
	world, err := newJellyWorld(raw, logo)
	if err != nil {
		return nil, err
	}
	s.world = world
	owned = append(owned, world)
	body := world.logo
	flat := ebiten.NewImageFromImageWithOptions(logoArtwork{Image: logo, rect: body.crop}, &ebiten.NewImageFromImageOptions{Unmanaged: true})
	s.still = flat
	flash := kit.Func{OnDraw: func(dst *ebiten.Image) { dst.Fill(color.White) }}
	s.layers, err = kit.NewLayers(Width, Height,
		kit.TimedLayer{Effect: world, Window: timeline.Window{Start: FlashAt}},
		kit.TimedLayer{Effect: flash, Window: timeline.Window{Start: FlashAt, Duration: .3, FadeOut: .3}},
	)
	if err != nil {
		return nil, err
	}
	s.canvas = render.NewSurface(Width, Height)
	s.outro, err = newPortraitOutro(portrait, font)
	if err != nil {
		return nil, err
	}
	if err = s.sample(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Scene) sample() error {
	if s.tick >= BoidsAt*FPS {
		if !s.frozen {
			s.frozen, s.dirty = true, true
			if err := s.layers.Update(kit.Frame{Tick: BoidsAt * FPS, Time: BoidsAt, Delta: 1.0 / FPS}); err != nil {
				return err
			}
		}
		s.outro.sample(int(s.tick) - int(BoidsAt*FPS))
		return nil
	}
	s.frozen = false
	s.dirty = true
	return s.layers.Update(kit.Frame{Tick: s.tick, Time: float64(s.tick) / FPS, Delta: 1.0 / FPS})
}
func (s *Scene) Update() error { s.tick++; return s.sample() }
func (s *Scene) Seek(tick int) error {
	if tick < 0 || tick > FPS*3600 {
		return fmt.Errorf("jelly preview must be within 0-3600 seconds")
	}
	s.tick = uint64(tick)
	s.clock = displayclock.Clock{}
	return s.sample()
}
func (s *Scene) Tick() uint64  { return s.tick }
func (s *Scene) Phase() string { return PhaseAt(float64(s.tick) / FPS) }
func (s *Scene) Done() bool    { return int(s.tick) >= PiEndTick() }
func PhaseAt(seconds float64) string {
	switch {
	case seconds >= float64(PiEndTick())/FPS:
		return "finished"
	case seconds >= float64(PiSettledTick())/FPS:
		return "pi-hold"
	case seconds >= PiPageAt:
		return "pi-decipher"
	case seconds >= PiClickAt:
		return "pi-click"
	case seconds >= PiCursorAt:
		return "pi-cursor"
	case seconds >= PiAt:
		return "pi-symbol"
	case seconds >= GreetingSettledAt:
		return "portrait-greeting"
	case seconds >= GreetingAt:
		return "portrait-decipher"
	case seconds >= PortraitMoveAt:
		return "portrait-position"
	case seconds >= PortraitAt:
		return "portrait-bounce"
	case seconds >= FlockGatherAt:
		return "portrait-formation"
	case seconds >= BoidsAt:
		return "boids"
	case seconds >= OrbitAt:
		return "jelly-orbit"
	case seconds >= DropAt+firstContact():
		return "jelly-bounce"
	case seconds >= DropAt:
		return "jelly-drop"
	case seconds >= SphereJellyAt:
		return "jelly-enclosure"
	case seconds >= SphereAt:
		return "point-sphere"
	case seconds >= FloorAt:
		return "checkerboard-drop"
	case seconds >= FlashAt+.3:
		return "glenz-logo"
	case seconds >= FlashAt:
		return "logo-flash"
	case seconds >= LogoAt:
		return "dma-logo"
	default:
		return "black"
	}
}
func (s *Scene) Draw(dst *ebiten.Image) {
	s.drawAt(dst, float64(s.tick)/FPS)
}

// DrawLive samples the flock and portrait at display cadence while the native
// simulation remains fixed at 50 Hz. Pausing preserves the last displayed pose.
func (s *Scene) DrawLive(dst *ebiten.Image, now time.Time, paused bool) {
	seconds := float64(s.tick) / FPS
	if s.tick >= BoidsAt*FPS {
		if paused {
			seconds = s.clock.Hold(now, seconds)
		} else {
			seconds = s.clock.Sample(now, seconds)
		}
		seconds = max(BoidsAt, seconds)
	}
	s.drawAt(dst, seconds)
}

func (s *Scene) drawAt(dst *ebiten.Image, seconds float64) {
	// The inherited still has no animated geometry to cache. Draw it directly
	// so an immediate scene seek never depends on a newly allocated canvas.
	if s.tick < FlashAt*FPS {
		dst.Fill(color.Black)
		window := timeline.Window{Start: LogoAt, Duration: FlashAt - LogoAt, FadeIn: .9}
		if _, alpha, active := window.At(float64(s.tick) / FPS); active && alpha > 0 {
			dmalogo.Draw(dst, s.still, dmalogo.Width, dmalogo.CenterY, alpha)
		}
		return
	}
	if s.dirty {
		s.canvas.Fill(color.Black)
		s.layers.Draw(s.canvas)
		s.dirty = false
	}
	if s.tick >= BoidsAt*FPS {
		s.outro.draw(dst, s.canvas, seconds)
		return
	}
	dst.DrawImage(s.canvas, nil)
}
func (s *Scene) Close() {
	if s.outro != nil {
		s.outro.close()
		s.outro = nil
	}
	if s.layers != nil {
		s.layers.Close()
		s.layers = nil
	}
	s.world = nil
	if s.still != nil {
		s.still.Deallocate()
		s.still = nil
	}
	if s.canvas != nil {
		s.canvas.Deallocate()
		s.canvas = nil
	}
}
