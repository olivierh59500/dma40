package jelly

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/render"
)

type portraitOutro struct {
	flock                                flock
	photo                                *ebiten.Image
	batch                                *render.Batch
	warp                                 *effects.Warp
	greeting                             *greeting
	pi                                   *piEnding
	pose                                 portraitPose
	tint                                 color.NRGBA
	originColors, portraitColors, colors *ebiten.Image
	colorsReady                          bool
	wingPhase                            [flockCount][2]float64
}

func newPortraitOutro(portrait, font image.Image) (_ *portraitOutro, err error) {
	o := &portraitOutro{photo: ebiten.NewImageFromImageWithOptions(portrait, &ebiten.NewImageFromImageOptions{Unmanaged: true}), batch: render.NewBatch(flockCount * 2)}
	defer func() {
		if err != nil {
			o.close()
		}
	}()
	o.flock.reset()
	o.originColors = render.NewSurface(flockSide, flockSide)
	o.portraitColors = render.NewSurface(flockSide, flockSide)
	o.colors = render.NewSurface(flockSide, flockSide)
	for i := range o.wingPhase {
		sin, cos := math.Sincos(float64(i) * .7)
		o.wingPhase[i] = [2]float64{sin, cos}
	}
	o.batch.Options.Blend = ebiten.BlendLighter
	o.greeting, err = newGreeting(font)
	if err != nil {
		return nil, err
	}
	o.pi, err = newPiEnding(font)
	if err != nil {
		return nil, err
	}
	source := kit.Func{OnDraw: func(dst *ebiten.Image) {
		var options ebiten.DrawImageOptions
		options.GeoM.Scale(260/float64(o.photo.Bounds().Dx()), 260/float64(o.photo.Bounds().Dy()))
		options.Filter = ebiten.FilterLinear
		dst.DrawImage(o.photo, &options)
	}}
	o.warp, err = effects.NewWarp(source, 260, 260, 20, 20)
	if err != nil {
		return nil, err
	}
	o.warp.Map = func(x, y, seconds float64) geometry.Vec2 {
		px, py := o.pose.point(x, y)
		return geometry.Vec2{X: px, Y: py}
	}
	o.warp.Tint = func(x, y, seconds float64) color.Color {
		return &o.tint
	}
	return o, nil
}

func (o *portraitOutro) sample(tick int) {
	// Once the photo is opaque, particle integration and its seek cost disappear.
	if tick >= 0 && tick < int((PortraitAt-BoidsAt)*FPS) {
		o.flock.seek(tick)
	}
}

func (o *portraitOutro) draw(dst, source *ebiten.Image, seconds float64) {
	if seconds >= PiPageAt {
		o.pi.draw(dst, seconds)
		return
	}
	age := max(0, seconds-BoidsAt)
	dst.Fill(color.Black)
	baseAlpha := 1 - ease(0, .7, age)
	if baseAlpha > 0 {
		var options ebiten.DrawImageOptions
		options.ColorScale.ScaleAlpha(float32(baseAlpha))
		dst.DrawImage(source, &options)
	}
	photoAlpha := ease(PortraitAt-.5, PortraitAt, seconds)
	if photoAlpha < 1 {
		alpha := ease(0, .7, age) * (1 - photoAlpha)
		colorMix := ease(1.8, 5.8, age)
		if alpha > 0 {
			o.drawBirds(dst, o.palette(source, colorMix), seconds, alpha)
		}
	}
	if photoAlpha > 0 {
		o.pose = portraitAt(seconds)
		o.tint = color.NRGBA{255, 255, 255, uint8(math.Round(photoAlpha * 255))}
		// DCK supplies the subdivided mesh, texture surface and deformation pass.
		_ = o.warp.Update(kit.Frame{Time: seconds})
		o.warp.Draw(dst)
	}
	if seconds >= GreetingAt {
		o.greeting.draw(dst, seconds)
	}
	if seconds >= PiAt {
		o.pi.draw(dst, seconds)
	}
}

// palette mixes one color per bird on a tiny DCK surface. This replaces two
// complete geometry uploads with one batch, without reading GPU pixels back.
func (o *portraitOutro) palette(source *ebiten.Image, mix float64) *ebiten.Image {
	if !o.colorsReady {
		for _, pair := range [2][2]*ebiten.Image{{source, o.originColors}, {o.photo, o.portraitColors}} {
			var options ebiten.DrawImageOptions
			options.GeoM.Scale(float64(flockSide)/float64(pair[0].Bounds().Dx()), float64(flockSide)/float64(pair[0].Bounds().Dy()))
			composite.Instance{Image: pair[0], Options: options}.Draw(pair[1])
		}
		o.colorsReady = true
	}
	if mix <= 0 {
		return o.originColors
	}
	if mix >= 1 {
		return o.portraitColors
	}
	o.colors.Clear()
	for _, layer := range []struct {
		image *ebiten.Image
		alpha float64
	}{{o.originColors, 1 - mix}, {o.portraitColors, mix}} {
		options := ebiten.DrawImageOptions{Blend: ebiten.BlendLighter}
		options.ColorScale.ScaleAlpha(float32(layer.alpha))
		composite.Instance{Image: layer.image, Options: options}.Draw(o.colors)
	}
	return o.colors
}

func (o *portraitOutro) drawBirds(dst, texture *ebiten.Image, seconds, alpha float64) {
	if alpha <= 0 {
		return
	}
	age := seconds - BoidsAt
	lock := ease(PortraitAt-BoidsAt-1.35, PortraitAt-BoidsAt-.5, age)
	point := ease(.1, 1.1, age)
	width := (10*(1-point)+2.2*point)*(1-lock) + (260.0/flockSide+.1)*lock
	height := (6.25*(1-point)+1.5*point)*(1-lock) + (260.0/flockSide+.1)*lock
	delta := min(.06, max(-.04, age-float64(o.flock.tick)/FPS))
	wingSin, wingCos := math.Sincos(age * 17)
	wingScale := .45 * point * (1 - lock)
	orientation := point * (1 - lock)
	o.batch.Begin(dst, texture)
	for i, b := range o.flock.birds {
		x, y := b.x+b.vx*delta, b.y+b.vy*delta
		depth := (b.z + b.vz*delta) * ease(.3, 1.5, age) * (1 - lock)
		perspective := 700 / (700 + depth)
		x = 320 + (x-320)*perspective
		y = 200 + (y-200)*perspective
		tx, ty := targetPoint(i)
		x += (tx - x) * lock
		y += (ty - y) * lock
		u := (float64(i%flockSide) + .5) / flockSide * float64(texture.Bounds().Dx())
		v := (float64(i/flockSide) + .5) / flockSide * float64(texture.Bounds().Dy())
		var quad [4]ebiten.Vertex
		wing := (wingSin*o.wingPhase[i][1] + wingCos*o.wingPhase[i][0]) * wingScale
		length := math.Hypot(b.ux, b.uy)
		ux, uy := 1.0, 0.0
		if length > 1e-6 {
			ux, uy = b.ux/length, b.uy/length
		}
		for j, p := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			dx, dy := p[0]*width*perspective/2, (p[1]*height/2+wing*p[0])*perspective
			forward, side := dx*ux-dy*uy, dx*uy+dy*ux
			dx += (forward - dx) * orientation
			dy += (side - dy) * orientation
			// At small size the wings hint at birds; the final grid closes cleanly.
			quad[j] = ebiten.Vertex{DstX: float32(x + dx), DstY: float32(y + dy), SrcX: float32(u), SrcY: float32(v), ColorR: float32(alpha), ColorG: float32(alpha), ColorB: float32(alpha), ColorA: float32(alpha)}
		}
		o.batch.Quad(quad)
	}
	o.batch.Flush()
}

func (o *portraitOutro) close() {
	if o == nil {
		return
	}
	if o.warp != nil {
		o.warp.Close()
	}
	if o.photo != nil {
		o.photo.Deallocate()
	}
	for _, surface := range [...]*ebiten.Image{o.originColors, o.portraitColors, o.colors} {
		if surface != nil {
			surface.Deallocate()
		}
	}
	o.greeting.close()
	o.pi.close()
}
