package ui

import (
	"embed"
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Иконки Lucide (ISC, см. icons/LICENSE) с обводкой 1.5.
//
//go:embed icons/*.svg
var iconFS embed.FS

// chevronDown — стрелка выпадающего списка. Она залита (fill), а не обведена,
// чтобы Fyne сам красил её в цвет текста и в «выключенный» цвет.
var chevronDown = func() fyne.Resource {
	data, err := iconFS.ReadFile("icons/chevron-down.svg")
	if err != nil {
		panic(err)
	}
	return theme.NewThemedResource(fyne.NewStaticResource("chevron-down.svg", data))
}()

var (
	iconMu    sync.Mutex
	iconCache = map[string]fyne.Resource{}
)

// iconRes возвращает иконку name, перекрашенную в c. Fyne сам перекрашивает только fill,
// а у Lucide рисует stroke="currentColor", поэтому цвет подставляем в SVG здесь.
func iconRes(name string, c color.Color) fyne.Resource {
	r, g, b, a := c.RGBA()
	key := fmt.Sprintf("%s-%02x%02x%02x%02x", name, r>>8, g>>8, b>>8, a>>8)
	iconMu.Lock()
	defer iconMu.Unlock()
	if res, ok := iconCache[key]; ok {
		return res
	}
	data, err := iconFS.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic(err) // имя иконки — константа в коде
	}
	hexColor := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	svg := strings.ReplaceAll(string(data), `stroke="currentColor"`,
		fmt.Sprintf(`stroke="%s" stroke-opacity="%.3f"`, hexColor, float32(a>>8)/255))
	res := fyne.NewStaticResource(key+".svg", []byte(svg))
	iconCache[key] = res
	return res
}

func themeColor(n fyne.ThemeColorName) color.Color { return theme.Color(n) }

// box — прямоугольник с цветами из темы (фон панелей, рамка карточки, точка статуса).
type box struct {
	widget.BaseWidget
	fill, stroke fyne.ThemeColorName // "" — прозрачно / без рамки
	radius       float32
	strokeWidth  float32
	min          fyne.Size
}

func newBox(fill, stroke fyne.ThemeColorName, radius float32) *box {
	b := &box{fill: fill, stroke: stroke, radius: radius, strokeWidth: 1}
	b.ExtendBaseWidget(b)
	return b
}

func (b *box) setColors(fill, stroke fyne.ThemeColorName) {
	if b.fill == fill && b.stroke == stroke {
		return
	}
	b.fill, b.stroke = fill, stroke
	b.Refresh()
}

func (b *box) MinSize() fyne.Size { return b.min }

func (b *box) CreateRenderer() fyne.WidgetRenderer {
	r := &boxRenderer{b: b, rect: canvas.NewRectangle(color.Transparent)}
	r.Refresh()
	return r
}

type boxRenderer struct {
	b    *box
	rect *canvas.Rectangle
}

func (r *boxRenderer) Layout(s fyne.Size)           { r.rect.Resize(s) }
func (r *boxRenderer) MinSize() fyne.Size           { return r.b.min }
func (r *boxRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }
func (r *boxRenderer) Destroy()                     {}

func (r *boxRenderer) Refresh() {
	r.rect.FillColor = color.Transparent
	if r.b.fill != "" {
		r.rect.FillColor = themeColor(r.b.fill)
	}
	r.rect.StrokeWidth = 0
	if r.b.stroke != "" {
		r.rect.StrokeColor = themeColor(r.b.stroke)
		r.rect.StrokeWidth = r.b.strokeWidth
	}
	r.rect.CornerRadius = r.b.radius
	r.rect.Refresh()
}

// text — однострочный текст с цветом из темы и своим размером.
type text struct {
	widget.BaseWidget
	text  string
	color fyne.ThemeColorName
	size  float32
	bold  bool
	align fyne.TextAlign
}

func newText(s string, c fyne.ThemeColorName, size float32, bold bool) *text {
	t := &text{text: s, color: c, size: size, bold: bold}
	t.ExtendBaseWidget(t)
	return t
}

func (t *text) setText(s string) {
	if t.text != s {
		t.text = s
		t.Refresh()
	}
}

func (t *text) CreateRenderer() fyne.WidgetRenderer {
	r := &textRenderer{t: t, c: canvas.NewText("", color.Transparent)}
	r.Refresh()
	return r
}

type textRenderer struct {
	t *text
	c *canvas.Text
}

func (r *textRenderer) Layout(s fyne.Size)           { r.c.Resize(s) }
func (r *textRenderer) MinSize() fyne.Size           { return r.c.MinSize() }
func (r *textRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.c} }
func (r *textRenderer) Destroy()                     {}

func (r *textRenderer) Refresh() {
	r.c.Text = r.t.text
	r.c.Color = themeColor(r.t.color)
	r.c.TextSize = r.t.size
	r.c.TextStyle = fyne.TextStyle{Bold: r.t.bold}
	r.c.Alignment = r.t.align
	r.c.Refresh()
}

// icon — иконка Lucide с цветом из темы.
type icon struct {
	widget.BaseWidget
	name  string
	color fyne.ThemeColorName
	size  float32
}

func newIcon(name string, c fyne.ThemeColorName, size float32) *icon {
	i := &icon{name: name, color: c, size: size}
	i.ExtendBaseWidget(i)
	return i
}

func (i *icon) MinSize() fyne.Size { return fyne.NewSquareSize(i.size) }

func (i *icon) CreateRenderer() fyne.WidgetRenderer {
	r := &iconRenderer{i: i, img: canvas.NewImageFromResource(nil)}
	r.img.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

type iconRenderer struct {
	i   *icon
	img *canvas.Image
}

func (r *iconRenderer) Layout(s fyne.Size)           { r.img.Resize(s) }
func (r *iconRenderer) MinSize() fyne.Size           { return r.i.MinSize() }
func (r *iconRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.img} }
func (r *iconRenderer) Destroy()                     {}

func (r *iconRenderer) Refresh() {
	r.img.Resource = iconRes(r.i.name, themeColor(r.i.color))
	r.img.Refresh()
}

// iconButton — квадратная кнопка-иконка со скруглённой подсветкой при наведении.
// Иконка берётся из iconName при каждой перерисовке.
type iconButton struct {
	widget.BaseWidget
	iconName func() string
	onTapped func()
	hovered  bool
}

const iconButtonSize = 28

func newIconButton(iconName func() string, tapped func()) *iconButton {
	b := &iconButton{iconName: iconName, onTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

func (b *iconButton) Tapped(*fyne.PointEvent) {
	if b.onTapped != nil {
		b.onTapped()
	}
}

func (b *iconButton) Cursor() desktop.Cursor         { return desktop.PointerCursor }
func (b *iconButton) MouseIn(*desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *iconButton) MouseMoved(*desktop.MouseEvent) {}
func (b *iconButton) MouseOut()                      { b.hovered = false; b.Refresh() }
func (b *iconButton) MinSize() fyne.Size             { return fyne.NewSquareSize(iconButtonSize) }
func (b *iconButton) CreateRenderer() fyne.WidgetRenderer {
	r := &iconButtonRenderer{b: b, bg: canvas.NewRectangle(color.Transparent), img: canvas.NewImageFromResource(nil)}
	r.bg.CornerRadius = 6
	r.img.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

type iconButtonRenderer struct {
	b   *iconButton
	bg  *canvas.Rectangle
	img *canvas.Image
}

func (r *iconButtonRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	const iconSize = 16
	r.img.Resize(fyne.NewSquareSize(iconSize))
	r.img.Move(fyne.NewPos((s.Width-iconSize)/2, (s.Height-iconSize)/2))
}

func (r *iconButtonRenderer) MinSize() fyne.Size           { return r.b.MinSize() }
func (r *iconButtonRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.bg, r.img} }
func (r *iconButtonRenderer) Destroy()                     {}

func (r *iconButtonRenderer) Refresh() {
	r.bg.FillColor = color.Transparent
	c := colMuted
	if r.b.hovered {
		r.bg.FillColor = themeColor(colRaised)
		c = theme.ColorNameForeground
	}
	r.img.Resource = iconRes(r.b.iconName(), themeColor(c))
	r.bg.Refresh()
	r.img.Refresh()
}
