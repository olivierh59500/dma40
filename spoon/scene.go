// Package spoon composes the native Go "There is no spoon" demo through DCK.
package spoon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/fs"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/timeline"
	"github.com/olivierh59500/dma40/city"
	"github.com/olivierh59500/dma40/internal/cityassets"
	"github.com/olivierh59500/dma40/internal/displayclock"
	"github.com/olivierh59500/dma40/internal/mentalmedia"
)

const Width, Height, FPS = 640, 400, 50
const ID, Title = "spoon", "THERE IS NO SPOON"

// DefaultNonameSeconds is the fixed hold before the logo-to-Jelly handoff.
const DefaultNonameSeconds, LogoHandoffSeconds = 40.0, 2.0
const LogoFadeSeconds, LogoSettleSeconds = .5, 1.5
const RevealAt, GatherAt, SpoonAt, SpoonSolidAt, BendAt = 6.0, 10.0, 12.0, 14.0, 15.0
const FlightAt, FlightDuration = BendAt + 8, 14.0
const MatrixAt = FlightAt + FlightDuration
const SecretsAt = MatrixAt + 8
const PacmanAt = SecretsAt + 17
const FadeAt, CircleAt = PacmanAt + pacmanSweepSeconds + .16, FadeAt + .44
const CircleLogoAt = CircleAt + 3
const CircleLensAt = CircleLogoAt + 3
const Quote = "THERE IS NO SPOON"
const Credit = "IDEA BY BILIZIR FROM DMA"
const Slogan = "DMA IS GOOD FOR YOU !"

type Scene struct {
	layers        *kit.Layers
	textures      []*ebiten.Image
	tick          uint64
	lens          *liveLens
	water         *circularWater
	finale        *nonameFinale
	flight        *voxelFlight
	nonameSeconds float64
	cloud         *ballCloud
	visualClock   displayclock.Clock
	city          *city.Scene
	cityReady     bool
	cityIncoming  *ebiten.Image
}

func New(files fs.FS, prompt, logo image.Image) (_ *Scene, err error) {
	return NewTimed(files, prompt, logo, DefaultNonameSeconds)
}

// NewTimed retains the entire source sequence before its logo-only handoff.
func NewTimed(files fs.FS, prompt, logo image.Image, nonameSeconds float64) (_ *Scene, err error) {
	if nonameSeconds <= 0 {
		return nil, fmt.Errorf("Noname duration must be positive")
	}
	s := &Scene{nonameSeconds: nonameSeconds}
	var owned kit.Group
	defer func() {
		if err != nil {
			if s.layers == nil {
				owned.Close()
			}
			s.Close()
		}
	}()
	load := func(name string) (image.Image, error) {
		raw, e := fs.ReadFile(files, name)
		if e != nil {
			return nil, e
		}
		im, _, e := image.Decode(bytes.NewReader(raw))
		return im, e
	}
	balls, err := load("balls.png")
	if err != nil {
		return nil, err
	}
	cloud, err := newBallCloud(balls)
	if err != nil {
		return nil, err
	}
	owned = append(owned, cloud)
	s.cloud = cloud
	silver, err := newMetal()
	if err != nil {
		return nil, err
	}
	owned = append(owned, silver)
	s.flight, err = newVoxelFlight()
	if err != nil {
		return nil, err
	}
	owned = append(owned, s.flight)
	grey := ebiten.NewImageFromImage(prompt)
	s.textures = append(s.textures, grey)
	font, err := presets.BitmapFont("cuddly-loader", grey, ebiten.FilterNearest)
	if err != nil {
		return nil, err
	}
	code, err := newTerminal(font)
	if err != nil {
		return nil, err
	}
	owned = append(owned, code)
	raw, err := fs.ReadFile(files, "circle.json")
	if err != nil {
		return nil, err
	}
	var art mentalmedia.Circle
	if err = json.Unmarshal(raw, &art); err != nil {
		return nil, err
	}
	ring, err := newCircle(art)
	if err != nil {
		return nil, err
	}
	owned = append(owned, ring)
	mark, err := newCuddlyLogo(logo)
	if err != nil {
		return nil, err
	}
	owned = append(owned, mark)
	finale, err := newNonameFinale(files, logo, nonameSeconds)
	if err != nil {
		return nil, err
	}
	owned = append(owned, finale)
	s.finale = finale
	comic, err := cityassets.Load()
	if err != nil {
		return nil, err
	}
	s.city, err = city.New(logo, prompt, comic)
	if err != nil {
		return nil, err
	}
	s.cityIncoming = ebiten.NewImage(Width, Height)
	quote := kit.Func{OnDraw: func(dst *ebiten.Image) { font.Print(dst, Quote, (Width-float64(len(Quote))*16)/2, 345, 1, 1) }}
	s.layers, err = kit.NewLayers(Width, Height,
		kit.TimedLayer{Effect: cloud, Window: timeline.Window{Duration: SpoonSolidAt, FadeIn: .8}},
		kit.TimedLayer{Effect: silver, Window: timeline.Window{Start: SpoonAt, Duration: FlightAt + 1.5 - SpoonAt, FadeOut: 2}},
		kit.TimedLayer{Effect: quote, Window: timeline.Window{Start: SpoonSolidAt, Duration: FlightAt + 1.5 - SpoonSolidAt, FadeIn: 1, FadeOut: 1.5}},
		kit.TimedLayer{Effect: s.flight, Window: timeline.Window{Start: FlightAt, Duration: MatrixAt + 1 - FlightAt, FadeIn: 1.3, FadeOut: 1.6}, LocalTime: true},
		kit.TimedLayer{Effect: code, Window: timeline.Window{Start: MatrixAt - 1, Duration: CircleAt - MatrixAt + 1, FadeIn: 3, FadeOut: CircleAt - FadeAt}},
		kit.TimedLayer{Effect: mark, Window: timeline.Window{Start: CircleLogoAt, Duration: CityAt() - CircleLogoAt, FadeIn: 2.5}, LocalTime: true},
		kit.TimedLayer{Effect: ring, Window: timeline.Window{Start: CircleAt, Duration: CityAt() - CircleAt, FadeIn: .5}, LocalTime: true},
		kit.TimedLayer{Effect: finale, Window: timeline.Window{Start: FinaleAt()}, LocalTime: true},
	)
	if err != nil {
		return nil, err
	}
	s.water, err = newCircularWater(s.layers)
	if err != nil {
		return nil, err
	}
	s.lens, err = newLiveLens()
	if err != nil {
		return nil, err
	}
	if err = s.sample(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Scene) sample() error {
	if err := s.finale.meter.update(s.tick); err != nil {
		return err
	}
	f := kit.Frame{Tick: s.tick, Time: float64(s.tick) / FPS, Delta: 1.0 / FPS}
	if err := s.layers.Update(f); err != nil {
		return err
	}
	s.lens.Update(f)
	if err := s.water.Update(f); err != nil {
		return err
	}
	if f.Time >= CityAt() && f.Time < FinaleAt() && !s.cityReady {
		return s.captureCityEntrance(f)
	}
	return nil
}

func (s *Scene) captureCityEntrance(current kit.Frame) error {
	// Reconstruct the last complete circular frame even after a direct preview
	// seek. Restoring the native frame leaves every existing source clock intact.
	end := kit.Frame{Time: CityAt() - 1.0/FPS}
	if err := s.layers.Update(end); err != nil {
		return err
	}
	s.lens.Update(end)
	if err := s.water.Update(end); err != nil {
		return err
	}
	s.city.Source.Fill(color.Black)
	s.lens.Draw(s.city.Source, s.water)
	if err := s.layers.Update(current); err != nil {
		return err
	}
	s.lens.Update(current)
	if err := s.water.Update(current); err != nil {
		return err
	}
	s.cityReady = true
	return nil
}
func (s *Scene) Update() error { s.tick++; return s.sample() }
func (s *Scene) Seek(tick int) error {
	if tick < 0 || tick > FPS*3600 {
		return fmt.Errorf("demo preview must be within 0-3600 seconds")
	}
	s.tick = uint64(tick)
	s.visualClock = displayclock.Clock{}
	return s.sample()
}
func (s *Scene) Tick() uint64 { return s.tick }
func (s *Scene) Phase() string {
	if float64(s.tick)/FPS >= s.HandoffAt() {
		return "dma-logo-handoff"
	}
	return PhaseAt(float64(s.tick) / FPS)
}
func (s *Scene) HandoffAt() float64 { return FinaleAt() + s.nonameSeconds }
func (s *Scene) Duration() float64  { return s.HandoffAt() + LogoHandoffSeconds }
func PhaseAt(seconds float64) string {
	switch {
	case seconds >= FinaleAt():
		return "noname-finale"
	case seconds >= CityAt():
		return city.PhaseAt(seconds - CityAt())
	case seconds >= CircleLensAt:
		return "magnified-scroll"
	case seconds >= CircleAt:
		return "circular-scroll"
	case seconds >= FadeAt:
		return "fade-to-black"
	case seconds >= PacmanAt:
		return "pacman"
	case seconds >= SecretsAt:
		return "no-more-secrets"
	case seconds >= MatrixAt:
		return "matrix"
	case seconds >= FlightAt:
		return "voxel-flight"
	case seconds >= BendAt:
		return "spoon-bend"
	case seconds >= SpoonAt:
		return "metal-spoon"
	case seconds >= GatherAt:
		return "ball-spoon"
	case seconds >= RevealAt:
		return "pull-back"
	default:
		return "vectorballs"
	}
}

func (s *Scene) Draw(dst *ebiten.Image) { s.drawAt(dst, float64(s.tick)/FPS) }

func (s *Scene) drawAt(dst *ebiten.Image, seconds float64) {
	dst.Fill(color.Black)
	if float64(s.tick)/FPS >= CityAt() && float64(s.tick)/FPS < FinaleAt() {
		if seconds >= CityAt()+city.PortalAt {
			// The aperture reveals Noname's exact first pose; its unchanged native
			// choreography starts when the logo has opened across the full canvas.
			_ = s.finale.Update(kit.Frame{})
			s.cityIncoming.Fill(color.Black)
			s.finale.Draw(s.cityIncoming)
		}
		s.city.DrawAt(dst, seconds-CityAt(), s.cityIncoming)
		return
	}
	s.lens.Draw(dst, s.water)
}

// DrawLive samples authored flights at display cadence. The original bitmap
// pages and source-specific object controllers retain their native clocks.
func (s *Scene) DrawLive(dst *ebiten.Image, now time.Time, paused bool) {
	seconds := float64(s.tick) / FPS
	visual := seconds
	if paused {
		visual = s.visualClock.Hold(now, seconds)
	} else {
		visual = s.visualClock.Sample(now, seconds)
	}
	if seconds < SpoonSolidAt {
		s.cloud.live, s.cloud.liveSeconds = true, visual
		defer func() { s.cloud.live = false }()
	}
	if seconds >= FinaleAt() {
		s.finale.stars.live, s.finale.stars.liveSeconds = true, max(0, visual-FinaleAt())
		defer func() { s.finale.stars.live = false }()
	}
	if seconds >= FlightAt && seconds < MatrixAt+1 {
		s.flight.liveAt(now, paused)
		defer func() { s.flight.volume.live = false }()
	}
	s.drawAt(dst, visual)
}
func (s *Scene) Close() {
	if s.city != nil {
		s.city.Close()
		s.city = nil
	}
	if s.cityIncoming != nil {
		s.cityIncoming.Deallocate()
		s.cityIncoming = nil
	}
	if s.water != nil {
		s.water.Close()
		s.water = nil
	}
	if s.lens != nil {
		s.lens.Close()
		s.lens = nil
	}
	if s.layers != nil {
		s.layers.Close()
		s.layers = nil
	}
	for _, im := range s.textures {
		im.Deallocate()
	}
	s.textures = nil
}
