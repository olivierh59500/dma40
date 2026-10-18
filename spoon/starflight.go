package spoon

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/sprites"
)

const flightStars, flightRadius = 2200, 800.0

var flightTravel = motion.HarmonicTranslation{
	X: []motion.HarmonicTerm{{Amplitude: 420, Rate: .62}, {Amplitude: 130, Rate: 1.11, Phase: .3}},
	Y: []motion.HarmonicTerm{{Amplitude: 300, Rate: .77}, {Amplitude: 80, Rate: .43, Phase: 1.2}},
}
var flightHeading = motion.HarmonicTranslation{
	X: []motion.HarmonicTerm{{Amplitude: .85, Rate: .39}, {Amplitude: .3, Rate: .91}},
	Y: []motion.HarmonicTerm{{Amplitude: 2.0, Rate: .31}, {Amplitude: .4, Rate: .67, Phase: .4}},
}
var flightDepthRoll = motion.HarmonicTranslation{
	X: []motion.HarmonicTerm{{Amplitude: 580, Rate: .82}, {Amplitude: 170, Rate: 1.37, Phase: .7}},
	Y: []motion.HarmonicTerm{{Amplitude: .5, Rate: .43}, {Amplitude: .2, Rate: .97}},
}

type flightView struct {
	position geometry.Vec3
	rotation geometry.Rotation
}

func flightAt(seconds float64) flightView {
	travel, heading, depth := flightTravel.At(seconds), flightHeading.At(seconds), flightDepthRoll.At(seconds)
	return flightView{
		position: geometry.Vec3{X: travel.X, Y: travel.Y, Z: depth.X},
		rotation: geometry.RotateXYZ(geometry.Vec3{X: heading.X, Y: heading.Y, Z: depth.Y}),
	}
}

// relative keeps the surrounding volume populated during an unbounded flight.
// Radial fog hides cube-boundary recycling before either copy can be visible.
func (v flightView) relative(point geometry.Vec3) (geometry.Vec3, float64) {
	wrap := func(value float64) float64 {
		return value - 2*flightRadius*math.Floor((value+flightRadius)/(2*flightRadius))
	}
	p := point.Sub(v.position)
	p = geometry.Vec3{X: wrap(p.X), Y: wrap(p.Y), Z: wrap(p.Z)}
	fade := 1 - smooth(flightRadius*.76, flightRadius*.98, math.Sqrt(p.Dot(p)))
	return v.rotation.Apply(p), fade
}

// starFlight reuses DCK's Nonameno star material, field projection and batched
// painter. Only the continuous 3D camera path and periodic volume are authored.
type starFlight struct {
	model                            [flightStars]geometry.Vec3
	field                            *sprites.Field
	renderer                         *sprites.FieldRenderer
	style                            sprites.FieldStyle
	camera                           geometry.Camera
	view                             flightView
	previous                         flightView
	fades                            [flightStars]float64
	samples                          []sprites.FieldSample
	seconds, preparedAt, liveSeconds float64
	prepared, dirty, live            bool
}

func newStarFlight() (*starFlight, error) {
	s := &starFlight{camera: geometry.Camera{Center: geometry.Vec2{X: Width / 2, Y: Height / 2}, Focal: 360, Near: 12}, samples: make([]sprites.FieldSample, 0, flightStars)}
	for i := range s.model {
		s.model[i] = geometry.Vec3{X: (noise(i*3+14001) - .5) * 2 * flightRadius, Y: (noise(i*3+14002) - .5) * 2 * flightRadius, Z: (noise(i*3+14003) - .5) * 2 * flightRadius}
	}
	config := presets.DefaultNonamenoStarsConfig()
	config.Width, config.Height, config.CenterY, config.DepthMax = Width, Height, Height/2, flightRadius
	material, err := presets.NonamenoProjectedStars(config)
	if err != nil {
		return nil, err
	}
	s.style = material.Style
	baseSample := s.style.Sample
	s.style.Sample = func(p sprites.FieldSample, a *sprites.FieldAppearance) bool {
		if s.fades[p.Index] == 0 || !baseSample(p, a) {
			return false
		}
		fade := func(c color.RGBA) color.RGBA {
			f := s.fades[p.Index]
			return color.RGBA{R: byte(float64(c.R) * f), G: byte(float64(c.G) * f), B: byte(float64(c.B) * f), A: byte(float64(c.A) * f)}
		}
		a.FillColor, a.TrailColor = fade(a.FillColor), fade(a.TrailColor)
		return true
	}
	s.field, err = sprites.NewField(sprites.FieldConfig{Count: flightStars, Spawn: func(index int, _ bool) sprites.Point {
		p, fade := s.view.relative(s.model[index])
		s.fades[index] = fade
		return sprites.Point{X: p.X, Y: p.Y, Z: p.Z}
	}})
	if err != nil {
		return nil, err
	}
	s.renderer = sprites.NewFieldRenderer(flightStars)
	return s, s.Update(kit.Frame{})
}

func (s *starFlight) Update(f kit.Frame) error {
	s.setTime(f.Time)
	return s.sampleAt(f.Time)
}

// Rendering samples this absolute path once per displayed frame. The native
// text-page update only queues its time, avoiding two full star projections.
func (s *starFlight) setTime(seconds float64) { s.seconds = seconds; s.dirty = true }

func (s *starFlight) sampleAt(seconds float64) error {
	if s.prepared && s.preparedAt == seconds {
		return nil
	}
	s.prepared, s.preparedAt = true, seconds
	s.view, s.previous = flightAt(seconds), flightAt(max(0, seconds-1.0/FPS))
	if err := s.field.ResetCount(flightStars); err != nil {
		return err
	}
	s.samples = s.samples[:0]
	for _, p := range s.field.Sample(sprites.FieldView{Camera: s.camera}) {
		previous, fade := s.previous.relative(s.model[p.Index])
		xy, _, ok := s.camera.Project(previous)
		// Sample the previous point on the same absolute trajectory. A seek
		// cannot create a streak from the last screen or across a recycled star.
		if ok && fade > 0 && seconds > 0 && math.Hypot(p.X-xy.X, p.Y-xy.Y) < 24 {
			p.PreviousX, p.PreviousY, p.Connected = xy.X, xy.Y, true
		}
		s.samples = append(s.samples, p)
	}
	return nil
}
func (s *starFlight) Draw(dst *ebiten.Image) {
	if s.live {
		_ = s.sampleAt(s.liveSeconds)
	} else if s.dirty {
		_ = s.sampleAt(s.seconds)
		s.dirty = false
	}
	s.renderer.Draw(dst, s.samples, s.style)
}
func (s *starFlight) Close() error { return s.renderer.Close() }
