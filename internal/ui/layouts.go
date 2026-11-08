package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// rowLayout — объекты в ряд по своей минимальной ширине, с заданным зазором, по центру по вертикали.
type rowLayout struct{ gap float32 }

func row(gap float32, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(rowLayout{gap}, objs...)
}

func (l rowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		s.Width += m.Width
		s.Height = fyne.Max(s.Height, m.Height)
		n++
	}
	if n > 1 {
		s.Width += l.gap * float32(n-1)
	}
	return s
}

func (l rowLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		o.Resize(m)
		o.Move(fyne.NewPos(x, (size.Height-m.Height)/2))
		x += m.Width + l.gap
	}
}

// insetLayout — отступы вокруг содержимого; height > 0 фиксирует высоту (полосы 46, 30, 28 px).
type insetLayout struct {
	left, top, right, bottom float32
	height                   float32
}

func inset(l insetLayout, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(l, objs...)
}

func (l insetLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	s = s.AddWidthHeight(l.left+l.right, l.top+l.bottom)
	if l.height > 0 {
		s.Height = l.height
	}
	return s
}

func (l insetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(fyne.NewSize(size.Width-l.left-l.right, size.Height-l.top-l.bottom))
		o.Move(fyne.NewPos(l.left, l.top))
	}
}

// maxWidthLayout — содержимое не шире max и по центру: на весь экран окно растягивается,
// а список стратегий и подписи остаются удобной ширины.
type maxWidthLayout struct{ max float32 }

func (l maxWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	return s
}

func (l maxWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := fyne.Min(size.Width, l.max)
	for _, o := range objs {
		o.Resize(fyne.NewSize(w, size.Height))
		o.Move(fyne.NewPos((size.Width-w)/2, 0))
	}
}

// powerLayout — кнопка питания и две строки под ней, группа по центру. Кнопка растёт
// вместе с окном (от powerMin до powerMax), чтобы на весь экран не выглядеть мелкой.
type powerLayout struct{}

const (
	powerMin     = 136
	powerMax     = 260
	powerGap     = 18 // от кнопки до статуса
	powerTextGap = 2  // между статусом и подсказкой
	textMaxWidth = 340
)

// objs: кнопка, статус (переносимый Label), подсказка.
func (powerLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	status, hint := objs[1].MinSize(), objs[2].MinSize()
	w := fyne.Max(powerMin, fyne.Max(status.Width, hint.Width))
	return fyne.NewSize(w, powerMin+powerGap+status.Height+powerTextGap+hint.Height)
}

func (powerLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	button, status, hint := objs[0], objs[1], objs[2]

	textW := fyne.Min(size.Width, textMaxWidth)
	status.Resize(fyne.NewSize(textW, status.MinSize().Height))
	statusH := status.MinSize().Height // у переносимого текста высота зависит от ширины
	status.Resize(fyne.NewSize(textW, statusH))
	hintH := hint.MinSize().Height
	hint.Resize(fyne.NewSize(textW, hintH))

	text := powerGap + statusH + powerTextGap + hintH
	side := fyne.Min(fyne.Min(size.Height-text, size.Width*0.7), powerMax)
	side = fyne.Max(side, powerMin)

	y := (size.Height - side - text) / 2
	button.Resize(fyne.NewSquareSize(side))
	button.Move(fyne.NewPos((size.Width-side)/2, y))
	y += side + powerGap
	status.Move(fyne.NewPos((size.Width-textW)/2, y))
	y += statusH + powerTextGap
	hint.Move(fyne.NewPos((size.Width-textW)/2, y))
}

// columnLayout — объекты сверху вниз с зазором gap (container.NewVBox/NewBorder добавляют
// свои отступы, а здесь они не нужны). Объект с индексом fill забирает свободную высоту.
type columnLayout struct {
	gap  float32
	fill int // -1 — никто не растягивается
}

func column(gap float32, fill int, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(columnLayout{gap, fill}, objs...)
}

func (l columnLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for i, o := range objs {
		m := o.MinSize()
		s.Width = fyne.Max(s.Width, m.Width)
		s.Height += m.Height
		if i > 0 {
			s.Height += l.gap
		}
	}
	return s
}

func (l columnLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	extra := size.Height - l.MinSize(objs).Height
	y := float32(0)
	for i, o := range objs {
		h := o.MinSize().Height
		if i == l.fill && extra > 0 {
			h += extra
		}
		o.Resize(fyne.NewSize(size.Width, h))
		o.Move(fyne.NewPos(0, y))
		y += h + l.gap
	}
}

// barLayout — содержимое полосы: objs[0] слева, objs[1] справа. Правая часть второстепенная
// (подсказки): в минимальную ширину не входит и прячется, если не помещается рядом с левой.
type barLayout struct{ gap float32 }

func (l barLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	s := objs[0].MinSize()
	s.Height = fyne.Max(s.Height, objs[1].MinSize().Height)
	return s
}

func (l barLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	left, right := objs[0], objs[1]
	lm, rm := left.MinSize(), right.MinSize()
	left.Resize(fyne.NewSize(lm.Width, size.Height))
	left.Move(fyne.NewPos(0, 0))
	if lm.Width+l.gap+rm.Width > size.Width {
		right.Hide()
		return
	}
	right.Show()
	right.Resize(fyne.NewSize(rm.Width, size.Height))
	right.Move(fyne.NewPos(size.Width-rm.Width, 0))
}
