// Package assets встраивает в бинарник иконку приложения и шрифты.
package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

// Icon — иконка окна (та же, что ставится в ~/.local/share/icons).
var Icon = fyne.NewStaticResource("zapret-toggle.png", iconPNG)

// Шрифты Geist, SIL OFL 1.1 — см. fonts/OFL.txt.
var (
	//go:embed fonts/Geist-Regular.ttf
	geistRegular []byte
	//go:embed fonts/Geist-SemiBold.ttf
	geistSemiBold []byte
	//go:embed fonts/GeistMono-Regular.ttf
	geistMono []byte

	FontRegular  = fyne.NewStaticResource("Geist-Regular.ttf", geistRegular)
	FontSemiBold = fyne.NewStaticResource("Geist-SemiBold.ttf", geistSemiBold)
	FontMono     = fyne.NewStaticResource("GeistMono-Regular.ttf", geistMono)
)
