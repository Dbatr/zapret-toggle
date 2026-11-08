package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"zapret-toggle/assets"
	"zapret-toggle/internal/config"
)

// Свои цвета поверх стандартных имён Fyne.
const (
	colChrome fyne.ThemeColorName = "zt.chrome" // верхняя и нижняя панели
	colPane   fyne.ThemeColorName = "zt.pane"   // фон карточки
	colRaised fyne.ThemeColorName = "zt.raised" // наведение, выключенная кнопка питания
	colBorder fyne.ThemeColorName = "zt.border" // рамки и разделители
	colMuted  fyne.ThemeColorName = "zt.muted"  // второстепенный текст
	colDim    fyne.ThemeColorName = "zt.dim"    // иконки и подсказки
	colOn     fyne.ThemeColorName = "zt.on"     // включено
	colBusy   fyne.ThemeColorName = "zt.busy"   // запуск/остановка
	colOnIcon fyne.ThemeColorName = "zt.onIcon" // символ на цветной кнопке
)

func hex(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}
}

// graphite — тёмная тема.
var graphite = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:          hex(0x131518ff),
	colChrome:                          hex(0x101114ff),
	colPane:                            hex(0x17191dff),
	colRaised:                          hex(0x26282cff),
	colBorder:                          hex(0x26272aff),
	colMuted:                           hex(0x8f9298ff),
	colDim:                             hex(0x5f6268ff),
	colOn:                              hex(0x22c55eff),
	colBusy:                            hex(0xeab308ff),
	colOnIcon:                          hex(0xffffffff),
	theme.ColorNameForeground:          hex(0xe4e4e5ff),
	theme.ColorNamePrimary:             hex(0x96b2d7ff),
	theme.ColorNameForegroundOnPrimary: hex(0x101114ff),
	theme.ColorNameFocus:               hex(0x96b2d74d),
	theme.ColorNameSelection:           hex(0x96b2d733),
	theme.ColorNameHover:               hex(0xffffff0d),
	theme.ColorNamePressed:             hex(0xffffff17),
	theme.ColorNameButton:              hex(0x26282cff),
	theme.ColorNameDisabledButton:      hex(0x1d1f23ff),
	theme.ColorNameDisabled:            hex(0x5f6268ff),
	theme.ColorNamePlaceHolder:         hex(0x6c6f75ff),
	theme.ColorNameInputBackground:     hex(0x1d1f23ff),
	theme.ColorNameInputBorder:         hex(0x2c2e32ff),
	theme.ColorNameMenuBackground:      hex(0x1c1e22ff),
	theme.ColorNameOverlayBackground:   hex(0x1c1e22ff),
	theme.ColorNameHeaderBackground:    hex(0x101114ff),
	theme.ColorNameSeparator:           hex(0x26272aff),
	theme.ColorNameShadow:              hex(0x00000080),
	theme.ColorNameScrollBar:           hex(0xffffff26),
	theme.ColorNameSuccess:             hex(0x4ade80ff),
	theme.ColorNameWarning:             hex(0xfacc15ff),
	theme.ColorNameError:               hex(0xf87171ff),
	theme.ColorNameHyperlink:           hex(0x96b2d7ff),
}

// paper — светлая тема в том же духе.
var paper = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:          hex(0xf3f3f1ff),
	colChrome:                          hex(0xebebe8ff),
	colPane:                            hex(0xffffffff),
	colRaised:                          hex(0xe8e8e5ff),
	colBorder:                          hex(0xdcdcd7ff),
	colMuted:                           hex(0x686b71ff),
	colDim:                             hex(0x9c9fa5ff),
	colOn:                              hex(0x22c55eff),
	colBusy:                            hex(0xeab308ff),
	colOnIcon:                          hex(0xffffffff),
	theme.ColorNameForeground:          hex(0x1c1d20ff),
	theme.ColorNamePrimary:             hex(0x3d6fa8ff),
	theme.ColorNameForegroundOnPrimary: hex(0xffffffff),
	theme.ColorNameFocus:               hex(0x3d6fa840),
	theme.ColorNameSelection:           hex(0x3d6fa826),
	theme.ColorNameHover:               hex(0x0000000a),
	theme.ColorNamePressed:             hex(0x00000014),
	theme.ColorNameButton:              hex(0xe8e8e5ff),
	theme.ColorNameDisabledButton:      hex(0xf0f0edff),
	theme.ColorNameDisabled:            hex(0xa3a6abff),
	theme.ColorNamePlaceHolder:         hex(0x8b8e93ff),
	theme.ColorNameInputBackground:     hex(0xffffffff),
	theme.ColorNameInputBorder:         hex(0xd4d4cfff),
	theme.ColorNameMenuBackground:      hex(0xffffffff),
	theme.ColorNameOverlayBackground:   hex(0xffffffff),
	theme.ColorNameHeaderBackground:    hex(0xebebe8ff),
	theme.ColorNameSeparator:           hex(0xe2e2deff),
	theme.ColorNameShadow:              hex(0x00000030),
	theme.ColorNameScrollBar:           hex(0x0000002e),
	theme.ColorNameSuccess:             hex(0x15803dff),
	theme.ColorNameWarning:             hex(0xa16207ff),
	theme.ColorNameError:               hex(0xdc2626ff),
	theme.ColorNameHyperlink:           hex(0x3d6fa8ff),
}

// appTheme — тема окна. mode: config.ThemeDark, config.ThemeLight или "" (как в системе).
type appTheme struct {
	mode string
}

func (t *appTheme) dark(v fyne.ThemeVariant) bool {
	switch t.mode {
	case config.ThemeDark:
		return true
	case config.ThemeLight:
		return false
	}
	return v == theme.VariantDark
}

func (t *appTheme) variant(v fyne.ThemeVariant) fyne.ThemeVariant {
	if t.dark(v) {
		return theme.VariantDark
	}
	return theme.VariantLight
}

func (t *appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	pal := paper
	if t.dark(v) {
		pal = graphite
	}
	if c, ok := pal[n]; ok {
		return c
	}
	return theme.DefaultTheme().Color(n, t.variant(v))
}

func (t *appTheme) Font(s fyne.TextStyle) fyne.Resource {
	switch {
	case s.Monospace:
		return assets.FontMono
	case s.Bold:
		return assets.FontSemiBold
	case s.Symbol:
		return theme.DefaultTheme().Font(s)
	}
	return assets.FontRegular
}

func (t *appTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	if n == theme.IconNameArrowDropDown {
		return chevronDown
	}
	return theme.DefaultTheme().Icon(n)
}

func (t *appTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameText:
		return 13
	case theme.SizeNameCaptionText:
		return 11.5
	case theme.SizeNameHeadingText:
		return 18
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 16
	case theme.SizeNameInputBorder, theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius, theme.SizeNameButtonRadius:
		return 6
	case theme.SizeNameMenuRadius, theme.SizeNamePopupRadius:
		return 8
	case theme.SizeNameDialogRadius, theme.SizeNameCardRadius:
		return 10
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 3
	}
	return theme.DefaultTheme().Size(n)
}

// isDark — тёмная ли тема сейчас (с учётом системной, если своя не выбрана).
func isDark() bool {
	s := fyne.CurrentApp().Settings()
	if t, ok := s.Theme().(*appTheme); ok {
		return t.dark(s.ThemeVariant())
	}
	return s.ThemeVariant() == theme.VariantDark
}
