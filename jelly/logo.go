package jelly

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
)

type jellyLogo struct {
	mesh             *meshBatch
	deformer         *geometry.DeformProgram
	deformed, viewed []geometry.Vec3
	waves            logoWaves
	warped           []geometry.Vec3
	deltaX           []float64
	height           float64
	crop             image.Rectangle
}

func newJellyLogo(art image.Image) (_ *jellyLogo, err error) {
	model, err := logoVolume(art)
	if err != nil {
		return nil, err
	}
	p := &jellyLogo{height: model.height, crop: model.crop}
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	p.deformer, err = geometry.NewDeformProgram([]geometry.DeformStage{
		{Kind: geometry.DeformTwist, Twist: geometry.TwistField{Axis: 0, Radius: logoWidth / 2}},
		{Kind: geometry.DeformRotate},
	})
	if err != nil {
		return nil, err
	}
	p.mesh, err = newMeshBatch(model.mesh, nil, viewAt(0).camera)
	if err != nil {
		return nil, err
	}
	p.deformed = make([]geometry.Vec3, len(model.mesh.Points))
	p.viewed = make([]geometry.Vec3, len(model.mesh.Points))
	p.warped = make([]geometry.Vec3, len(model.mesh.Points))
	p.deltaX = make([]float64, len(model.mesh.Points))
	p.waves = newLogoWaves(model.mesh.Points)
	p.mesh.Deform = func(index int, _ geometry.Vec3, _ float64) geometry.Vec3 { return p.viewed[index] }
	p.mesh.Light, p.mesh.Ambient = geometry.Vec3{X: -.5, Y: -.5, Z: -1}, .5
	p.mesh.Blend = ebiten.BlendLighter
	if err = p.mesh.SetOutline(effects.MeshOutlineConfig{
		Faces: model.outline, Width: .65, Color: color.NRGBA{R: 155, G: 220, B: 255, A: 100},
	}); err != nil {
		return nil, err
	}
	return p, nil
}

func ramp(start, end, seconds float64) float64 {
	return motion.Smooth(max(0, min(1, (seconds-start)/(end-start))))
}

// prepare keeps the logo's own jelly movement inside the enclosure. The world
// then applies one shared contact deformation and camera to both objects.
func (p *jellyLogo) prepare(f kit.Frame) error {
	t := max(0, f.Time-FlashAt)
	formed := ramp(0, 1.2, t)
	if err := p.deformer.SetTwistAmount(0, formed*(.1+.24*math.Sin(t*.83))); err != nil {
		return err
	}
	angles := geometry.Vec3{
		X: formed * (.12 + .14*math.Sin(t*.83)), Y: formed * .42 * math.Sin(t*.56), Z: formed * .05 * math.Sin(t*.71),
	}
	if err := p.deformer.SetVector(1, angles); err != nil {
		return err
	}
	p.waves.sample(p.warped, p.deltaX, p.mesh.Mesh.Points, t, formed*2.2)
	p.deformer.Apply(p.deformed, p.warped, t)
	// X displacement commutes with an X-axis twist. Add its rotated value
	// afterwards so DCK still evaluates the twist from the original X key.
	xAxis := geometry.RotateXYZ(angles).Apply(geometry.Vec3{X: 1})
	for i := range p.deformed {
		p.deformed[i] = p.deformed[i].Add(xAxis.Scale(p.deltaX[i]))
	}
	return nil
}

func (p *jellyLogo) Close() error {
	if p.mesh != nil {
		p.mesh.Close()
		p.mesh = nil
	}
	return nil
}
