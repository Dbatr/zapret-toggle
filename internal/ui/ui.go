// Package ui — окно zapret-toggle: одна большая кнопка, выбор стратегии и «выключать при закрытии».
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
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"zapret-toggle/internal/config"
	"zapret-toggle/internal/service"
	"zapret-toggle/internal/strategies"
)

const (
	pollInterval = 2 * time.Second
	opTimeout    = 45 * time.Second
	stopTimeout  = 20 * time.Second
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
	strategy   *widget.Select
	stopOnExit *widget.Check

	// Только для UI-потока.
	running        string // slug активной стратегии, "" — выключено
	lastErr        string // текст последней ошибки; висит в статусе до следующего действия
	busy           bool   // идёт start/stop
	seq            uint64 // растёт при каждой операции; устаревшие результаты опроса отбрасываются
	syncing        bool   // программная смена выбора стратегии, OnChanged игнорируется
	closing        bool
	closeRequested bool // закрытие запрошено во время операции — выполнить после неё
}

func New(a fyne.App) *Window {
	w := &Window{app: a, win: a.NewWindow("Zapret Toggle")}

	var err error
	if w.cfgPath, err = config.Path(); err == nil {
		w.cfg, err = config.Load(w.cfgPath)
	}
	if err != nil {
		log.Printf("конфиг: %v (используются настройки по умолчанию)", err)
		w.cfg = config.Default()
	}

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
			go w.pollLoop()
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

	subtitle := widget.NewLabel("YouTube · Discord")
	subtitle.Alignment = fyne.TextAlignCenter
	subtitle.Importance = widget.MediumImportance

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
		w.cfg.StopOnClose = on
		w.saveConfig()
	})
	w.stopOnExit.SetChecked(w.cfg.StopOnClose)

	strategyLabel := widget.NewLabel("Стратегия")
	strategyLabel.TextStyle.Bold = true
	bottom := container.NewVBox(
		widget.NewSeparator(),
		container.NewBorder(nil, nil, strategyLabel, nil, w.strategy),
		w.stopOnExit,
	)
	center := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(w.button),
		w.status,
		layout.NewSpacer(),
	)
	top := container.NewVBox(subtitle)

	w.win.SetContent(container.NewPadded(container.NewBorder(top, bottom, nil, nil, center)))
	w.win.Resize(fyne.NewSize(320, 420))
	w.win.SetFixedSize(true)
	w.win.CenterOnScreen()

	if w.installErr != nil {
		w.button.Disable()
		w.strategy.Disable()
		msg := "zapret-dy не установлен.\nЗапусти: sudo scripts/install.sh"
		if !errors.Is(w.installErr, service.ErrNotInstalled) {
			msg = "Установка zapret-dy повреждена:\n" + w.installErr.Error() + "\nПереустанови: sudo scripts/install.sh"
		}
		w.setStatus(msg, widget.DangerImportance)
		return
	}
	w.render()
}

// render приводит кнопку и статус в соответствие с running/busy.
func (w *Window) render() {
	switch {
	case w.running != "":
		w.button.SetColor(colorOn, false)
		w.setStatus("Включено: YouTube и Discord", widget.SuccessImportance)
	case w.lastErr != "":
		w.button.SetColor(colorOff, false)
		w.setStatus(w.lastErr, widget.DangerImportance)
	default:
		w.button.SetColor(colorOff, false)
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
	slug := w.slugOf(name)
	if slug == "" {
		return
	}
	w.cfg.Strategy = slug
	w.saveConfig()
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
	w.button.SetColor(colorBusy, true)
	w.button.Disable()
	w.strategy.Disable()
	w.setStatus(text, widget.WarningImportance)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		defer cancel()
		err := op(ctx)
		active, aerr := service.Active(ctx)
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

// pollLoop раз в pollInterval читает список активных юнитов (zapret могли включить/выключить
// извне или он упал). Результат отбрасывается, если за время запроса началась операция.
func (w *Window) pollLoop() {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for ; ; <-t.C {
		var skip bool
		var seq uint64
		fyne.DoAndWait(func() { skip, seq = w.busy || w.closing, w.seq })
		if skip {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), pollInterval*2)
		active, err := service.Active(ctx)
		cancel()
		fyne.Do(func() {
			if w.busy || w.closing || w.seq != seq {
				return
			}
			w.applyActive(active, err)
		})
	}
}

func (w *Window) applyActive(active []string, err error) {
	if err != nil {
		log.Printf("статус: %v", err)
		w.setStatus("Не удалось получить статус:\n"+err.Error(), widget.DangerImportance)
		return
	}
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
	w.button.SetColor(colorBusy, true)
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
