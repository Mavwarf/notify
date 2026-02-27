//go:build windows

package toast

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/Mavwarf/notify/internal/paths"
)

const iconFileName = "icon.png"

// EnsureIcon writes a 64×64 PNG app icon to DataDir()/icon.png if it doesn't
// already exist and returns the absolute file path. The icon is a green circle
// with a white "N" letter, generated programmatically with no external deps.
func EnsureIcon() (string, error) {
	p := filepath.Join(paths.DataDir(), iconFileName)
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), paths.DirPerm); err != nil {
		return "", err
	}
	img := drawIcon(64)
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return "", err
	}
	return p, nil
}


// drawIcon creates a size×size image with a green circle and white "N".
func drawIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	green := color.RGBA{R: 34, G: 197, B: 94, A: 255} // #22c55e
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size) / 2

	// Draw filled circle.
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if dx*dx+dy*dy <= radius*radius {
				img.SetRGBA(x, y, green)
			}
		}
	}

	// Draw "N" letter using line segments.
	drawN(img, size, white)
	return img
}

// drawN draws a white "N" centered in the icon. The letter is composed of
// three strokes: left vertical, right vertical, and a diagonal connecting
// top-left to bottom-right.
func drawN(img *image.RGBA, size int, c color.RGBA) {
	// Letter bounds (roughly centered in the circle).
	thick := max(size/10, 2) // stroke width
	margin := size / 4
	top := margin
	bot := size - margin
	left := margin + thick/2
	right := size - margin - thick/2

	// Left vertical stroke.
	fillRect(img, left-thick/2, top, left+thick/2, bot, c)
	// Right vertical stroke.
	fillRect(img, right-thick/2, top, right+thick/2, bot, c)
	// Diagonal stroke from top-left to bottom-right.
	drawLine(img, left, top, right, bot, thick, c)
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

// drawLine draws a thick line from (x0,y0) to (x1,y1).
func drawLine(img *image.RGBA, x0, y0, x1, y1, thick int, c color.RGBA) {
	dx := float64(x1 - x0)
	dy := float64(y1 - y0)
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}
	halfT := float64(thick) / 2

	steps := int(length * 2)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px := float64(x0) + dx*t
		py := float64(y0) + dy*t
		// Fill a square around the point.
		for oy := -halfT; oy <= halfT; oy++ {
			for ox := -halfT; ox <= halfT; ox++ {
				ix := int(px + ox)
				iy := int(py + oy)
				if ix >= 0 && ix < img.Bounds().Dx() && iy >= 0 && iy < img.Bounds().Dy() {
					img.SetRGBA(ix, iy, c)
				}
			}
		}
	}
}
