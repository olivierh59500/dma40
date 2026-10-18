package opening

import (
	"errors"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
)

// LeonardCube applies DMA's world-space deformation before Leonard's original
// rotation and six-oscillator 3D translation. Perspective changes with depth.
type LeonardCube struct {
	mesh  *effects.MeshEffect
	dma   *effects.JellyCube
	xy, z *motion.HarmonicFormation
}

func newLeonardCube() (*LeonardCube, error) {
	c := &LeonardCube{}
	config := effects.DMAJellyCubeConfig()
	config.Size, config.RotationSpeed, config.Delay, config.ZoomDuration = 30, 0, 0, 0
	var err error
	c.dma, err = effects.NewJellyCube(config)
	if err != nil {
		return nil, err
	}
	mesh := effects.Cube(60, geometry.Vec2{X: 1, Y: 1}, color.NRGBA{R: 255, G: 200, B: 255, A: 255})
	colors := [6]color.NRGBA{{255, 240, 255, 255}, {255, 224, 255, 255}, {255, 208, 255, 255}, {255, 192, 255, 255}, {255, 176, 255, 255}, {255, 160, 255, 255}}
	for face, native := range [...]int{4, 0, 3, 1, 2, 5} {
		mesh.Triangles[face*2].Color, mesh.Triangles[face*2+1].Color = colors[native], colors[native]
	}
	c.mesh, err = effects.NewMesh(mesh, nil, geometry.Camera{Center: geometry.Vec2{X: Width / 2, Y: 215}, Focal: Height * math.Sqrt(3) / 2, Near: .1})
	if err != nil {
		c.Close()
		return nil, err
	}
	c.mesh.CullBackFaces = true
	c.xy, c.z, err = cubePaths()
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// cubePaths retains the source's amplitudes, phase rounding and 85-unit clock.
func cubePaths() (xy, z *motion.HarmonicFormation, err error) {
	term := func(amplitude, rate float64) motion.IndexedHarmonic {
		return motion.IndexedHarmonic{Amplitude: amplitude, Rate: rate, PhasePeriod: 2 * math.Pi, RoundProduct: true, FusedIndexPhase: true}
	}
	xy, err = motion.NewHarmonicFormation(motion.HarmonicFormationConfig{X: []motion.IndexedHarmonic{term(60, .0323), term(60, .02)}, Y: []motion.IndexedHarmonic{term(50, .0321), term(50, .0257)}})
	if err != nil {
		return nil, nil, err
	}
	z, err = motion.NewHarmonicFormation(motion.HarmonicFormationConfig{Origin: motion.Point{X: 300}, X: []motion.IndexedHarmonic{term(50, .0357), term(50, .0279)}})
	return
}

func (c *LeonardCube) Transform(seconds float64) effects.Transform {
	clocks := [2]float64{seconds * 85}
	xy, depth := c.xy.At(0, clocks, 1), c.z.At(0, clocks, 1)
	angle := seconds * math.Pi / 2
	return effects.Transform{Position: geometry.Vec3{X: xy.X, Y: -xy.Y, Z: depth.X}, Rotation: geometry.Vec3{X: -angle, Y: angle * 1.1}, Scale: 1}
}

func (c *LeonardCube) Update(frame kit.Frame) error {
	if err := c.dma.Update(frame); err != nil {
		return err
	}
	points := c.dma.Pose()
	copy(c.mesh.Mesh.Points, points[:])
	c.mesh.Transform = c.Transform(frame.Time)
	return c.mesh.Update(frame)
}
func (c *LeonardCube) Draw(dst *ebiten.Image) { c.mesh.Draw(dst) }
func (c *LeonardCube) Close() error {
	var err error
	if c.mesh != nil {
		err = c.mesh.Close()
	}
	if c.dma != nil {
		err = errors.Join(err, c.dma.Close())
	}
	return err
}
