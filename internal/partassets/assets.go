// Package partassets verifies the authorized media for the registered demo parts.
package partassets

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
)

var Bundle []byte

//go:embed checksums.json
var checksums []byte

type Set struct {
	Files fs.FS
	Music []byte
}

func Load() (*Set, error) {
	if len(Bundle) == 0 {
		return nil, fmt.Errorf("demo media is missing; run go run ./cmd/prepare-parts -demos <directory> -music <Union-Demo-Mega-Apocalypse.ym>")
	}
	var manifest struct {
		Bundle string            `json:"bundle_sha256"`
		Files  map[string]string `json:"files_sha256"`
	}
	if err := json.Unmarshal(checksums, &manifest); err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(Bundle)) != manifest.Bundle {
		return nil, fmt.Errorf("demo archive checksum mismatch")
	}
	z, err := zip.NewReader(bytes.NewReader(Bundle), int64(len(Bundle)))
	if err != nil {
		return nil, err
	}
	if len(z.File) != len(manifest.Files) {
		return nil, fmt.Errorf("unexpected demo resource count")
	}
	seen := make(map[string]bool)
	for _, file := range z.File {
		if !fs.ValidPath(file.Name) || seen[file.Name] || file.UncompressedSize64 > 16<<20 {
			return nil, fmt.Errorf("invalid demo resource %q", file.Name)
		}
		seen[file.Name] = true
		r, e := file.Open()
		if e != nil {
			return nil, e
		}
		data, e := io.ReadAll(io.LimitReader(r, 16<<20))
		r.Close()
		if e != nil {
			return nil, e
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != manifest.Files[file.Name] {
			return nil, fmt.Errorf("demo checksum mismatch: %s", file.Name)
		}
	}
	music, err := fs.ReadFile(z, "music.ym")
	if err != nil {
		return nil, err
	}
	return &Set{Files: z, Music: music}, nil
}
