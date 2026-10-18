// Command video exports the complete presentation and its original soundtrack
// through DCK, then prepares a WebM copy and a gallery of lossless screenshots.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/democonstructionkit/video"
	"github.com/olivierh59500/dma40/presentation"
)

func main() {
	config := video.Config{Output: "recordings/dma40-complete/dma40.mp4", Title: "DMA40 Megademo", Width: 1280, Height: 800,
		FPS: presentation.FPS, TPS: presentation.FPS, SampleRate: 48000, PosterAt: 18 * time.Second}
	config.Flags(flag.CommandLine)
	webm := flag.Bool("webm", true, "also encode a VP9/Opus WebM from the completed MP4")
	shots := flag.String("screenshots", "", "lossless screenshot directory; defaults to screenshots beside the video")
	flag.Parse()
	if *shots == "" {
		*shots = filepath.Join(filepath.Dir(config.Output), "screenshots")
	}
	if err := os.MkdirAll(*shots, 0755); err != nil {
		log.Fatal(err)
	}
	var capture *recording
	err := video.Run(config, func() (ebiten.Game, error) {
		game, err := presentation.New(presentation.Options{Deterministic: true})
		if err != nil {
			return nil, err
		}
		capture = &recording{Game: game, directory: *shots, shots: screenshotPlan(), width: config.Width, height: config.Height}
		return capture, nil
	})
	if err != nil {
		log.Fatal(err)
	}
	if capture == nil {
		log.Fatal("the presentation was not created")
	}
	if config.Duration == 0 {
		for _, shot := range capture.shots {
			if !shot.saved {
				log.Fatalf("the completed recording missed screenshot %s", shot.File)
			}
		}
	}
	webmPath := ""
	if *webm {
		webmPath = strings.TrimSuffix(config.Output, filepath.Ext(config.Output)) + ".webm"
		fmt.Fprintln(os.Stderr, "Encoding the WebM copy...")
		cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-n", "-i", config.Output,
			"-map", "0:v:0", "-map", "0:a:0", "-map_metadata", "0", "-map_chapters", "0",
			"-c:v", "libvpx-vp9", "-crf", "26", "-b:v", "0", "-deadline", "good", "-cpu-used", "4",
			"-row-mt", "1", "-threads", "4", "-pix_fmt", "yuv420p", "-c:a", "libopus", "-b:a", "192k", webmPath)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
	if err := capture.writeGallery(config.Output, webmPath); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "Videos: %s\nGallery: %s\n", filepath.Dir(config.Output), filepath.Join(filepath.Dir(config.Output), "index.html"))
}
