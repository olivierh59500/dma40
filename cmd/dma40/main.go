// Command dma40 plays the standalone automatic DMA40 presentation.
package main

import (
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/dma40/presentation"
)

func main() {
	mute := flag.Bool("mute", false, "disable music and the Union loader sound")
	part := flag.String("part", "opening", "preview opening, four, spoon, flight, circle, city, noname, handoff, jelly, boids, portrait, greeting, pi or pi-page")
	start := flag.Duration("start", 0, "preview position within the selected part")
	quad := flag.Duration("quad-duration", 0, "four-panel hold override; zero completes one visitor tour")
	noname := flag.Duration("noname-duration", 0, "Noname hold override; zero uses the fixed forty-second hold")
	fullscreen := flag.Bool("fullscreen", false, "start in fullscreen")
	frames := flag.Int("frames", 0, "optional finite update count for a smoke check")
	capture := flag.String("capture", "", "optional PNG path at the end of a finite smoke check")
	flag.Parse()
	if *frames < 0 || *capture != "" && *frames == 0 {
		log.Fatal("a capture requires a positive -frames count")
	}
	ebiten.SetTPS(presentation.FPS)
	ebiten.SetWindowSize(960, 600)
	ebiten.SetWindowTitle("DMA40")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(*fullscreen)
	game, err := presentation.New(presentation.Options{Mute: *mute, StartPart: *part, StartTime: *start, QuadDuration: *quad, NonameDuration: *noname, Deterministic: *frames > 0})
	if err != nil {
		log.Fatal(err)
	}
	defer game.Close()
	if err := ebiten.RunGame(&finite{Game: game, limit: *frames, capture: *capture}); err != nil {
		log.Fatal(err)
	}
}

type finite struct {
	*presentation.Game
	limit, ticks int
	capture      string
	saved        bool
}

func (g *finite) Update() error {
	if g.limit > 0 && g.ticks >= g.limit {
		if g.capture == "" || g.saved {
			return ebiten.Termination
		}
		// Window initialization can request several updates before drawing.
		// Hold the requested frame until its screenshot has been saved.
		return nil
	}
	g.ticks++
	return g.Game.Update()
}

func (g *finite) Draw(dst *ebiten.Image) {
	g.Game.Draw(dst)
	if g.capture == "" || g.saved || g.ticks < g.limit {
		return
	}
	img := image.NewRGBA(dst.Bounds())
	dst.ReadPixels(img.Pix)
	if err := os.MkdirAll(filepath.Dir(g.capture), 0755); err != nil {
		log.Fatal(err)
	}
	file, err := os.Create(g.capture)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		log.Fatal(err)
	}
	if err := file.Close(); err != nil {
		log.Fatal(err)
	}
	g.saved = true
}
