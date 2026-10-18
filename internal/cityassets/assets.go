// Package cityassets verifies the locally prepared comic portrait.
package cityassets

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
)

var Portrait []byte

//go:embed checksums.json
var checksums []byte

func Load() (image.Image, error) {
	if len(Portrait) == 0 {
		return nil, fmt.Errorf("comic portrait missing; run go run ./cmd/prepare-city -image <elon-comic.png>")
	}
	var manifest struct {
		SHA256 string `json:"sha256"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err := json.Unmarshal(checksums, &manifest); err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(Portrait)) != manifest.SHA256 {
		return nil, fmt.Errorf("comic portrait checksum mismatch")
	}
	config, err := png.DecodeConfig(bytes.NewReader(Portrait))
	if err != nil {
		return nil, err
	}
	if config.Width != manifest.Width || config.Height != manifest.Height || config.Width > 2048 || config.Height > 2048 {
		return nil, fmt.Errorf("unexpected comic portrait dimensions")
	}
	return png.Decode(bytes.NewReader(Portrait))
}
