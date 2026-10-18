package city

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/render"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/dma40/internal/dmalogo"
)

type Scene struct {
	route         route
	district      district
	car           *carModel
	renderer      *renderer
	pixels        *pixelMorph
	Source        *ebiten.Image
	canvas        *ebiten.Image
	overhead      *ebiten.Image
	overheadReady bool
	entrance      *ebiten.Image
	entranceReady bool
	mask          *ebiten.Image
	logo          *ebiten.Image
	portrait      *ebiten.Image
	fontImage     *ebiten.Image
	font          scrolling.BitmapGrid
	text          *ebiten.Image
	ink           *render.Batch
	white         *ebiten.Image
	sky, portal   *ebiten.Shader
	skyOptions    ebiten.DrawRectShaderOptions
	portalOptions ebiten.DrawRectShaderOptions
	preparedAt    float64
	prepared      bool
}

func New(logo, prompt, portrait image.Image) (_ *Scene, err error) {
	s := &Scene{}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	s.route, err = newRoute()
	if err != nil {
		return nil, err
	}
	s.district, err = newDistrict(s.route, logo)
	if err != nil {
		return nil, err
	}
	s.car, s.renderer = newCar(), newRenderer()
	s.pixels, err = newPixelMorph(s.car, s.route)
	if err != nil {
		return nil, err
	}
	art, err := dmalogo.Crop(logo)
	if err != nil {
		return nil, err
	}
	s.logo = ebiten.NewImageFromImage(art)
	s.portrait = ebiten.NewImageFromImage(portrait)
	s.fontImage = ebiten.NewImageFromImage(prompt)
	s.font, err = presets.BitmapFont("cuddly-loader", s.fontImage, ebiten.FilterNearest)
	if err != nil {
		return nil, err
	}
	s.Source, s.canvas, s.mask = render.NewSurface(Width, Height), render.NewSurface(Width, Height), render.NewSurface(Width, Height)
	s.overhead = render.NewSurface(Width, Height)
	s.entrance = render.NewSurface(Width, Height)
	s.text, s.white = render.NewSurface(400, 110), ebiten.NewImage(1, 1)
	s.white.Fill(color.White)
	s.ink = render.NewBatch(128)
	s.sky, err = ebiten.NewShader([]byte(skyShader))
	if err != nil {
		return nil, err
	}
	s.portal, err = ebiten.NewShader([]byte(portalShader))
	if err != nil {
		return nil, err
	}
	s.skyOptions.Uniforms = map[string]any{"Pitch": float32(0)}
	s.portalOptions.Uniforms = map[string]any{"Progress": float32(0)}
	s.portalOptions.Images[0], s.portalOptions.Images[2] = s.canvas, s.mask
	s.mask.WritePixels(portalDistance(art).Pix)
	return s, nil
}

func (s *Scene) drawLogo(dst *ebiten.Image, alpha float64) {
	var options ebiten.DrawImageOptions
	options.Filter = ebiten.FilterLinear
	scale := 470 / float64(s.logo.Bounds().Dx())
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(85, 212-float64(s.logo.Bounds().Dy())*scale/2)
	options.ColorScale.ScaleAlpha(float32(alpha))
	dst.DrawImage(s.logo, &options)
}

func (s *Scene) compose(seconds float64) {
	if s.prepared && s.preparedAt == seconds {
		return
	}
	s.prepared, s.preparedAt = true, seconds
	if seconds >= RooftopAt && s.overheadReady {
		s.canvas.Clear()
		s.canvas.DrawImage(s.overhead, nil)
		s.flattenRoofLogo(seconds)
		return
	}
	// Anchor the retained overhead scene to its authored cue, rather than the
	// first display frame that happens to cross it at a particular frame rate.
	worldTime := min(seconds, RooftopAt)
	v, pose := s.route.cameraAt(worldTime), s.route.pose(worldTime)
	if seconds < FormedAt && s.entranceReady {
		// Formation has a fixed camera and a stationary car. Retain the actual
		// district render while all incoming pixels move above it.
		s.canvas.Clear()
		s.canvas.DrawImage(s.entrance, nil)
		s.renderer.begin(v)
		if seconds >= FormedAt-.6 {
			s.car.roll(0)
			s.renderer.shadow(pose)
			s.renderer.mesh(s.car.Mesh, geometry.RotateXYZ(pose.rotation), pose.position, false, ease(FormedAt-.6, FormedAt, seconds), nil)
		}
		s.renderer.draw(s.canvas)
		return
	}
	s.skyOptions.Uniforms["Pitch"] = float32(v.forward.Y)
	s.canvas.DrawRectShader(Width, Height, s.sky, &s.skyOptions)
	identity := geometry.RotateXYZ(geometry.Vec3{})
	// Coplanar ground and markings draw first, avoiding painter-depth guesses
	// for the two enormous ground triangles that straddle the entire district.
	s.renderer.begin(v)
	s.renderer.mesh(effects.Mesh{Points: s.district.ground.Points[:4], Triangles: s.district.ground.Triangles[:2]}, identity, geometry.Vec3{}, true, 1, s.district.groundMaterials[:2])
	s.renderer.draw(s.canvas)
	s.renderer.begin(v)
	s.renderer.mesh(effects.Mesh{Points: s.district.ground.Points, Triangles: s.district.ground.Triangles[2:]}, identity, geometry.Vec3{}, true, 1, s.district.groundMaterials[2:])
	s.renderer.draw(s.canvas)
	s.renderer.begin(v)
	for _, building := range s.district.buildings {
		if !s.renderer.visible(building.center, building.radius) {
			continue
		}
		mesh := building.mesh
		// Distant window strips are below a display pixel. Keep every solid
		// building while avoiding their subpixel facade geometry.
		if s.renderer.view.point(building.center).Z > 320 {
			mesh.Triangles = mesh.Triangles[:24]
			mesh.Points = mesh.Points[:16]
		}
		s.renderer.mesh(mesh, identity, geometry.Vec3{}, true, 1, building.materials)
	}
	if s.renderer.visible(geometry.Vec3{Y: -14, Z: 900}, 285) {
		s.renderer.mesh(s.district.campus, identity, geometry.Vec3{}, true, 1, s.district.campusMaterials)
	}
	if seconds < FormedAt {
		s.renderer.draw(s.canvas)
		s.entrance.Clear()
		s.entrance.DrawImage(s.canvas, nil)
		s.entranceReady = true
		s.renderer.begin(v)
	}
	if seconds >= FormedAt-.6 {
		s.car.roll(pose.distance)
		s.renderer.shadow(pose)
		s.renderer.mesh(s.car.Mesh, geometry.RotateXYZ(pose.rotation), pose.position, false, ease(FormedAt-.6, FormedAt, seconds), nil)
		s.renderer.thrusters(pose, worldTime)
	}
	s.renderer.draw(s.canvas)
	if seconds >= RooftopAt {
		s.overhead.Clear()
		s.overhead.DrawImage(s.canvas, nil)
		s.overheadReady = true
	}
	s.flattenRoofLogo(seconds)
}

func (s *Scene) flattenRoofLogo(seconds float64) {
	if seconds >= RooftopAt {
		// The stepped, actual rooftops resolve into the same supplied DMA alpha.
		// Their footprint and this flat mark share the overhead registration.
		fade := ease(RooftopAt, PortalAt+.5, seconds)
		var dark ebiten.DrawImageOptions
		dark.ColorScale.Scale(0, 0, 0, float32(fade*.93))
		dark.GeoM.Scale(Width, Height)
		s.canvas.DrawImage(s.white, &dark)
		s.drawLogo(s.canvas, fade)
	}
}

// DrawAt samples absolute display time. Repeated drawing, pause and seeks do
// not advance physics, dialogue, wheel rotation or either transition.
func (s *Scene) DrawAt(dst *ebiten.Image, seconds float64, incoming *ebiten.Image) {
	seconds = max(0, min(Duration, seconds))
	if seconds <= .08 {
		dst.DrawImage(s.Source, nil)
		return
	}
	s.compose(seconds)
	if seconds >= PortalAt && incoming != nil {
		s.portalOptions.Images[1] = incoming
		s.portalOptions.Uniforms["Progress"] = float32(ease(PortalAt, Duration, seconds))
		dst.DrawRectShader(Width, Height, s.portal, &s.portalOptions)
		return
	}
	dst.DrawImage(s.canvas, nil)
	if seconds < FormedAt {
		s.pixels.draw(dst, s.Source, seconds)
	}
	s.dialogue(dst, seconds)
}

func (s *Scene) rectangle(dst *ebiten.Image, x, y, w, h float64, paint color.NRGBA) {
	s.ink.Begin(dst, s.white)
	s.ink.Rect(x, y, w, h, image.Rect(0, 0, 1, 1), paint)
	s.ink.Flush()
}

func (s *Scene) letters(dst *ebiten.Image, text string, x, y, scale float64, black bool) {
	s.text.Clear()
	for row, line := range strings.Split(text, "\n") {
		s.font.Print(s.text, line, 0, float64(row)*20*scale, scale, scale)
	}
	var options ebiten.DrawImageOptions
	options.GeoM.Translate(x, y)
	if black {
		options.ColorScale.Scale(0, 0, 0, 1)
	}
	dst.DrawImage(s.text, &options)
}

func (s *Scene) bubble(dst *ebiten.Image, text string, x, y, w, h, tailX, tailY float64) {
	s.rectangle(dst, x+3, y+4, w, h, color.NRGBA{A: 190})
	s.rectangle(dst, x, y, w, h, color.NRGBA{R: 8, G: 12, B: 22, A: 255})
	s.rectangle(dst, x+3, y+3, w-6, h-6, color.NRGBA{R: 255, G: 250, B: 225, A: 255})
	base := max(x+18, min(x+w-18, tailX))
	s.ink.Begin(dst, s.white)
	s.ink.Triangle(render.Vertex(base-9, y+h-2, 0, 0, color.NRGBA{A: 255}), render.Vertex(base+9, y+h-2, 0, 0, color.NRGBA{A: 255}), render.Vertex(tailX, tailY, 0, 0, color.NRGBA{A: 255}))
	cream := color.NRGBA{R: 255, G: 250, B: 225, A: 255}
	s.ink.Triangle(render.Vertex(base-5, y+h-3, 0, 0, cream), render.Vertex(base+5, y+h-3, 0, 0, cream), render.Vertex(tailX, tailY-4, 0, 0, cream))
	s.ink.Flush()
	s.letters(dst, text, x+14, y+13, .68, true)
}

func (s *Scene) dialogue(dst *ebiten.Image, seconds float64) {
	if seconds >= ReplyAt-.3 && seconds < CloseAt+.6 {
		open := ease(ReplyAt-.3, ReplyAt, seconds) * (1 - ease(CloseAt, CloseAt+.6, seconds))
		x, y := 12-(1-open)*210, 132.0
		s.rectangle(dst, x+4, y+4, 176, 232, color.NRGBA{A: 200})
		s.rectangle(dst, x, y, 176, 232, color.NRGBA{R: 255, G: 247, B: 217, A: 255})
		s.rectangle(dst, x+4, y+4, 168, 224, color.NRGBA{R: 44, G: 75, B: 114, A: 255})
		s.rectangle(dst, x+4, y+4, 168, 30, color.NRGBA{R: 255, G: 247, B: 217, A: 255})
		s.letters(dst, "ELON", x+62, y+10, .65, true)
		var options ebiten.DrawImageOptions
		options.Filter = ebiten.FilterLinear
		scale := 164 / float64(s.portrait.Bounds().Dx())
		options.GeoM.Scale(scale, scale)
		options.GeoM.Rotate(.018 * math.Sin(seconds*3.7))
		options.GeoM.Translate(x+7, y+51+2*math.Sin(seconds*7))
		dst.DrawImage(s.portrait, &options)
		if seconds < ByeAt {
			s.bubble(dst, Answer, x+2, 47, 224, 75, x+166, y+78)
		}
	}
	car, v := s.route.pose(seconds), s.route.cameraAt(seconds)
	anchor, _, visible := v.camera.Project(v.point(car.position.Add(geometry.Vec3{Y: -1.6})))
	if !visible {
		return
	}
	if seconds >= AskAt && seconds < ReplyAt {
		s.bubble(dst, Question, 300, 49, 309, 53, anchor.X, anchor.Y-8)
	}
	if seconds >= ByeAt && seconds < CloseAt {
		s.bubble(dst, Farewell, 294, 40, 326, 76, anchor.X, anchor.Y-8)
	}
}

func (s *Scene) Close() {
	if s.renderer != nil {
		s.renderer.close()
	}
	if s.pixels != nil {
		s.pixels.close()
	}
	for _, im := range []*ebiten.Image{s.Source, s.canvas, s.overhead, s.entrance, s.mask, s.logo, s.portrait, s.fontImage, s.text, s.white} {
		if im != nil {
			im.Deallocate()
		}
	}
	if s.sky != nil {
		s.sky.Deallocate()
	}
	if s.portal != nil {
		s.portal.Deallocate()
	}
}

const skyShader = `//kage:unit pixels
package main
var Pitch float
func Fragment(position vec4, uv vec2, color vec4) vec4 {
 y:=position.y/400
 sky:=mix(vec3(.025,.065,.14),vec3(.18,.38,.49),clamp(y*1.25+Pitch*.35,0,1))
 haze:=exp(-pow((y-.47+Pitch*.45)*7,2))
 return vec4(sky+haze*vec3(.13,.09,.055),1)
}
`

const portalShader = `//kage:unit pixels
package main
var Progress float
func Fragment(position vec4, uv vec2, color vec4) vec4 {
 p:=uv-imageSrc0Origin()
 distance:=imageSrc2At(imageSrc2Origin()+p).r
 radius:=Progress*1.01
 mask:=1-smoothstep(radius-.004,radius+.004,distance)
 mask*=smoothstep(0,.2,Progress)
 return mix(imageSrc0At(uv),imageSrc1At(imageSrc1Origin()+p),mask)
}
`
