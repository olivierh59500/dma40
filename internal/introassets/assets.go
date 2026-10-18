// Package introassets loads the explicitly authorized opening resources.
package introassets

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"io"
)

// Bundle is assigned by the generated Go embedding bridge.
var Bundle []byte

//go:embed checksums.json
var checksums []byte

// Set borrows decoded artwork and the original Leonard Intro DX YM stream.
type Set struct {
	Logo, Ball, Chrome, Bars, FontGrey, UnionFont image.Image
	Music, LoaderSound                            []byte
}

// Load checks every file before any GPU or audio object is constructed.
func Load() (*Set, error) {
	if len(Bundle) == 0 {
		return nil, fmt.Errorf("opening resources are missing; run go run ./cmd/prepare-intro -logo <transparent-logo.png> -demos <demos-directory>")
	}
	var manifest struct {
		Bundle string            `json:"bundle_sha256"`
		Files  map[string]string `json:"files_sha256"`
	}
	if err := json.Unmarshal(checksums, &manifest); err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(Bundle)) != manifest.Bundle {
		return nil, fmt.Errorf("opening resource archive checksum mismatch")
	}
	z, err := zip.NewReader(bytes.NewReader(Bundle), int64(len(Bundle)))
	if err != nil {
		return nil, err
	}
	if len(z.File) != 8 {
		return nil, fmt.Errorf("unexpected opening resource count")
	}
	files := make(map[string][]byte)
	for _, f := range z.File {
		if f.UncompressedSize64 > 8<<20 {
			return nil, fmt.Errorf("oversized opening resource %s", f.Name)
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(r, 8<<20))
		r.Close()
		if err != nil {
			return nil, err
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != manifest.Files[f.Name] {
			return nil, fmt.Errorf("opening checksum mismatch: %s", f.Name)
		}
		if _, ok := files[f.Name]; ok {
			return nil, fmt.Errorf("duplicate opening resource")
		}
		files[f.Name] = data
	}
	s := &Set{Music: files["music.ym"], LoaderSound: files["loader.ogg"]}
	for name, target := range map[string]*image.Image{"logo.png": &s.Logo, "ball.png": &s.Ball, "chrome.png": &s.Chrome, "bars.png": &s.Bars, "font-grey.png": &s.FontGrey, "union-font.png": &s.UnionFont} {
		im, _, err := image.Decode(bytes.NewReader(files[name]))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		*target = im
	}
	return s, nil
}
