// Package portraitassets loads the supplied portrait without altering its pixels.
package portraitassets

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
)

// Portrait is assigned by the generated embedding bridge.
var Portrait []byte

//go:embed checksums.json
var checksums []byte

// Load checks the original JPEG before decoding it. Runtime playback never
// depends on the source image's location outside the application.
func Load() (image.Image, error) {
	if len(Portrait) == 0 {
		return nil, fmt.Errorf("portrait missing; run go run ./cmd/prepare-portrait -image <supplied-portrait.jpeg>")
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
		return nil, fmt.Errorf("portrait checksum mismatch")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(Portrait))
	if err != nil {
		return nil, err
	}
	if config.Width != manifest.Width || config.Height != manifest.Height || config.Width > 2048 || config.Height > 2048 {
		return nil, fmt.Errorf("unexpected portrait dimensions")
	}
	return jpeg.Decode(bytes.NewReader(Portrait))
}
