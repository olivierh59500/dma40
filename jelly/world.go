package jelly

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/render"
)

type jellyWorld struct {
	logo             *jellyLogo
	sphere           *jellySphere
	floor            *checkerboard
	deformer         *geometry.DeformProgram
	source, deformed []geometry.Vec3
	bounce           bounce
	view             sceneView
	center, contact  geometry.Vec3
	white            *ebiten.Image
	shadow           *render.Batch
	seconds          float64
}

func newJellyWorld(colors []byte, logo image.Image) (_ *jellyWorld, err error) {
	w := &jellyWorld{bounce: newBounce()}
	defer func() {
		if err != nil {
			w.Close()
		}
	}()
	w.logo, err = newJellyLogo(logo)
	if err != nil {
		return nil, err
	}
	w.sphere, err = newJellySphere()
	if err != nil {
		return nil, err
	}
	w.floor, err = newCheckerboard(colors)
	if err != nil {
		return nil, err
	}
	w.deformer, err = geometry.NewDeformProgram([]geometry.DeformStage{
		{Kind: geometry.DeformWobble, Wobble: geometry.WobbleField{
			Key: geometry.Vec3{X: .006, Y: .007, Z: .008}, Radius: sphereRadius, BaseInfluence: .25, RadialInfluence: .75,
			X: []geometry.Harmonic{{Gain: .75, Speed: 5.4, Spatial: 1.1}},
			Y: []geometry.Harmonic{{Gain: .4, Speed: 6.5, Spatial: 1.5, Phase: 1}},
			Z: []geometry.Harmonic{{Gain: .7, Speed: 4.2, Spatial: 1.1, Phase: .7}},
		}},
		{Kind: geometry.DeformTwist, Twist: geometry.TwistField{Axis: 1, Radius: sphereRadius}},
		{Kind: geometry.DeformScale, Vector: geometry.Vec3{X: 1, Y: 1, Z: 1}},
	})
	if err != nil {
		return nil, err
	}
	w.source = make([]geometry.Vec3, len(w.logo.deformed)+len(w.sphere.base))
	w.deformed = make([]geometry.Vec3, len(w.source))
	w.white = ebiten.NewImage(1, 1)
	w.white.Fill(color.White)
	w.shadow = render.NewBatch(256)
	return w, nil
}

func (w *jellyWorld) sampleGeometry(f kit.Frame) error {
	w.seconds, w.view = f.Time, viewAt(f.Time)
	w.bounce.sample(f.Time - DropAt)
	if err := w.logo.prepare(f); err != nil {
		return err
	}
	copy(w.source, w.logo.deformed)
	rotation := geometry.RotateXYZ(geometry.Vec3{Y: max(0, f.Time-SphereAt) * .13})
	offset := len(w.logo.deformed)
	for i, point := range w.sphere.base {
		w.source[offset+i] = rotation.Apply(point)
	}
	gel := ramp(SphereJellyAt, SphereJellyAt+1.5, f.Time)
	compression := w.bounce.compression
	if err := w.deformer.SetWobbleAmount(0, gel*(2.8+50*math.Abs(compression))); err != nil {
		return err
	}
	if err := w.deformer.SetTwistAmount(1, gel*(.07+.3*compression)); err != nil {
		return err
	}
	sy := 1 - compression
	lateral := 1 / math.Sqrt(sy)
	if err := w.deformer.SetVector(2, geometry.Vec3{X: lateral, Y: sy, Z: lateral}); err != nil {
		return err
	}
	w.deformer.Apply(w.deformed, w.source, f.Time)
	// The full envelope, including its point skin, chooses the support point.
	// All world vertices remain above the same plane that the floor renders.
	bottom := offset
	for i := offset + 1; i < len(w.deformed); i++ {
		if w.deformed[i].Y > w.deformed[bottom].Y {
			bottom = i
		}
	}
	support, source := w.support(w.source[bottom], w.deformed[bottom], f.Time)
	// One point always marks the analytic foot. This avoids a visible gap
	// between a finite point population and the continuous spherical surface.
	last := len(w.deformed) - 1
	w.source[last], w.deformed[last] = source, support
	w.center = geometry.Vec3{Y: groundY - w.bounce.height - support.Y}
	w.contact = support.Add(w.center)
	for i := range w.logo.viewed {
		w.logo.viewed[i] = w.view.point(w.deformed[i].Add(w.center))
	}
	for i := range w.sphere.viewed {
		w.sphere.viewed[i] = w.view.point(w.deformed[offset+i].Add(w.center))
	}
	return nil
}

func (w *jellyWorld) Update(f kit.Frame) error {
	if err := w.sampleGeometry(f); err != nil {
		return err
	}
	w.logo.mesh.Camera = w.view.camera
	if err := w.logo.mesh.Update(f); err != nil {
		return err
	}
	if err := w.sphere.update(f, w.view, w.view.point(w.center).Z); err != nil {
		return err
	}
	return w.floor.update(f, w.view)
}

// Refine the support on the continuous sphere instead of snapping the shadow
// between whichever two discrete points happen to be lowest this frame.
func (w *jellyWorld) support(source, best geometry.Vec3, seconds float64) (geometry.Vec3, geometry.Vec3) {
	step := .04
	for range 7 {
		for _, axis := range []geometry.Vec3{{X: step}, {X: -step}, {Z: step}, {Z: -step}} {
			candidate := geometry.RotateXYZ(axis).Apply(source)
			value := w.deformer.Deform(0, candidate, seconds)
			if value.Y > best.Y {
				source, best = candidate, value
			}
		}
		step *= .5
	}
	return best, source
}

func (w *jellyWorld) drawShadow(dst *ebiten.Image) {
	alpha := ramp(SphereAt, DropAt, w.seconds) / (1 + w.bounce.height*.009)
	if alpha <= 0 {
		return
	}
	radius := sphereRadius * (.8 + .25*w.bounce.compression)
	w.shadow.Begin(dst, w.white)
	for layer := 0; layer < 3; layer++ {
		r := radius * (1.1 - float64(layer)*.2)
		paint := color.NRGBA{A: uint8(alpha * (10 + float64(layer)*12))}
		w.shadow.Fan(48, func(i int) ebiten.Vertex {
			a := float64(i) * 2 * math.Pi / 48
			world := geometry.Vec3{X: w.contact.X + r*math.Cos(a), Y: groundY - .2, Z: w.contact.Z + r*math.Sin(a)}
			point, _, _ := w.view.camera.Project(w.view.point(world))
			return render.Vertex(point.X, point.Y, 0, 0, paint)
		})
	}
	w.shadow.Flush()
}

func (w *jellyWorld) Draw(dst *ebiten.Image) {
	if w.seconds >= FloorAt {
		w.floor.Draw(dst)
		w.drawShadow(dst)
	}
	w.sphere.drawBack(dst)
	w.logo.mesh.Draw(dst)
	w.logo.mesh.DrawOutline(dst)
	w.sphere.drawFront(dst)
}

func (w *jellyWorld) Close() error {
	if w.logo != nil {
		w.logo.Close()
		w.logo = nil
	}
	if w.sphere != nil {
		w.sphere.Close()
		w.sphere = nil
	}
	if w.floor != nil {
		w.floor.Close()
		w.floor = nil
	}
	if w.white != nil {
		w.white.Deallocate()
		w.white = nil
	}
	return nil
}
