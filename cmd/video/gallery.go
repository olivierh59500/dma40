package main

import (
	"archive/zip"
	"encoding/json"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (r *recording) writeGallery(mp4, webm string) error {
	var saved []screenshot
	for _, shot := range r.shots {
		if shot.saved {
			saved = append(saved, shot)
		}
	}
	manifest, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(r.directory, "manifest.json"), append(manifest, '\n'), 0644); err != nil {
		return err
	}
	output := filepath.Dir(mp4)
	relative, err := filepath.Rel(output, r.directory)
	if err != nil {
		return err
	}
	data := struct {
		MP4, WebM, Poster, Screenshots string
		Shots                          []screenshot
	}{MP4: filepath.Base(mp4), WebM: filepath.Base(webm), Poster: strings.TrimSuffix(filepath.Base(mp4), filepath.Ext(mp4)) + ".png",
		Screenshots: filepath.ToSlash(relative), Shots: saved}
	if webm == "" {
		data.WebM = ""
	}
	file, err := os.Create(filepath.Join(output, "index.html"))
	if err != nil {
		return err
	}
	err = template.Must(template.New("gallery").Parse(galleryHTML)).Execute(file, data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	archive, err := os.OpenFile(filepath.Join(output, "screenshots.zip"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer archive.Close()
	writer := zip.NewWriter(archive)
	for _, shot := range saved {
		if err := addPNG(writer, filepath.Join(r.directory, shot.File), shot.File); err != nil {
			writer.Close()
			return err
		}
	}
	entry, err := writer.Create("manifest.json")
	if err != nil {
		writer.Close()
		return err
	}
	if _, err := entry.Write(append(manifest, '\n')); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}

func addPNG(writer *zip.Writer, path, name string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}

const galleryHTML = `<!doctype html>
<html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>DMA40 — Complete presentation</title>
<style>
body{margin:0;background:#080b13;color:#e2e9f7;font:16px system-ui,sans-serif}main{max-width:1400px;margin:32px auto;padding:0 24px}
h1{font-size:32px}a{color:#8edcff}video{display:block;width:100%;aspect-ratio:8/5;max-height:75vh;background:#000}
.downloads{display:flex;gap:24px;margin:20px 0 36px;flex-wrap:wrap}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(300px,1fr));gap:22px}
figure{margin:0;background:#141b2a;border-radius:8px;overflow:hidden}img{display:block;width:100%;aspect-ratio:8/5;image-rendering:pixelated}
figcaption{padding:12px 16px}.time{display:block;color:#99abc5;font-size:13px;margin-top:6px}
</style><main><h1>DMA40 — Complete presentation</h1>
<p>The complete production, with its own soundtrack. Videos: 1280 × 800, 50 FPS. Screenshots: lossless PNG.</p>
<video controls preload="metadata" poster="{{.Poster}}"><source src="{{.MP4}}" type="video/mp4">{{if .WebM}}<source src="{{.WebM}}" type="video/webm">{{end}}</video>
<p class="downloads"><a href="{{.MP4}}" download>Download MP4</a>{{if .WebM}}<a href="{{.WebM}}" download>Download WebM</a>{{end}}<a href="screenshots.zip" download>Download all screenshots</a></p>
<h2>Scenes and effects</h2><div class="grid">{{range .Shots}}<figure><a href="{{$.Screenshots}}/{{.File}}"><img loading="lazy" src="{{$.Screenshots}}/{{.File}}" alt="{{.Title}}"></a>
<figcaption>{{.Title}}<span class="time">{{printf "%.2f" .Seconds}} s · {{.Stage}}</span></figcaption></figure>{{end}}</div></main></html>`
