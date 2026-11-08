package ui

import (
	"image/color"
	"io/fs"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

// Каждая иконка Lucide должна разбираться и рисоваться тем цветом, что подставил iconRes.
func TestIconsRenderInGivenColor(t *testing.T) {
	test.NewTempApp(t)
	names, err := fs.Glob(iconFS, "icons/*.svg")
	if err != nil || len(names) == 0 {
		t.Fatalf("иконки не найдены: %v", err)
	}
	red := color.NRGBA{R: 0xff, A: 0xff}
	for _, path := range names {
		name := strings.TrimSuffix(strings.TrimPrefix(path, "icons/"), ".svg")
		if name == "chevron-down" { // залитая, её красит сам Fyne
			continue
		}
		img := canvas.NewImageFromResource(iconRes(name, red))
		img.SetMinSize(fyne.NewSquareSize(48))
		w := test.NewWindow(img)
		w.SetPadded(false)
		w.Resize(fyne.NewSquareSize(48))
		shot := w.Canvas().Capture()
		w.Close()

		found := false
		b := shot.Bounds()
		for y := b.Min.Y; y < b.Max.Y && !found; y++ {
			for x := b.Min.X; x < b.Max.X && !found; x++ {
				r, g, bl, _ := shot.At(x, y).RGBA()
				found = r>>8 > 0xc0 && g>>8 < 0x40 && bl>>8 < 0x40
			}
		}
		if !found {
			t.Errorf("%s: нет пикселей заданного цвета", name)
		}
	}
}
