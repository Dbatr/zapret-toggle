package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

var (
	colorOff  = color.NRGBA{R: 0x8a, G: 0x8f, B: 0x98, A: 0xff}
	colorOn   = color.NRGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff}
	colorBusy = color.NRGBA{R: 0xea, G: 0xb3, B: 0x08, A: 0xff}
	colorIcon = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

const buttonSize = 176

// PowerButton — большая круглая кнопка в стиле VPN-клиентов: круг с символом питания и «ореолом».
type PowerButton struct {
	widget.DisableableWidget

	OnTapped func()

	color   color.NRGBA
	hovered bool
	pulse   *fyne.Animation
	halo    *canvas.Circle
}

func NewPowerButton(tapped func()) *PowerButton {
	b := &PowerButton{OnTapped: tapped, color: colorOff}
	b.ExtendBaseWidget(b)
	return b
}

// SetColor меняет цвет кнопки. pulse включает пульсацию ореола (идёт запуск/остановка).
func (b *PowerButton) SetColor(c color.NRGBA, pulse bool) {
	b.color = c
	if pulse && b.pulse == nil {
		b.pulse = fyne.NewAnimation(900*time.Millisecond, func(f float32) {
			if b.halo != nil {
				b.halo.FillColor = withAlpha(b.color, uint8(0x18+f*0x50))
				b.halo.Refresh()
			}
		})
		b.pulse.AutoReverse = true
		b.pulse.RepeatCount = fyne.AnimationRepeatForever
		b.pulse.Start()
	} else if !pulse && b.pulse != nil {
		b.pulse.Stop()
		b.pulse = nil
	}
	b.Refresh()
}

func (b *PowerButton) Tapped(e *fyne.PointEvent) {
	if b.Disabled() || b.OnTapped == nil {
		return
	}
	// Реагируем только на нажатие внутри круга, а не по углам квадрата.
	s := b.Size()
	r := fyne.Min(s.Width, s.Height) / 2
	dx, dy := e.Position.X-s.Width/2, e.Position.Y-s.Height/2
	if dx*dx+dy*dy > r*r {
		return
	}
	b.OnTapped()
}

func (b *PowerButton) Cursor() desktop.Cursor {
	if b.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *PowerButton) MouseIn(*desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *PowerButton) MouseMoved(*desktop.MouseEvent) {}
func (b *PowerButton) MouseOut()                      { b.hovered = false; b.Refresh() }

func (b *PowerButton) MinSize() fyne.Size {
	return fyne.NewSquareSize(buttonSize)
}

func (b *PowerButton) CreateRenderer() fyne.WidgetRenderer {
	r := &powerRenderer{
		b:    b,
		halo: canvas.NewCircle(color.Transparent),
		disc: canvas.NewCircle(color.Transparent),
		ring: canvas.NewArc(40, 320, 0.74, colorIcon),
		bar:  canvas.NewRectangle(colorIcon),
	}
	b.halo = r.halo
	r.Refresh()
	return r
}

type powerRenderer struct {
	b          *PowerButton
	halo, disc *canvas.Circle
	ring       *canvas.Arc
	bar        *canvas.Rectangle
}

func (r *powerRenderer) Layout(size fyne.Size) {
	d := fyne.Min(size.Width, size.Height)
	center := fyne.NewPos(size.Width/2, size.Height/2)
	place := func(o fyne.CanvasObject, diameter float32) {
		o.Resize(fyne.NewSquareSize(diameter))
		o.Move(center.SubtractXY(diameter/2, diameter/2))
	}
	place(r.halo, d)
	place(r.disc, d*0.82)

	ringD := d * 0.38
	stroke := ringD * (1 - r.ring.CutoutRatio) / 2
	place(r.ring, ringD)
	r.ring.CornerRadius = stroke / 2

	barH := ringD * 0.5
	r.bar.CornerRadius = stroke / 2
	r.bar.Resize(fyne.NewSize(stroke, barH))
	r.bar.Move(fyne.NewPos(center.X-stroke/2, center.Y-ringD/2-stroke*0.6))
}

func (r *powerRenderer) MinSize() fyne.Size { return r.b.MinSize() }

func (r *powerRenderer) Refresh() {
	c := r.b.color
	if r.b.hovered && !r.b.Disabled() {
		c = lighten(c, 0.12)
	}
	r.disc.FillColor = c
	if r.b.pulse == nil {
		r.halo.FillColor = withAlpha(r.b.color, 0x30)
	}
	icon := colorIcon
	if r.b.Disabled() && r.b.pulse == nil {
		icon = withAlpha(colorIcon, 0x80)
	}
	r.ring.FillColor = icon
	r.bar.FillColor = icon
	r.Layout(r.b.Size())
	canvas.Refresh(r.b)
}

func (r *powerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.halo, r.disc, r.ring, r.bar}
}

func (r *powerRenderer) Destroy() {
	if r.b.pulse != nil {
		r.b.pulse.Stop()
	}
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func lighten(c color.NRGBA, k float32) color.NRGBA {
	mix := func(v uint8) uint8 { return v + uint8(float32(0xff-v)*k) }
	return color.NRGBA{R: mix(c.R), G: mix(c.G), B: mix(c.B), A: c.A}
}
