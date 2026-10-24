// Package service управляет юнитами zapret-dy@<slug>.service через systemctl.
//
// Root не нужен: start/stop/restart этих юнитов разрешает polkit-правило 50-zapret-dy.rules.
// Все функции блокирующие — вызывать из горутин, не из UI-потока.
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"zapret-toggle/internal/strategies"
)

const (
	BaseDir  = "/opt/zapret-dy"
	UnitFile = "/etc/systemd/system/zapret-dy@.service"

	unitPrefix = "zapret-dy@"
	unitSuffix = ".service"
)

// Сколько ждать после `systemctl start`, чтобы убедиться, что nfqws не упал сразу.
// Type=simple отвечает «запущено» до того, как nfqws разберёт аргументы.
var (
	settleTime   = 1500 * time.Millisecond
	pollInterval = 250 * time.Millisecond
)

// ErrNotInstalled — системная часть не установлена (нет /opt/zapret-dy или юнита).
var ErrNotInstalled = errors.New("zapret-dy не установлен")

// Error — ошибка операции над конкретной стратегией; по Slug UI строит подсказку про journalctl.
type Error struct {
	Slug string
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// JournalCmd — команда, которой смотреть лог юнита.
func (e *Error) JournalCmd() string {
	return "journalctl -u " + Unit(e.Slug) + " -n 50"
}

// Unit возвращает имя юнита для стратегии.
func Unit(slug string) string {
	return unitPrefix + slug + unitSuffix
}

// Installed проверяет, что системная часть на месте.
func Installed() error {
	for _, p := range []string{UnitFile, BaseDir + "/nfqws", strategies.Dir + "/index.txt"} {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("%w: нет %s", ErrNotInstalled, p)
		}
	}
	return nil
}

// Active возвращает slug'и активных (active) юнитов zapret-dy@*.
func Active(ctx context.Context) ([]string, error) {
	out, err := systemctl(ctx, "list-units", "zapret-dy@*", "--state=active", "--plain", "--no-legend")
	if err != nil {
		return nil, err
	}
	var slugs []string
	for _, u := range parseUnits(out) {
		slugs = append(slugs, u.slug)
	}
	return slugs, nil
}

// Start запускает стратегию slug, предварительно остановив все остальные zapret-dy@*
// (у них общая таблица nftables и очередь 200), и ждёт, что nfqws не упал сразу после старта.
func Start(ctx context.Context, slug string) error {
	if !strategies.ValidSlug(slug) {
		return fmt.Errorf("недопустимое имя стратегии: %q", slug)
	}
	if err := stopAll(ctx, slug); err != nil {
		return err
	}
	unit := Unit(slug)
	if _, err := systemctl(ctx, "--no-ask-password", "start", unit); err != nil {
		return &Error{Slug: slug, Msg: "Не удалось запустить " + unit + ":\n" + explain(err)}
	}
	if err := waitRunning(ctx, unit); err != nil {
		// Иначе Restart=on-failure будет перезапускать сломанную стратегию.
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		_, _ = systemctl(stopCtx, "--no-ask-password", "stop", unit)
		cancel()
		return &Error{Slug: slug, Msg: err.Error()}
	}
	return nil
}

// Stop останавливает стратегию slug.
func Stop(ctx context.Context, slug string) error {
	if !strategies.ValidSlug(slug) {
		return fmt.Errorf("недопустимое имя стратегии: %q", slug)
	}
	if _, err := systemctl(ctx, "--no-ask-password", "stop", Unit(slug)); err != nil {
		return &Error{Slug: slug, Msg: "Не удалось остановить " + Unit(slug) + ":\n" + explain(err)}
	}
	return nil
}

// StopAll останавливает все работающие (в том числе перезапускающиеся) zapret-dy@*.
func StopAll(ctx context.Context) error {
	return stopAll(ctx, "")
}

func stopAll(ctx context.Context, except string) error {
	out, err := systemctl(ctx, "list-units", "zapret-dy@*", "--all", "--plain", "--no-legend")
	if err != nil {
		return err
	}
	for _, u := range parseUnits(out) {
		if u.slug == except || u.active == "inactive" || u.active == "failed" {
			continue
		}
		if err := Stop(ctx, u.slug); err != nil {
			return err
		}
	}
	return nil
}

// waitRunning следит за юнитом settleTime и возвращает ошибку, если nfqws упал
// (юнит failed, ушёл в auto-restart или неактивен).
func waitRunning(ctx context.Context, unit string) error {
	deadline := time.Now().Add(settleTime)
	for {
		props, err := show(ctx, unit, "ActiveState", "SubState", "Result")
		if err != nil {
			return err
		}
		state, sub := props["ActiveState"], props["SubState"]
		if state == "failed" || state == "inactive" || sub == "auto-restart" {
			return fmt.Errorf("nfqws завершился сразу после запуска (%s, %s, result=%s).\n"+
				"Возможно, стратегия не подходит или повреждены файлы в %s", state, sub, props["Result"], BaseDir)
		}
		if time.Now().After(deadline) {
			if state == "active" {
				return nil
			}
			return fmt.Errorf("%s не перешёл в состояние active (сейчас %s/%s)", unit, state, sub)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func show(ctx context.Context, unit string, props ...string) (map[string]string, error) {
	out, err := systemctl(ctx, "show", unit, "--property="+strings.Join(props, ","))
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for line := range strings.Lines(out) {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			m[k] = v
		}
	}
	return m, nil
}

// explain добавляет подсказку к отказу polkit (правило не установлено или сессия не активна).
func explain(err error) string {
	msg := err.Error()
	low := strings.ToLower(msg)
	if strings.Contains(low, "authentication required") || strings.Contains(low, "access denied") {
		msg += "\n\npolkit не разрешил операцию. Проверь, что выполнен sudo scripts/install.sh " +
			"(ставит /etc/polkit-1/rules.d/50-zapret-dy.rules) и что это активная локальная сессия."
	}
	return msg
}

type unitState struct {
	slug   string
	active string // столбец ACTIVE: active, activating, failed, ...
}

// parseUnits разбирает вывод `systemctl list-units --plain --no-legend`:
// "zapret-dy@general.service loaded active running описание...".
func parseUnits(out string) []unitState {
	var units []unitState
	for line := range strings.Lines(out) {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		slug, ok := strings.CutPrefix(f[0], unitPrefix)
		if !ok {
			continue
		}
		slug, ok = strings.CutSuffix(slug, unitSuffix)
		if !ok || !strategies.ValidSlug(slug) {
			continue
		}
		units = append(units, unitState{slug: slug, active: f[2]})
	}
	return units
}

func systemctl(ctx context.Context, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("systemctl %s: превышено время ожидания", strings.Join(args, " "))
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}
