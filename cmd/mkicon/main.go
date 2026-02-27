// mkicon generates a 256×256 app icon PNG from the shared icon package.
// Usage: go run ./cmd/mkicon <output.png>
package main

import (
	"image/png"
	"os"

	"github.com/Mavwarf/notify/internal/icon"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	img := icon.Draw(256)
	f, err := os.Create(os.Args[1])
	if err != nil {
		os.Exit(1)
	}
	defer f.Close()
	png.Encode(f, img)
}
