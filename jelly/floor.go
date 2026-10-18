package jelly

import (
	"encoding/binary"
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/geometry"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/render"
)

type floorPose struct{ top, height float64 }

// Translate the checker cells themselves, independently of the camera orbit
// and the source's forward/backward depth clock. Units are checker widths.
var floorLateral = motion.HarmonicTranslation{
	X: []motion.HarmonicTerm{{Amplitude: 4.5, Rate: 1.2}},
}

// floorEntrance retains Second Reality's 70 Hz integer drop and rebound.
func floorEntrance() []floorPose {
	poses := []floorPose{{top: 260}}
	yy, ya := 0, 0
	for range 2048 {
		ya++
		yy += ya
		if yy > 48*16 {
			yy -= ya
			ya = -ya * 2 / 3
			if ya > -4 && ya < 4 {
				break
			}
		}
		y := yy / 16
		top, bottom := 130+y/2, 130+y*3/2
		poses = append(poses, floorPose{top: float64(top * 2), height: float64((bottom - top) * 2)})
	}
	return poses
}

// floorClock translates only Move the Mouse's floor and palette clocks.
// The two original hold/reversal intervals remain independent at 50 Hz.
type floorClock struct{ tick, phase, floorHold, colorHold, direction, colorDirection, cursor int }

func newFloorClock() floorClock { return floorClock{floorHold: 100, direction: 1, colorDirection: 1} }
func (c *floorClock) sample(target, colors int) {
	if target < c.tick {
		*c = newFloorClock()
	}
	for c.tick < target {
		c.tick++
		c.floorHold--
		if c.floorHold < 0 {
			c.floorHold, c.direction = 200, -c.direction
		}
		c.colorHold--
		if c.colorHold < 0 {
			c.colorHold, c.colorDirection = 400, -c.colorDirection
		}
		c.phase = (c.phase + c.direction + 12) % 12
		c.cursor = (c.cursor + c.colorDirection + colors) % colors
	}
}

type checkerboard struct {
	shader        *ebiten.Shader
	banks         *ebiten.Image
	pixels        [2 * 34 * 4]byte
	vertices      []ebiten.Vertex
	poses         []floorPose
	clock         floorClock
	data          []byte
	first, second [34]color.NRGBA
	options       ebiten.DrawTrianglesShaderOptions
	view          sceneView
	open          float64
}

func newCheckerboard(data []byte) (_ *checkerboard, err error) {
	if len(data) < 4 || len(data)%4 != 0 {
		return nil, fmt.Errorf("Move the Mouse floor: invalid color table")
	}
	p := &checkerboard{data: data, poses: floorEntrance(), clock: newFloorClock(), banks: render.NewSurface(2, 34)}
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	p.shader, err = ebiten.NewShader([]byte(floorShader))
	if err != nil {
		return nil, err
	}
	p.options.Uniforms = map[string]any{}
	p.options.Images[0] = p.banks
	p.vertices = make([]ebiten.Vertex, 0, 12)
	return p, nil
}

func (p *checkerboard) update(f kit.Frame, view sceneView) error {
	p.view = view
	index := min(len(p.poses)-1, int(math.Floor(max(0, f.Time-FloorAt)*70+1e-7)))
	p.open = p.poses[index].height / p.poses[len(p.poses)-1].height
	p.clock.sample(int(math.Floor(max(0, f.Time-FloorAt)*50+1e-7)), len(p.data)/4)
	source, acceleration, step := p.clock.cursor, 0, 5
	for row := 0; row < 34; row++ {
		for {
			acceleration += step
			if acceleration >= 25 {
				acceleration -= 25
				step++
				break
			}
			source++
		}
		source %= len(p.data) / 4
		p.first[row], p.second[row] = floorColorPair(p.data[source*4:])
		source++
	}
	p.project(view)
	p.options.Uniforms["Phase"] = float32(p.clock.phase) / 6
	p.options.Uniforms["Sideways"] = float32(floorLateral.At(max(0, f.Time-FloorAt)).X)
	// Upload only 68 opaque colors. A second palette-composition render target
	// would copy these identical colors and force another mobile GPU pass.
	for row := 0; row < 34; row++ {
		for bank, paint := range [2]color.NRGBA{p.first[row], p.second[row]} {
			at := row*8 + bank*4
			p.pixels[at], p.pixels[at+1], p.pixels[at+2], p.pixels[at+3] = paint.R, paint.G, paint.B, paint.A
		}
	}
	p.banks.WritePixels(p.pixels[:])
	return nil
}

// The source alternates the palette banks to generate its perspective rows.
// Here geometry already determines both checker axes, so retain the colors
// while ordering each pair by brightness. A second screen-space parity would
// cancel rows into stripes and cut diagonals through rotating world cells.
func floorColorPair(data []byte) (color.NRGBA, color.NRGBA) {
	decode := func(raw []byte) color.NRGBA {
		w := binary.BigEndian.Uint16(raw)
		return color.NRGBA{R: uint8(w>>8&7) * 34, G: uint8(w>>4&7) * 34, B: uint8(w&7) * 34, A: 255}
	}
	a, b := decode(data), decode(data[2:])
	luminance := func(c color.NRGBA) int { return 2126*int(c.R) + 7152*int(c.G) + 722*int(c.B) }
	if luminance(a) > luminance(b) {
		a, b = b, a
	}
	return a, b
}

// Interpolate homogeneous world coordinates, not ordinary texture coordinates.
// Dividing by interpolated 1/Z in the shader keeps both triangles on the same
// perspective grid throughout the orbit, without subdividing the floor.
func (p *checkerboard) project(view sceneView) {
	p.vertices = p.vertices[:0]
	var corners [4]geometry.Vertex
	for i, point := range [4]geometry.Vec3{{X: -800, Z: -800}, {X: 800, Z: -800}, {X: 800, Z: 800}, {X: -800, Z: 800}} {
		corners[i] = geometry.Vertex{Position: view.point(point), UV: geometry.Vec2{X: point.X, Y: point.Z}}
	}
	for _, ids := range [2][3]int{{0, 1, 2}, {0, 2, 3}} {
		var scratch [4]geometry.Vertex
		polygon := geometry.ClipNear(scratch[:0], [3]geometry.Vertex{corners[ids[0]], corners[ids[1]], corners[ids[2]]}, view.camera.Near)
		for j := 1; j+1 < len(polygon); j++ {
			for _, corner := range [3]geometry.Vertex{polygon[0], polygon[j], polygon[j+1]} {
				pixel, _, _ := view.camera.Project(corner.Position)
				pixel.Y = 160 + (pixel.Y-160)*p.open
				q := 1 / corner.Position.Z
				p.vertices = append(p.vertices, ebiten.Vertex{DstX: float32(pixel.X), DstY: float32(pixel.Y), SrcX: float32(pixel.X), SrcY: float32(pixel.Y),
					Custom0: float32(corner.UV.X * q), Custom1: float32(corner.UV.Y * q), Custom2: float32(q)})
			}
		}
	}
}

var floorIndices = [...]uint16{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

func (p *checkerboard) Draw(dst *ebiten.Image) {
	dst.DrawTrianglesShader(p.vertices, floorIndices[:len(p.vertices)], p.shader, &p.options)
}
func (p *checkerboard) Close() error {
	if p.shader != nil {
		p.shader.Deallocate()
		p.shader = nil
	}
	if p.banks != nil {
		p.banks.Deallocate()
		p.banks = nil
	}
	return nil
}

// DCK projects and clips the plane. This material recovers world coordinates
// per pixel; no affine UV stretch or mesh diagonal affects checker parity.
const floorShader = `//kage:unit pixels
package main
var Phase float
var Sideways float
func Fragment(position vec4, source vec2, color vec4, world vec4) vec4 {
	point := world.xy/world.z
	bank := mod(floor(point.x/110+Sideways)+floor(point.y/110+Phase), 2)
	row := clamp(floor((source.y-imageSrc0Origin().y-180)*34/220),0,33)
	return imageSrc0At(imageSrc0Origin()+vec2(bank+0.5,row+0.5))
}
`
