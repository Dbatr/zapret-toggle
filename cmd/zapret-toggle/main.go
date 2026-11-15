// zapret-toggle — включает/выключает zapret (обход DPI) для YouTube и Discord одной кнопкой.
package main

import (
	"fyne.io/fyne/v2/app"

	"zapret-toggle/assets"
	"zapret-toggle/internal/ui"
)

func main() {
	// ID совпадает с именем .desktop-файла: по нему оболочка (GNOME, KDE Plasma) находит
	// ярлык окна под Wayland (app_id).
	a := app.NewWithID("zapret-toggle")
	a.SetIcon(assets.Icon)
	ui.New(a).Run()
}
