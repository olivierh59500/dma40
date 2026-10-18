package spoon

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/render"
)

// circleWaterline follows the lowest possible letter on the open, tilting arc.
// It depends on geometry rather than the currently visible message, so blanks
// never make the water jump. The original circular path remains unchanged.
func circleWaterline(seconds float64) int {
	camera, rotation := circleCamera(), circleRotation(seconds)
	bottom := 0.0
	for i := 0; i <= 64; i++ {
		for _, row := range [...]float64{circleGlyphRow(0), circleGlyphRow(10)} {
			p := rotation.Apply(circlePoint(float64(i)*circleArc/64, row, seconds)).Add(geometry.Vec3{Z: 840})
			if xy, _, ok := camera.Project(p); ok {
				bottom = max(bottom, xy.Y)
			}
		}
	}
	return max(Height-130, min(Height-32, int(math.Ceil(bottom))+2))
}

// circularWater borrows the timed scene and adds DCK's live reflection before
// the magnifier. The source is captured once without reflecting its own output.
type circularWater struct {
	scene      kit.Effect
	source     *ebiten.Image
	reflection *composite.WaterReflection
	frame      kit.Frame
	active     bool
}

func newCircularWater(scene kit.Effect) (*circularWater, error) {
	config := composite.DefaultWaterReflectionConfig()
	config.Alpha, config.Fade = .55, 1
	config.Tint.Scale(.6, .8, 1, 1)
	config.Wave = composite.WaterWave{Amplitude: 3.5, Wavelength: 18, Speed: 2.4, Phase: .3}
	config.Filter = ebiten.FilterLinear
	reflection, err := composite.NewWaterReflection(config)
	if err != nil {
		return nil, err
	}
	return &circularWater{scene: scene, reflection: reflection, source: render.NewSurface(Width, Height)}, nil
}

func (w *circularWater) Update(f kit.Frame) error {
	w.frame = f
	w.active = f.Time >= CircleAt && f.Time < CityAt()
	if !w.active {
		return nil
	}
	horizon := circleWaterline(f.Time - CircleAt)
	config := w.reflection.Config()
	config.Horizon = float64(horizon)
	config.Source = image.Rect(0, horizon-180, Width, horizon)
	config.ScaleY = float64(Height-horizon) / 180
	return w.reflection.SetConfig(config)
}
func (w *circularWater) Draw(dst *ebiten.Image) {
	if !w.active {
		w.scene.Draw(dst)
		return
	}
	w.source.Clear()
	w.scene.Draw(w.source)
	dst.DrawImage(w.source, nil)
	w.reflection.Draw(dst, w.source, w.frame)
}
func (w *circularWater) Close() {
	w.reflection.Close()
	w.source.Deallocate()
}
