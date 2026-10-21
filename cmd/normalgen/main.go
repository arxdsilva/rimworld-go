// Command normalgen writes a normal map <name>_n.png next to every world
// sprite under the given directories (ART §6.1), with pkg/spritenormals,
// a port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez); see
// THIRD_PARTY_NOTICES.md. Tint masks, normal maps and UI art are skipped,
// and a sprite whose normal map is newer is left alone unless -force.
//
//	normalgen [-dirs assets/generated/world] [-force]
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rimworld-go/pkg/spritenormals"
)

func main() {
	dirs := flag.String("dirs", "assets/generated/world", "comma-separated sprite directories")
	force := flag.Bool("force", false, "regenerate every normal map")
	flag.Parse()
	start := time.Now()
	n, skipped, err := run(strings.Split(*dirs, ","), *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "normalgen:", err)
		os.Exit(1)
	}
	fmt.Printf("normalgen: %d normal maps written, %d up to date, in %s\n", n, skipped, time.Since(start).Round(time.Millisecond))
}

// settings is spritefusion's default scaled for our @2x art: radius,
// softness and detail scale doubled (ART §6.1).
func settings() spritenormals.Settings {
	s := spritenormals.DefaultSettings()
	s.ShapeRadius, s.ShapeSoftness, s.DetailScale = 8, 2.3, 1.2
	return s
}

// wanted reports a sprite PNG that needs a normal map.
func wanted(path string) bool {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".png") || strings.HasSuffix(base, "_mask.png") || strings.HasSuffix(base, "_n.png") {
		return false
	}
	return !strings.Contains(filepath.ToSlash(path), "/ui/")
}

func run(dirs []string, force bool) (written, skipped int, err error) {
	s := settings()
	for _, dir := range dirs {
		err = filepath.WalkDir(strings.TrimSpace(dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !wanted(path) {
				return err
			}
			out := strings.TrimSuffix(path, ".png") + "_n.png"
			if !force && newer(out, path) {
				skipped++
				return nil
			}
			if err := one(path, out, s); err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			written++
			return nil
		})
		if err != nil {
			return written, skipped, err
		}
	}
	return written, skipped, nil
}

// newer reports a exists and is at least as new as b.
func newer(a, b string) bool {
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	return err == nil && !sa.ModTime().Before(sb.ModTime())
}

func one(in, out string, s spritenormals.Settings) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		return err
	}
	nrgba, ok := src.(*image.NRGBA)
	if !ok {
		nrgba = image.NewNRGBA(src.Bounds())
		draw.Draw(nrgba, nrgba.Bounds(), src, src.Bounds().Min, draw.Src)
	}
	n, err := spritenormals.Generate(nrgba, s)
	if err != nil {
		return err
	}
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(w, n); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
