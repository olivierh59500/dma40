package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/dma40/city"
	"github.com/olivierh59500/dma40/internal/sequence"
	"github.com/olivierh59500/dma40/internal/startup"
	"github.com/olivierh59500/dma40/jelly"
	"github.com/olivierh59500/dma40/presentation"
	"github.com/olivierh59500/dma40/quad"
	"github.com/olivierh59500/dma40/spoon"
)

type screenshot struct {
	File    string  `json:"file"`
	Title   string  `json:"title"`
	Stage   string  `json:"stage"`
	Tick    int     `json:"tick"`
	Seconds float64 `json:"seconds"`
	at      int
	before  int
	saved   bool
	region  image.Rectangle
}

type recording struct {
	*presentation.Game
	directory     string
	shots         []screenshot
	width, height int
	surface       *ebiten.Image
	err           error
}

// Step bypasses interactive input so a recording cannot skip a scene when the
// export window has focus. DCK owns the fixed simulation and audio clock.
func (r *recording) Update() error {
	if r.err != nil {
		return r.err
	}
	return r.Game.Step()
}

func (r *recording) Draw(dst *ebiten.Image) {
	r.Game.Draw(dst)
	if r.err != nil {
		return
	}
	state := r.Snapshot()
	for i := range r.shots {
		shot := &r.shots[i]
		if shot.saved || shot.Stage != state.Stage {
			continue
		}
		target := shot.at
		if shot.before != 0 {
			target = r.JellyStart() - (state.Tick - state.LocalTick) - shot.before
		}
		if state.LocalTick < target {
			continue
		}
		if err := r.save(dst, shot, state); err != nil {
			r.err = err
			return
		}
	}
}

func (r *recording) save(source *ebiten.Image, shot *screenshot, state presentation.State) error {
	if !shot.region.Empty() {
		source = source.SubImage(shot.region).(*ebiten.Image)
	}
	if r.surface == nil {
		r.surface = ebiten.NewImage(r.width, r.height)
	}
	r.surface.Clear()
	var options ebiten.DrawImageOptions
	options.GeoM.Scale(float64(r.width)/float64(source.Bounds().Dx()), float64(r.height)/float64(source.Bounds().Dy()))
	r.surface.DrawImage(source, &options)
	frame := image.NewRGBA(r.surface.Bounds())
	r.surface.ReadPixels(frame.Pix)
	path := filepath.Join(r.directory, shot.File)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err := png.Encode(file, frame); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	shot.saved, shot.Tick, shot.Seconds = true, state.Tick, float64(state.Tick-1)/presentation.FPS
	fmt.Fprintf(os.Stderr, "Screenshot %.2fs: %s\n", shot.Seconds, shot.Title)
	return nil
}

func (r *recording) Close() {
	if r.surface != nil {
		r.surface.Deallocate()
		r.surface = nil
	}
	r.Game.Close()
}

func (r *recording) RecordingChapter() string {
	state := r.Snapshot()
	if state.Stage == sequence.Opening {
		for _, cue := range []struct {
			at    float64
			title string
		}{
			{startup.ScrollAt, "Opening: chrome scroll"}, {startup.RastersAt, "Opening: rasters"},
			{startup.BobsAt, "Opening: bobs"}, {startup.LogoAt, "Opening: DMA logo"}, {0, "Opening: cube"},
		} {
			if float64(state.LocalTick)/presentation.FPS >= cue.at {
				return cue.title
			}
		}
	}
	if title, ok := chapterTitles[state.Effect]; ok {
		return title
	}
	return state.Effect
}

func screenshotPlan() []screenshot {
	var shots []screenshot
	add := func(stage, slug, title string, seconds float64) {
		shots = append(shots, screenshot{File: fmt.Sprintf("%02d-%s.png", len(shots)+1, slug), Title: title, Stage: stage, at: int(math.Round(seconds * presentation.FPS))})
	}
	add(sequence.Opening, "opening-cube", "Opening: moving cube", .9)
	add(sequence.Opening, "opening-logo", "Opening: DMA logo", 3.3)
	add(sequence.Opening, "opening-bobs", "Opening: staggered bobs", 8.4)
	add(sequence.Opening, "opening-rasters", "Opening: warped rasters", 12)
	add(sequence.Opening, "opening-scroll", "Opening: chrome scroll", 18)
	add(sequence.FourLoader, "four-loader", "Union transition: four demos", 4.3)
	add(sequence.Four, "four-panels", "Four complete demo panels", 5)
	regions := quad.Regions()
	for index, panel := range []struct{ slug, title string }{
		{"lockbuster", "Lockbuster 3"}, {"raster-master", "Jinx Raster Master"},
		{"chonchon", "Chonchon"}, {"dma-is-back", "DMA is Back"},
	} {
		add(sequence.Four, panel.slug, "Four panels: "+panel.title, 3)
		shots[len(shots)-1].region = regions[index]
	}
	add(sequence.Four, "four-traveling-sprites", "Sprites crossing the four panels", 20)
	add(sequence.FourFade, "four-fade", "Four panels: fade to black", .65)
	add(sequence.SpoonLoader, "spoon-loader", "Union transition: There Is No Spoon", 4.3)
	add(sequence.Spoon, "vectorballs", "Multidirectional vectorball flight", 3)
	add(sequence.Spoon, "spoon-gathering", "Vectorballs gathering into the spoon", 8)
	add(sequence.Spoon, "ball-spoon", "The complete vectorball spoon", spoon.GatherAt+.6)
	add(sequence.Spoon, "metal-reveal", "The spoon becomes metal", spoon.SpoonAt+1)
	add(sequence.Spoon, "metal-spoon", "Low-poly metal spoon", spoon.SpoonSolidAt+.6)
	add(sequence.Spoon, "bent-spoon", "Side-view spoon bending", spoon.BendAt+5)
	add(sequence.Spoon, "volumetric-flight", "Volumetric lattice flight", spoon.FlightAt+7)
	add(sequence.Spoon, "matrix", "Matrix digital rain", spoon.MatrixAt+4)
	add(sequence.Spoon, "secrets-reveal", "Scrambling text reveal", spoon.SecretsAt+7)
	add(sequence.Spoon, "secrets-readable", "The decoded tribute", spoon.PacmanAt-.8)
	add(sequence.Spoon, "pacmen", "Alternating Pac-Men clear every row", spoon.PacmanAt+1.9)
	add(sequence.Spoon, "circle-water", "Circular scroll and water reflection", spoon.CircleAt+1.8)
	add(sequence.Spoon, "circle-logo", "Circular scroll: DMA logo entrance", spoon.CircleLogoAt+2.6)
	add(sequence.Spoon, "circle-magnifier", "Circular scroll: live magnifier", spoon.CircleLensAt+4)
	add(sequence.Spoon, "circle-magnifier-contours", "Circular scroll: magnified original contours", spoon.CircleLensAt+8)
	add(sequence.Spoon, "circle-magnifier-water", "Circular scroll: magnifier and water", spoon.CircleLensAt+15)
	add(sequence.Spoon, "pixel-car-morph", "Circular pixels form the car", spoon.CityAt()+1.8)
	add(sequence.Spoon, "delorean-city", "DeLorean-inspired coupe in the city", spoon.CityAt()+9)
	add(sequence.Spoon, "car-question", "Elon, does it fly?", spoon.CityAt()+city.AskAt+1)
	add(sequence.Spoon, "elon-reply", "Elon's comic reply", spoon.CityAt()+city.ReplyAt+2)
	add(sequence.Spoon, "car-farewell", "The driver's farewell", spoon.CityAt()+city.ByeAt+2)
	add(sequence.Spoon, "flying-car", "Rocket-powered flight through the city", spoon.CityAt()+city.LaunchAt+4)
	add(sequence.Spoon, "dma-rooftops", "DMA formed by the city rooftops", spoon.CityAt()+city.RooftopAt+.5)
	add(sequence.Spoon, "city-dma-portal", "DMA portal into Noname", spoon.CityAt()+city.PortalAt+1.5)
	add(sequence.Spoon, "noname", "Noname: DMA members and equalizer", spoon.FinaleAt()+8.4)
	add(sequence.Spoon, "noname-greetings", "Noname: greetings to the demoscene", spoon.FinaleAt()+20)
	add(sequence.Spoon, "noname-lateam", "Noname: the French LATEAM invitation", spoon.FinaleAt()+34)
	add(sequence.Spoon, "dma-logo-handoff", "Noname: DMA logo settles for Jelly", 0)
	shots[len(shots)-1].before = 55
	add(sequence.Spoon, "dma-logo-still", "The shared DMA logo still", 0)
	shots[len(shots)-1].before = 5
	j := func(slug, title string, seconds float64) {
		add(sequence.Jelly, slug, title, seconds-float64(jelly.EntryTick)/presentation.FPS)
	}
	j("jelly-flash", "DMA logo: transformation flash", jelly.FlashAt+.12)
	j("jelly-glenz", "The DMA glenz logo", jelly.FlashAt+1.5)
	j("jelly-checkerboard", "The checkerboard enters", jelly.FloorAt+1.2)
	j("jelly-point-sphere", "The enclosing point sphere", jelly.SphereAt+1)
	j("jelly-enclosure", "Shared jelly deformation", jelly.SphereJellyAt+1.5)
	j("jelly-drop", "The enclosed logo drops", jelly.DropAt+.6)
	j("jelly-bounce", "Jelly contact and rebound", jelly.OrbitAt-1)
	j("jelly-orbit", "The orbiting 3D checkerboard scene", jelly.OrbitAt+5)
	j("flock-dissolve", "The scene dissolves into birds", jelly.BoidsAt+.5)
	j("flock-flight", "Flocking: group flight", jelly.BoidsAt+3.5)
	j("flock-turns", "Flocking: coordinated turns", jelly.BoidsAt+8)
	j("flock-depth", "Flocking: depth and regrouping", jelly.BoidsAt+12)
	j("portrait-formation", "Birds assemble the portrait", jelly.FlockGatherAt+2.5)
	j("portrait-bounce", "The portrait rebounds like jelly", jelly.PortraitAt+1.5)
	j("portrait-position", "The portrait settles to the right", jelly.PortraitMoveAt+1.5)
	j("portrait-decipher", "The farewell deciphers", jelly.GreetingAt+3)
	j("audience-question", "Bilizir from DMA: is the audience listening?", jelly.GreetingSettledAt+1)
	j("pi-symbol", "The Matrix-style pi symbol", jelly.PiAt+.45)
	j("pi-cursor", "The cursor approaches pi", jelly.PiCursorAt+1.8)
	j("pi-click", "The cursor clicks pi", jelly.PiClickAt+.16)
	j("pi-decipher", "The pi page deciphers", jelly.PiPageAt+2)
	j("pi-complete", "The complete page of pi", float64(jelly.PiSettledTick())/presentation.FPS+1)
	return shots
}

var chapterTitles = map[string]string{
	sequence.FourLoader: "Union transition: four demos", sequence.Four: "Four demos and traveling sprites",
	sequence.FourFade: "Four demos: fade to black", sequence.SpoonLoader: "Union transition: There Is No Spoon",
	"vectorballs": "Vectorball flight", "pull-back": "The balls gather", "ball-spoon": "Vectorball spoon",
	"metal-spoon": "The metal spoon", "spoon-bend": "There Is No Spoon", "voxel-flight": "Volumetric flight",
	"matrix": "Matrix rain", "no-more-secrets": "No More Secrets", "pacman": "The Pac-Men sweep",
	"fade-to-black": "Into the circular scroll", "circular-scroll": "Circular scroll and water",
	"magnified-scroll": "Circular scroll, DMA and magnifier", "pixels-to-roadster": "Pixels become a DeLorean",
	"roadster-drive": "The city drive", "roadster-question": "Elon, does it fly?", "elon-reply": "Elon's reply",
	"roadster-farewell": "See you soon, Elon", "roadster-countdown": "Preparing for flight", "roadster-flight": "The flying car",
	"dma-rooftops": "DMA rooftops", "dma-city-portal": "Through DMA to Noname", "noname-finale": "Noname",
	"dma-logo-handoff": "The DMA logo handoff", "dma-logo": "DMA: the shared still", "logo-flash": "DMA: transformation",
	"glenz-logo": "The glenz logo", "checkerboard-drop": "The falling checkerboard", "point-sphere": "The point sphere",
	"jelly-enclosure": "The jelly enclosure", "jelly-drop": "The jelly drop", "jelly-bounce": "Jelly rebound",
	"jelly-orbit": "The 360-degree orbit", "boids": "Flocking birds", "portrait-formation": "The birds form a portrait",
	"portrait-bounce": "The portrait rebounds", "portrait-position": "The portrait settles", "portrait-decipher": "The farewell deciphers",
	"portrait-greeting": "Is the audience listening?", "pi-symbol": "Pi", "pi-cursor": "The cursor approaches",
	"pi-click": "Clicking pi", "pi-decipher": "Deciphering pi", "pi-hold": "The complete pi page and musical fade",
}
