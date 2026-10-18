// Command android generates the disposable platform bridge and builds an APK.
// Presentation, rendering, input, audio and asset preparation remain in Go.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/olivierh59500/dma40/internal/introassets"
)

const applicationID = "com.olivierh.dma40"
const ebitenVersion = "v2.10.2"

// Ebitengine requires this Activity/View lifecycle bridge. It is regenerated
// from Go, contains no gameplay, and is never tracked as Java source.
const activity = `package com.olivierh.dma40;
import android.app.Activity;
import android.os.Bundle;
import android.view.View;
import android.view.WindowManager;
import com.olivierh.dma40.mobile.EbitenView;
public final class MainActivity extends Activity {
 private EbitenView game;
 @Override public void onCreate(Bundle state) {
  super.onCreate(state);
  getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
  game = new EbitenView(this) {
   @Override protected void onErrorOnGameUpdate(Exception error) {
    android.util.Log.e("DMA40", "Game update failed", error);
    MainActivity.this.finish();
   }
  };
  setContentView(game);
  immersive();
 }
 private void immersive() {
  getWindow().getDecorView().setSystemUiVisibility(
   View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY | View.SYSTEM_UI_FLAG_FULLSCREEN |
   View.SYSTEM_UI_FLAG_HIDE_NAVIGATION | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN |
   View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION | View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
 }
 @Override public void onWindowFocusChanged(boolean focus) {
  super.onWindowFocusChanged(focus); if (focus) immersive();
 }
 @Override protected void onPause() {
  if (game != null) game.suspendGame(); super.onPause();
 }
 @Override protected void onResume() {
  super.onResume(); if (game != null) game.resumeGame(); immersive();
 }
}
`

const manifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.olivierh.dma40">
 <uses-sdk android:minSdkVersion="23" android:targetSdkVersion="36" />
 <uses-feature android:glEsVersion="0x00030000" android:required="true" />
 <application android:label="DMA40" android:icon="@drawable/icon" android:debuggable="true" android:allowBackup="false" android:extractNativeLibs="false" android:theme="@android:style/Theme.Material.NoActionBar.Fullscreen">
  <activity android:name=".MainActivity" android:exported="true" android:screenOrientation="sensorLandscape" android:configChanges="orientation|screenSize|keyboardHidden" android:launchMode="singleTop">
   <intent-filter><action android:name="android.intent.action.MAIN" /><category android:name="android.intent.category.LAUNCHER" /></intent-filter>
  </activity>
 </application>
 <instrumentation android:name=".TouchProbe" android:targetPackage="com.olivierh.dma40" />
</manifest>
`

func main() {
	target := flag.String("target", "android/arm64,android/amd64", "ebitenmobile target architectures")
	out := flag.String("out", "dist/dma40-android.apk", "debug APK output path")
	sdkArg := flag.String("sdk", os.Getenv("ANDROID_HOME"), "Android SDK directory")
	ndkArg := flag.String("ndk", os.Getenv("ANDROID_NDK_HOME"), "Android NDK directory")
	install := flag.String("install", "", "optional ADB serial to install and launch")
	probe := flag.String("check", "", "optional ADB serial for a finite multitouch and lifecycle check")
	part := flag.String("part", "", "optional part preview in a development APK")
	start := flag.Duration("start", 0, "preview position within the selected part")
	flag.Parse()
	if *probe != "" {
		*part = "opening"
		*start = 0
	}
	if *start < 0 || *start > time.Hour {
		check(fmt.Errorf("preview time must be within 0-3600 seconds"))
	}
	switch *part {
	case "", "opening", "four-loader", "four", "four-fade", "spoon-loader", "spoon", "flight", "circle", "city", "noname", "handoff", "jelly", "boids", "portrait", "greeting", "pi", "pi-page":
	default:
		check(fmt.Errorf("unknown preview part %q", *part))
	}
	root, err := os.Getwd()
	check(err)
	check(os.Setenv("GOCACHE", filepath.Join(root, ".cache", "go-build")))
	moduleCache, err := exec.Command("go", "env", "GOMODCACHE").Output()
	check(err)
	check(os.Setenv("GOMODCACHE", strings.TrimSpace(string(moduleCache))))
	check(os.Setenv("GOPATH", filepath.Join(root, ".cache", "gopath")))
	home, err := os.UserHomeDir()
	check(err)
	sdk := existing(*sdkArg, "/opt/homebrew/share/android-commandlinetools", filepath.Join(home, "Library", "Android", "sdk"), filepath.Join(home, "Android", "Sdk"))
	java := existing(os.Getenv("JAVA_HOME"), "/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home", "/Applications/Android Studio.app/Contents/jbr/Contents/Home")
	ndk := *ndkArg
	if ndk == "" {
		ndk = latest(filepath.Join(sdk, "ndk"))
	}
	tools := latest(filepath.Join(sdk, "build-tools"))
	jar := filepath.Join(sdk, "platforms", "android-36", "android.jar")
	if _, err := os.Stat(jar); err != nil {
		check(fmt.Errorf("Android SDK platform 36 is required: %w", err))
	}
	for name, value := range map[string]string{"JAVA_HOME": java, "ANDROID_HOME": sdk, "ANDROID_SDK_ROOT": sdk, "ANDROID_NDK_HOME": ndk, "GOMOBILE": filepath.Join(root, ".cache", "gomobile")} {
		check(os.Setenv(name, value))
	}
	check(os.Setenv("PATH", filepath.Join(java, "bin")+string(os.PathListSeparator)+os.Getenv("PATH")))
	check(os.Setenv("CGO_LDFLAGS", strings.TrimSpace(os.Getenv("CGO_LDFLAGS")+" -Wl,-z,max-page-size=16384 -Wl,-z,common-page-size=16384")))
	work := filepath.Join(root, "android", "generated")
	check(os.MkdirAll(work, 0755))
	aar := filepath.Join(work, "dma40.aar")
	fmt.Println("Building the native Go/Ebitengine library...")
	linker := "-s -w"
	if *part != "" {
		linker += " -X github.com/olivierh59500/dma40/mobile.previewPart=" + *part + " -X github.com/olivierh59500/dma40/mobile.previewSeconds=" + strconv.FormatFloat(start.Seconds(), 'f', -1, 64)
	}
	bind := []string{"run", "github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile", "bind", "-target", *target, "-androidapi", "23", "-javapkg", applicationID, "-trimpath", "-ldflags=" + linker, "-o", aar}
	activitySource := activity
	run("go", append(bind, "./mobile")...)
	check(os.WriteFile(filepath.Join(work, "MainActivity.java"), []byte(activitySource), 0644))
	check(os.WriteFile(filepath.Join(work, "TouchProbe.java"), []byte(touchProbe), 0644))
	check(os.WriteFile(filepath.Join(work, "AndroidManifest.xml"), []byte(manifest), 0644))
	classes := filepath.Join(work, "classes")
	dex := filepath.Join(work, "dex")
	res := filepath.Join(work, "res", "drawable")
	for _, dir := range []string{classes, dex, res} {
		check(os.MkdirAll(dir, 0755))
	}
	z, err := zip.OpenReader(aar)
	check(err)
	defer z.Close()
	libs := map[string][]byte{}
	for _, f := range z.File {
		if f.Name == "classes.jar" {
			check(os.WriteFile(filepath.Join(work, "classes.jar"), readZip(f), 0644))
		}
		if strings.HasPrefix(f.Name, "jni/") && strings.HasSuffix(f.Name, ".so") {
			data := readZip(f)
			verifyELF(f.Name, data)
			libs["lib/"+strings.TrimPrefix(f.Name, "jni/")] = data
		}
	}
	if len(libs) == 0 {
		check(fmt.Errorf("AAR contains no native libraries"))
	}
	source, err := introassets.Load()
	check(err)
	icon := image.NewNRGBA(image.Rect(0, 0, 192, 192))
	bounds := source.Logo.Bounds()
	height := max(1, 192*bounds.Dy()/bounds.Dx())
	for y := 0; y < min(height, 192); y++ {
		for x := 0; x < 192; x++ {
			icon.Set(x, (192-height)/2+y, source.Logo.At(bounds.Min.X+x*bounds.Dx()/192, bounds.Min.Y+y*bounds.Dy()/height))
		}
	}
	f, err := os.Create(filepath.Join(res, "icon.png"))
	check(err)
	check(png.Encode(f, icon))
	check(f.Close())
	run(filepath.Join(java, "bin", "javac"), "-source", "8", "-target", "8", "-Xlint:-options", "-bootclasspath", jar, "-classpath", filepath.Join(work, "classes.jar"), "-d", classes, filepath.Join(work, "MainActivity.java"), filepath.Join(work, "TouchProbe.java"))
	args := []string{"--lib", jar, "--min-api", "23", "--output", dex, filepath.Join(work, "classes.jar")}
	check(filepath.WalkDir(classes, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".class") {
			args = append(args, path)
		}
		return nil
	}))
	run(filepath.Join(tools, "d8"), args...)
	flat := filepath.Join(work, "res.zip")
	run(filepath.Join(tools, "aapt2"), "compile", "--dir", filepath.Join(work, "res"), "-o", flat)
	base := filepath.Join(work, "base.apk")
	run(filepath.Join(tools, "aapt2"), "link", "-I", jar, "--manifest", filepath.Join(work, "AndroidManifest.xml"), "--min-sdk-version", "23", "--target-sdk-version", "36", "--version-code", "1", "--version-name", "0.1", "-o", base, flat)
	unsigned := filepath.Join(work, "unsigned.apk")
	libraryCount := len(libs)
	check(packageAPK(base, unsigned, dex, libs))
	check(os.MkdirAll(filepath.Dir(*out), 0755))
	aligned := filepath.Join(work, "aligned.apk")
	run(filepath.Join(tools, "zipalign"), "-f", "-P", "16", "4", unsigned, aligned)
	key := filepath.Join(root, ".cache", "android-debug.keystore")
	if _, err := os.Stat(key); os.IsNotExist(err) {
		run(filepath.Join(java, "bin", "keytool"), "-genkeypair", "-keystore", key, "-storepass", "android", "-keypass", "android", "-alias", "androiddebugkey", "-dname", "CN=Android Debug,O=Android,C=US", "-keyalg", "RSA", "-keysize", "2048", "-validity", "10000")
	}
	run(filepath.Join(tools, "apksigner"), "sign", "--ks", key, "--ks-pass", "pass:android", "--key-pass", "pass:android", "--out", *out, aligned)
	run(filepath.Join(tools, "apksigner"), "verify", "--verbose", *out)
	run(filepath.Join(tools, "zipalign"), "-c", "-P", "16", "4", *out)
	data, err := os.ReadFile(*out)
	check(err)
	check(os.WriteFile(*out+".sha256", []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(data), filepath.Base(*out))), 0644))
	fmt.Printf("Built %s (debug signed, %d native libraries).\n", *out, libraryCount)
	if *install != "" {
		adb := filepath.Join(sdk, "platform-tools", "adb")
		run(adb, "-s", *install, "install", "-r", *out)
		run(adb, "-s", *install, "shell", "am", "start", "-W", "-S", "-n", applicationID+"/.MainActivity")
	}
	if *probe != "" {
		check(checkDevice(filepath.Join(sdk, "platform-tools", "adb"), *probe, *out))
	}
}

func packageAPK(base, out, dex string, libs map[string][]byte) error {
	src, err := zip.OpenReader(base)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(out)
	if err != nil {
		return err
	}
	z := zip.NewWriter(dst)
	for _, f := range src.File {
		if err := z.Copy(f); err != nil {
			dst.Close()
			return err
		}
	}
	entries, err := os.ReadDir(dex)
	if err != nil {
		dst.Close()
		return err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".dex") {
			data, err := os.ReadFile(filepath.Join(dex, e.Name()))
			if err != nil {
				dst.Close()
				return err
			}
			libs[e.Name()] = data
		}
	}
	names := make([]string, 0, len(libs))
	for name := range libs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		f, err := z.CreateHeader(h)
		if err != nil {
			dst.Close()
			return err
		}
		if _, err := f.Write(libs[name]); err != nil {
			dst.Close()
			return err
		}
	}
	if err := z.Close(); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

func verifyELF(name string, data []byte) {
	f, err := elf.NewFile(bytes.NewReader(data))
	check(err)
	defer f.Close()
	for _, p := range f.Progs {
		if p.Type == elf.PT_LOAD && (p.Align < 16384 || p.Off%16384 != p.Vaddr%16384) {
			check(fmt.Errorf("%s has a load segment without 16 KiB alignment", name))
		}
		if p.Type == elf.PT_GNU_RELRO && (p.Vaddr+p.Memsz)%16384 != 0 {
			check(fmt.Errorf("%s has a RELRO boundary without 16 KiB alignment", name))
		}
	}
}

func existing(paths ...string) string {
	for _, p := range paths {
		if p != "" {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				return p
			}
		}
	}
	check(fmt.Errorf("required toolchain directory not found among %v", paths))
	return ""
}
func latest(dir string) string {
	entries, err := os.ReadDir(dir)
	check(err)
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		check(fmt.Errorf("no installed toolchain in %s", dir))
	}
	return filepath.Join(dir, names[len(names)-1])
}
func readZip(f *zip.File) []byte {
	r, err := f.Open()
	check(err)
	defer r.Close()
	b, err := io.ReadAll(r)
	check(err)
	return b
}
func run(name string, args ...string) {
	c := exec.Command(name, args...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	check(c.Run())
}
func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
