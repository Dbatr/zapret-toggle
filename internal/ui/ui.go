// Package ui — окно zapret-toggle: одна большая кнопка, выбор стратегии и «выключать при закрытии».
// Оформление: шапка и строка состояния, карточка со скруглёнными углами,
// иконки Lucide, шрифт Geist, светлая и тёмная темы.
//
// Всё состояние окна меняется только в UI-потоке (колбэки Fyne и fyne.Do);
// вызовы systemctl идут в горутинах.
package ui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"zapret-toggle/assets"
	"zapret-toggle/internal/config"
	"zapret-toggle/internal/service"
	"zapret-toggle/internal/strategies"
)

const (
	// Без D-Bus статус опрашивается так часто; с ним — по сигналам systemd, а опрос раз
	// в watchPollInterval лишь страхует (юнит не из списка, пропущенный сигнал).
	pollInterval      = 2 * time.Second
	watchPollInterval = 30 * time.Second
	// Сигналы одной операции приходят пачкой — ждём, пока она закончится.
	watchDebounce = 150 * time.Millisecond
	statusTimeout = 10 * time.Second
	opTimeout     = 45 * time.Second
	stopTimeout   = 20 * time.Second
)

type Window struct {
	app fyne.App
	win fyne.Window

	cfg     config.Config
	cfgPath string
	list    []strategies.Strategy
	// installErr != nil — системная часть не установлена, управлять нечем.
	installErr error

	button     *PowerButton
	status     *widget.Label
	hint       *text
	strategy   *widget.Select
	stopOnExit *widget.Check

	pane      *box  // рамка карточки: зелёная, пока zapret включён
	dot       *box  // точка состояния в нижней строке
	stateText *text // «Работает» / «Остановлено» в нижней строке
	unitText  *text // zapret-dy@<стратегия>
	fullBtn   *iconButton

	// Только для UI-потока.
	running        string // slug активной стратегии, "" — выключено
	lastErr        string // текст последней ошибки; висит в статусе до следующего действия
	statusErr      string // не удалось получить статус; перекрывает остальные сообщения
	busy           bool   // идёт start/stop
	seq            uint64 // растёт при каждой операции; устаревшие результаты опроса отбрасываются
	syncing        bool   // программная смена выбора стратегии, OnChanged игнорируется
	closing        bool
	closeRequested bool // закрытие запрошено во время операции — выполнить после неё
}

func New(a fyne.App) *Window {
	w := &Window{app: a}

	var err error
	if w.cfgPath, err = config.Path(); err == nil {
		w.cfg, err = config.Load(w.cfgPath)
	}
	if err != nil {
		log.Printf("конфиг: %v (используются настройки по умолчанию)", err)
		w.cfg = config.Default()
	}
	a.Settings().SetTheme(&appTheme{mode: w.cfg.Theme})
	w.win = a.NewWindow("Zapret Toggle")

	if w.installErr = service.Installed(); w.installErr == nil {
		w.list, w.installErr = strategies.Load(strategies.Dir)
	}

	w.build()
	return w
}

// Run показывает окно и запускает цикл приложения.
func (w *Window) Run() {
	w.win.SetCloseIntercept(w.requestClose)
	w.app.Lifecycle().SetOnStarted(func() {
		go w.handleSignals()
		if w.installErr == nil {
			go w.statusLoop()
		}
	})
	w.win.ShowAndRun()
}

func (w *Window) build() {
	w.button = NewPowerButton(w.toggle)
	w.status = widget.NewLabel("")
	w.status.Alignment = fyne.TextAlignCenter
	w.status.Wrapping = fyne.TextWrapWord
	w.status.TextStyle.Bold = true
	w.status.SizeName = theme.SizeNameSubHeadingText
	w.hint = newText("", colMuted, 12, false)
	w.hint.align = fyne.TextAlignCenter

	names := make([]string, len(w.list))
	for i, s := range w.list {
		names[i] = s.Name
	}
	// Не NewSelect: он запрашивает перевод плейсхолдера "(Select one)", которого нет для ru.
	w.strategy = &widget.Select{Options: names, OnChanged: w.strategyChanged, PlaceHolder: "—"}
	w.strategy.ExtendBaseWidget(w.strategy)
	w.syncing = true
	w.strategy.SetSelected(w.nameOf(w.selectedSlug()))
	w.syncing = false

	w.stopOnExit = widget.NewCheck("Выключать при закрытии окна", func(on bool) {
		w.win.Canvas().Unfocus() // иначе галочка забирает F11
		w.cfg.StopOnClose = on
		w.saveConfig()
	})
	w.stopOnExit.SetChecked(w.cfg.StopOnClose)

	// Шапка: иконка и название, справа кнопки-иконки.
	appIcon := canvas.NewImageFromResource(assets.Icon)
	appIcon.FillMode = canvas.ImageFillContain
	appIcon.SetMinSize(fyne.NewSquareSize(18))
	themeBtn := newIconButton(func() string {
		if isDark() {
			return "sun"
		}
		return "moon"
	}, w.toggleTheme)
	w.fullBtn = newIconButton(func() string {
		if w.win.FullScreen() {
			return "minimize"
		}
		return "maximize"
	}, w.toggleFullScreen)
	header := bar(46, 14, 9, colChrome,
		row(10, appIcon, newText("Zapret Toggle", theme.ColorNameForeground, 13.5, true)),
		row(2, w.fullBtn, themeBtn))

	// Карточка: заголовок 30 px, рамка, радиус 10.
	paneHeader := bar(30, 12, 12, "", row(8,
		newIcon("shield", colDim, 14),
		newText("YouTube · Discord", theme.ColorNameForeground, 12.5, true),
		newText("zapret1 · Flowseal", colMuted, 12.5, false),
	), nil)
	power := inset(insetLayout{left: 16, top: 20, right: 16, bottom: 20},
		container.New(powerLayout{}, w.button, w.status, w.hint))
	strategyRow := container.NewBorder(nil, nil,
		row(8, newIcon("layers", colDim, 15), newText("Стратегия", colMuted, 13, false)), nil, w.strategy)
	footer := container.New(maxWidthLayout{max: 440},
		inset(insetLayout{left: 14, top: 12, right: 14, bottom: 8}, column(4, -1, strategyRow, w.stopOnExit)))
	w.pane = newBox(colPane, colBorder, 10)
	pane := container.NewStack(w.pane, column(0, 2, paneHeader, hline(), power, hline(), footer))

	// Нижняя строка состояния.
	w.dot = newBox(colDim, "", 3)
	w.dot.min = fyne.NewSquareSize(6)
	w.stateText = newText("", colMuted, 11.8, false)
	w.unitText = newText("", colMuted, 11.8, false)
	statusBar := bar(28, 14, 14, colChrome,
		row(16, row(6, w.dot, w.stateText), row(6, newIcon("server", colMuted, 13), w.unitText)),
		newText("F11 — полный экран", colDim, 11.8, false))

	w.win.SetContent(column(0, 2,
		header, hline(),
		inset(insetLayout{left: 8, top: 8, right: 8, bottom: 8}, pane),
		hline(), statusBar,
	))
	w.win.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) {
		switch {
		case e.Name == fyne.KeyF11:
			w.toggleFullScreen()
		case e.Name == fyne.KeyEscape && w.win.FullScreen():
			w.toggleFullScreen()
		}
	})
	// Размер окна меняется свободно; меньше минимального содержимого Fyne сжать не даст.
	w.win.Resize(fyne.NewSize(420, 620))
	w.win.CenterOnScreen()

	if w.installErr != nil {
		w.button.Disable()
		w.strategy.Disable()
		w.paint(stateOff)
		w.stateText.setText("Не установлено")
		msg := "zapret-dy не установлен.\nЗапусти: sudo scripts/install.sh"
		if !errors.Is(w.installErr, service.ErrNotInstalled) {
			msg = "Установка zapret-dy повреждена:\n" + w.installErr.Error() + "\nПереустанови: sudo scripts/install.sh"
		}
		w.setStatus(msg, widget.DangerImportance)
		return
	}
	w.render()
}

// bar — полоса фиксированной высоты с фоном fill ("" — прозрачная): слева left, справа right.
func bar(height, padLeft, padRight float32, fill fyne.ThemeColorName, left, right fyne.CanvasObject) fyne.CanvasObject {
	if right == nil {
		right = row(0)
	}
	return container.NewStack(
		newBox(fill, "", 0),
		inset(insetLayout{left: padLeft, right: padRight, height: height}, container.New(barLayout{gap: 16}, left, right)),
	)
}

// hline — разделитель в 1 px цвета рамки.
func hline() fyne.CanvasObject {
	b := newBox(colBorder, "", 0)
	b.min = fyne.NewSize(0, 1)
	return b
}

func (w *Window) toggleTheme() {
	w.cfg.Theme = config.ThemeDark
	if isDark() {
		w.cfg.Theme = config.ThemeLight
	}
	w.saveConfig()
	w.app.Settings().SetTheme(&appTheme{mode: w.cfg.Theme})
}

func (w *Window) toggleFullScreen() {
	w.win.SetFullScreen(!w.win.FullScreen())
	w.fullBtn.Refresh()
}

// paint красит всё, что зависит от состояния: кнопку, рамку карточки, нижнюю строку.
func (w *Window) paint(st powerState) {
	w.button.SetState(st)
	switch st {
	case stateOn:
		w.pane.setColors(colPane, colOn)
		w.dot.setColors(colOn, "")
		w.stateText.setText("Работает")
		w.hint.setText("Нажми, чтобы выключить")
	case stateBusy:
		w.pane.setColors(colPane, colBusy)
		w.dot.setColors(colBusy, "")
		w.stateText.setText("Подожди…")
		w.hint.setText("")
	default:
		w.pane.setColors(colPane, colBorder)
		w.dot.setColors(colDim, "")
		w.stateText.setText("Остановлено")
		w.hint.setText("Нажми, чтобы включить")
		if w.installErr != nil {
			w.hint.setText("")
		}
	}
	slug := w.running
	if slug == "" {
		slug = w.slugOf(w.strategy.Selected)
	}
	w.unitText.setText("zapret-dy@" + slug)
}

// render приводит кнопку и статус в соответствие с running/busy.
func (w *Window) render() {
	switch {
	case w.statusErr != "":
		st := stateOff
		if w.running != "" {
			st = stateOn
		}
		w.paint(st)
		w.setStatus("Не удалось получить статус:\n"+w.statusErr, widget.DangerImportance)
	case w.running != "":
		w.paint(stateOn)
		w.setStatus("Включено: YouTube и Discord", widget.SuccessImportance)
	case w.lastErr != "":
		w.paint(stateOff)
		w.setStatus(w.lastErr, widget.DangerImportance)
	default:
		w.paint(stateOff)
		w.setStatus("Выключено", widget.MediumImportance)
	}
	if w.busy || w.closing {
		w.button.Disable()
		w.strategy.Disable()
	} else {
		w.button.Enable()
		w.strategy.Enable()
	}
}

func (w *Window) setStatus(text string, imp widget.Importance) {
	// Label.SetText всегда заново раскладывает текст и перерисовывает окно, а статус
	// обновляется после каждого опроса — без изменений не трогаем.
	if w.status.Text == text && w.status.Importance == imp {
		return
	}
	w.status.Importance = imp
	w.status.SetText(text)
}

func (w *Window) toggle() {
	if w.busy || w.closing {
		return
	}
	if slug := w.running; slug != "" {
		w.runOp("Остановка…", func(ctx context.Context) error { return service.Stop(ctx, slug) })
		return
	}
	slug := w.slugOf(w.strategy.Selected)
	if slug == "" {
		w.showError(errors.New("Не выбрана стратегия"))
		return
	}
	w.runOp("Запуск…", func(ctx context.Context) error { return service.Start(ctx, slug) })
}

func (w *Window) strategyChanged(name string) {
	if w.syncing {
		return
	}
	w.win.Canvas().Unfocus() // иначе список забирает F11
	slug := w.slugOf(name)
	if slug == "" {
		return
	}
	w.cfg.Strategy = slug
	w.saveConfig()
	if w.running == "" && !w.busy {
		w.unitText.setText("zapret-dy@" + slug)
	}
	// Включено на другой стратегии — перезапуск на выбранную (Start сам остановит старую).
	if w.running != "" && w.running != slug && !w.busy {
		w.runOp("Переключение…", func(ctx context.Context) error { return service.Start(ctx, slug) })
	}
}

// runOp выполняет start/stop в горутине: блокирует кнопку, по завершении перечитывает статус.
func (w *Window) runOp(text string, op func(ctx context.Context) error) {
	w.lastErr = ""
	w.busy = true
	w.seq++
	w.paint(stateBusy)
	w.button.Disable()
	w.strategy.Disable()
	w.setStatus(text, widget.WarningImportance)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		defer cancel()
		err := op(ctx)
		// Свой таймаут: контекст операции мог уже истечь, а статус узнать всё равно нужно.
		actx, acancel := context.WithTimeout(context.Background(), statusTimeout)
		active, aerr := service.Active(actx)
		acancel()
		fyne.Do(func() {
			w.busy = false
			w.seq++
			if err != nil {
				w.lastErr = "Ошибка: " + firstLine(err.Error())
			}
			w.applyActive(active, aerr)
			if err != nil {
				w.showError(err)
			}
			if w.closeRequested {
				w.closeRequested = false
				w.requestClose()
			}
		})
	}()
}

// statusLoop следит за состоянием юнитов (zapret могли включить/выключить извне или он упал):
// по сигналам systemd через D-Bus, а если подписаться не вышло — опросом раз в pollInterval.
// Результат отбрасывается, если за время запроса началась операция.
func (w *Window) statusLoop() {
	slugs := make([]string, len(w.list)) // w.list после New не меняется
	for i, s := range w.list {
		slugs[i] = s.Slug
	}
	interval := pollInterval
	changes, err := service.Watch(context.Background(), slugs)
	if err != nil {
		log.Printf("D-Bus недоступен, статус опрашивается каждые %v: %v", pollInterval, err)
	} else {
		interval = watchPollInterval
	}

	t := time.NewTimer(0) // первый раз — сразу
	defer t.Stop()
	for {
		select {
		case <-t.C:
		case _, ok := <-changes:
			if !ok {
				log.Printf("соединение с D-Bus потеряно, статус опрашивается каждые %v", pollInterval)
				changes, interval = nil, pollInterval
				break
			}
			time.Sleep(watchDebounce)
			select { // то, что пришло за время ожидания, учтётся этим же запросом
			case <-changes:
			default:
			}
		}
		next := interval
		if !w.refreshStatus() {
			next = pollInterval // шла операция — проверить вскоре ещё раз
		}
		t.Reset(next)
	}
}

// refreshStatus перечитывает статус; false — пропущено, потому что идёт операция.
func (w *Window) refreshStatus() bool {
	var skip bool
	var seq uint64
	fyne.DoAndWait(func() { skip, seq = w.busy || w.closing, w.seq })
	if skip {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	active, err := service.Active(ctx)
	cancel()
	fyne.Do(func() {
		if w.busy || w.closing || w.seq != seq {
			return
		}
		w.applyActive(active, err)
	})
	return true
}

func (w *Window) applyActive(active []string, err error) {
	if err != nil {
		log.Printf("статус: %v", err)
		// Не просто текст: после операции кнопка и список должны выйти из «Подожди…».
		w.statusErr = err.Error()
		w.render()
		return
	}
	w.statusErr = ""
	w.running = ""
	if len(active) > 0 {
		w.running = active[0]
		// Показываем в списке ту стратегию, что реально работает (её могли включить до запуска окна).
		if name := w.nameOf(w.running); name != "" && w.strategy.Selected != name {
			w.syncing = true
			w.strategy.SetSelected(name)
			w.syncing = false
		}
	}
	w.render()
}

// requestClose — закрытие окна (крестик, SIGTERM/SIGINT). При включённой галочке сначала
// выключает zapret; если выключить не удалось — окно остаётся открытым с ошибкой.
func (w *Window) requestClose() {
	if w.closing {
		return
	}
	if !w.cfg.StopOnClose || w.installErr != nil {
		w.app.Quit()
		return
	}
	if w.busy {
		w.closeRequested = true
		return
	}
	w.closing = true
	w.paint(stateBusy)
	w.button.Disable()
	w.strategy.Disable()
	w.stopOnExit.Disable()
	w.setStatus("Выключение…", widget.WarningImportance)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		defer cancel()
		err := service.StopAll(ctx)
		fyne.Do(func() {
			if err == nil {
				w.app.Quit()
				return
			}
			w.closing = false
			w.stopOnExit.Enable()
			w.render()
			w.showError(fmt.Errorf("Не удалось выключить zapret при закрытии:\n%w\n\n"+
				"Чтобы закрыть окно, не выключая zapret, сними галочку «Выключать при закрытии окна».", err))
		})
	}()
}

func (w *Window) handleSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	for range ch {
		fyne.Do(w.requestClose)
	}
}

func (w *Window) showError(err error) {
	log.Printf("ошибка: %v", err)
	msg := widget.NewLabel(err.Error())
	msg.Wrapping = fyne.TextWrapWord
	msg.Selectable = true
	content := container.NewVBox(msg)

	var se *service.Error
	if errors.As(err, &se) && se.Slug != "" {
		cmd := se.JournalCmd()
		hint := widget.NewLabel("Подробности в журнале:")
		cmdLabel := widget.NewLabel(cmd)
		cmdLabel.TextStyle.Monospace = true
		cmdLabel.Wrapping = fyne.TextWrapBreak
		cmdLabel.Selectable = true
		copyBtn := widget.NewButtonWithIcon("Копировать команду", theme.ContentCopyIcon(), func() {
			w.app.Clipboard().SetContent(cmd)
		})
		content.Add(widget.NewSeparator())
		content.Add(hint)
		content.Add(cmdLabel)
		content.Add(copyBtn)
	}

	d := dialog.NewCustom("Ошибка", "Закрыть", container.NewVScroll(content), w.win)
	d.Resize(fyne.NewSize(300, 340))
	d.Show()
}

func firstLine(s string) string {
	if line, _, ok := strings.Cut(s, "\n"); ok {
		return strings.TrimSuffix(line, ":")
	}
	return s
}

func (w *Window) saveConfig() {
	if w.cfgPath == "" {
		return
	}
	if err := config.Save(w.cfgPath, w.cfg); err != nil {
		log.Printf("не удалось сохранить конфиг: %v", err)
	}
}

// selectedSlug — стратегия из конфига, если она есть в списке, иначе первая (general).
func (w *Window) selectedSlug() string {
	if w.nameOf(w.cfg.Strategy) != "" {
		return w.cfg.Strategy
	}
	if len(w.list) > 0 {
		return w.list[0].Slug
	}
	return ""
}

func (w *Window) nameOf(slug string) string {
	for _, s := range w.list {
		if s.Slug == slug {
			return s.Name
		}
	}
	return ""
}

func (w *Window) slugOf(name string) string {
	for _, s := range w.list {
		if s.Name == name {
			return s.Slug
		}
	}
	return ""
}
