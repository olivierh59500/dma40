//go:build !jellyprofile && !openingprofile && !quadprofile && !spoonprofile && !flightprofile && !boidsprofile && !portraitprofile && !ballsprofile && !cityprofile

// Package mobile exposes the same automatic Go presentation to Android.
package mobile

import (
	"encoding/json"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/mobile"
	"github.com/olivierh59500/dma40/presentation"
)

// Preview values are optional linker inputs used only by development APKs.
var previewPart, previewSeconds string
var state atomic.Value

func init() {
	ebiten.SetTPS(presentation.FPS)
	seconds := 0.0
	if previewSeconds != "" {
		var err error
		seconds, err = strconv.ParseFloat(previewSeconds, 64)
		if err != nil {
			log.Fatal(err)
		}
	}
	game, err := presentation.New(presentation.Options{StartPart: previewPart, StartTime: time.Duration(seconds * float64(time.Second)), OnState: func(snapshot presentation.State) {
		raw, _ := json.Marshal(snapshot)
		state.Store(string(raw))
	}})
	if err != nil {
		log.Fatal(err)
	}
	mobile.SetGame(game)
}

func Start() {}
func State() string {
	if value := state.Load(); value != nil {
		return value.(string)
	}
	return "{}"
}
