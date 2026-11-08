package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// powerState — что показывает кнопка.
type powerState int

const (
	stateOff powerState = iota
	stateOn
	stateBusy
)

// PowerButton — большая круглая кнопка в стиле VPN-клиентов: круг с символом питания и «ореолом».
// Цвета берутся из темы при каждой перерисовке, поэтому кнопка сама следует за светлой/тёмной темой.
type PowerButton struct {
	widget.DisableableWidget

	OnTapped func()

	state   powerState
	hovered bool
	pulse   *fyne.Animation
	halo    *canvas.Circle
}

func NewPowerButton(tapped func()) *PowerButton {
	b := &PowerButton{OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

// SetState меняет вид кнопки. Во время запуска/остановки ореол пульсирует.
func (b *PowerButton) SetState(s powerState) {
	b.state = s
	if s == stateBusy && b.pulse == nil {
		b.pulse = fyne.NewAnimation(900*time.Millisecond, func(f float32) {
			if b.halo != nil {
				b.halo.FillColor = withAlpha(nrgba(themeColor(colBusy)), uint8(0x18+f*0x50))
				b.halo.Refresh()
			}
		})
		b.pulse.AutoReverse = true
		b.pulse.RepeatCount = fyne.AnimationRepeatForever
		b.pulse.Start()
	} else if s != stateBusy && b.pulse != nil {
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
	return fyne.NewSquareSize(powerMin)
}

func (b *PowerButton) CreateRenderer() fyne.WidgetRenderer {
	r := &powerRenderer{
		b:    b,
		halo: canvas.NewCircle(color.Transparent),
		disc: canvas.NewCircle(color.Transparent),
		ring: canvas.NewArc(40, 320, 0.76, color.Transparent),
		bar:  canvas.NewRectangle(color.Transparent),
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
	place(r.disc, d*0.8)

	ringD := d * 0.34
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
	var disc, icon color.NRGBA
	switch r.b.state {
	case stateOn:
		disc, icon = nrgba(themeColor(colOn)), nrgba(themeColor(colOnIcon))
	case stateBusy:
		disc, icon = nrgba(themeColor(colBusy)), nrgba(themeColor(colOnIcon))
	default:
		// Выключено — спокойный «приподнятый» круг с рамкой.
		disc, icon = nrgba(themeColor(colRaised)), nrgba(themeColor(colMuted))
	}
	active := !r.b.Disabled() || r.b.pulse != nil
	if r.b.hovered && active {
		if r.b.state == stateOff {
			icon = nrgba(themeColor(colOn))
		} else {
			disc = lighten(disc, 0.1)
		}
	}
	if !active {
		icon = withAlpha(icon, icon.A/2)
	}

	r.disc.FillColor = disc
	r.disc.StrokeWidth = 0
	if r.b.state == stateOff {
		r.disc.StrokeColor = themeColor(colBorder)
		r.disc.StrokeWidth = 1
	}
	if r.b.pulse == nil {
		if r.b.state == stateOff {
			r.halo.FillColor = withAlpha(nrgba(themeColor(colRaised)), 0x60)
		} else {
			r.halo.FillColor = withAlpha(disc, 0x30)
		}
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

func nrgba(c color.Color) color.NRGBA {
	return color.NRGBAModel.Convert(c).(color.NRGBA)
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func lighten(c color.NRGBA, k float32) color.NRGBA {
	mix := func(v uint8) uint8 { return v + uint8(float32(0xff-v)*k) }
	return color.NRGBA{R: mix(c.R), G: mix(c.G), B: mix(c.B), A: c.A}
}
