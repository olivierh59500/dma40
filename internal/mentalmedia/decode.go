// Package mentalmedia reconstructs the authorized Mental Hangover demo artwork.
// The layouts follow the native source port; no executable data is packaged.
package mentalmedia

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

type Point [2]byte
type Circle struct {
	Glyphs        map[byte][][]Point
	Wave, Profile []int8
}

type bank []byte

func (b bank) read(address, count int) ([]byte, error) {
	offset := address - 0x9000
	if offset < 0 || count < 0 || offset > len(b)-count {
		return nil, fmt.Errorf("mental artwork: truncated data at %#x", address)
	}
	return b[offset : offset+count], nil
}

func pngBytes(im image.Image) ([]byte, error) {
	var out bytes.Buffer
	err := png.Encode(&out, im)
	return out.Bytes(), err
}

// Decode reads cylinder 46's two-plane sphere bank and cylinder 44's vector font
// only. These are the same bounded addresses used by the source Go port.
func Decode(contact, circle []byte) (map[string][]byte, error) {
	balls, err := spheres(bank(contact))
	if err != nil {
		return nil, err
	}
	font, err := outlines(bank(circle))
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte)
	result["spoon/balls.png"], err = pngBytes(balls)
	if err != nil {
		return nil, err
	}
	result["spoon/circle.json"], err = json.Marshal(font)
	return result, err
}

func spheres(b bank) (*image.NRGBA, error) {
	// The first bank carries the white highlight and blue shading. The second
	// repeats the silhouette in both planes and supplies transparency only.
	art, err := b.read(0xb596, 1024)
	if err != nil {
		return nil, err
	}
	mask, err := b.read(0xb996, 1024)
	if err != nil {
		return nil, err
	}
	pal := [4]color.NRGBA{{}, {255, 255, 255, 255}, {51, 68, 187, 255}, {0, 17, 119, 255}}
	im := image.NewNRGBA(image.Rect(0, 0, 256, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 256; x++ {
			bit := uint(7 - x%8)
			at := y*64 + x/8
			if ((mask[at]>>bit)&1)|((mask[at+32]>>bit)&1) == 0 {
				continue
			}
			index := (art[at]>>bit)&1 | ((art[at+32]>>bit)&1)<<1
			c := pal[index]
			c.A = 255
			im.SetNRGBA(x, y, c)
		}
	}
	return im, nil
}

func outlines(b bank) (Circle, error) {
	c := Circle{Glyphs: make(map[byte][][]Point)}
	fail := func(err error) (Circle, error) { return Circle{}, err }
	for _, address := range []int{0x9d12, 0x9e12} {
		raw, err := b.read(address, 256)
		if err != nil {
			return fail(err)
		}
		wave := make([]int8, len(raw))
		for i, v := range raw {
			wave[i] = int8(v)
		}
		if address == 0x9d12 {
			c.Wave = wave
		} else {
			c.Profile = wave
		}
	}
	for i := 0; i < 59; i++ {
		raw, err := b.read(0x9958+i*4, 4)
		if err != nil {
			return fail(err)
		}
		address := int(binary.BigEndian.Uint32(raw))
		header, err := b.read(address, 1)
		if err != nil {
			return fail(err)
		}
		count := int(header[0])
		address++
		if count == 0 {
			c.Glyphs[byte(i+32)] = nil
			continue
		}
		var contours [][]Point
		var contour []Point
		readPoint := func() (Point, error) {
			v, e := b.read(address, 2)
			if e != nil {
				return Point{}, e
			}
			address += 2
			if v[0] >= 128 || v[1] > 10 {
				return Point{}, fmt.Errorf("mental artwork: invalid contour point")
			}
			return Point{v[0], v[1]}, nil
		}
		p, err := readPoint()
		if err != nil {
			return fail(err)
		}
		contour = append(contour, p)
		for edge, separators := 0, 0; edge < count; {
			marker, err := b.read(address, 1)
			if err != nil {
				return fail(err)
			}
			if marker[0] >= 128 {
				separators++
				if separators > 64 {
					return fail(fmt.Errorf("mental artwork: excessive contours"))
				}
				contours = append(contours, contour)
				contour = nil
				address++
				p, err = readPoint()
				if err != nil {
					return fail(err)
				}
				contour = append(contour, p)
				continue
			}
			p, err = readPoint()
			if err != nil {
				return fail(err)
			}
			contour = append(contour, p)
			edge++
		}
		contours = append(contours, contour)
		for _, path := range contours {
			if len(path) < 4 || path[0] != path[len(path)-1] {
				return fail(fmt.Errorf("mental artwork: open contour for %q", rune(i+32)))
			}
		}
		c.Glyphs[byte(i+32)] = contours
	}
	return c, nil
}
