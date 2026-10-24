// Package assets встраивает иконку приложения в бинарник.
package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

// Icon — иконка окна (та же, что ставится в ~/.local/share/icons).
var Icon = fyne.NewStaticResource("zapret-toggle.png", iconPNG)
