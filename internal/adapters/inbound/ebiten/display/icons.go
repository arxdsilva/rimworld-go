package display

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"rimworld-go/internal/platform/assetroot"
)

// iconSizes are the window icon sizes shipped under iconDir, smallest first
// (`make app-icon` writes them).
var iconSizes = [...]int{16, 32, 48, 256}

const iconDir = "assets/generated/ui/app_icon"

// LoadIcons reads the window icons from the assets folder, found through
// assetroot like the art catalog does.
func LoadIcons() ([]image.Image, error) {
	dir, err := assetroot.Find(iconDir, true)
	if err != nil {
		return nil, err
	}
	return loadIconsFrom(dir)
}

func loadIconsFrom(dir string) ([]image.Image, error) {
	out := make([]image.Image, 0, len(iconSizes))
	for _, n := range iconSizes {
		f, err := os.Open(filepath.Join(dir, fmt.Sprintf("icon_%d.png", n)))
		if err != nil {
			return nil, err
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("icon_%d.png: %w", n, err)
		}
		out = append(out, img)
	}
	return out, nil
}
