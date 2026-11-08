package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestUnitPath(t *testing.T) {
	for unit, want := range map[string]dbus.ObjectPath{
		"zapret-dy@general.service":        "/org/freedesktop/systemd1/unit/zapret_2ddy_40general_2eservice",
		"zapret-dy@alt-2.service":          "/org/freedesktop/systemd1/unit/zapret_2ddy_40alt_2d2_2eservice",
		"1zapret-dy@fake-tls-auto.service": "/org/freedesktop/systemd1/unit/_31zapret_2ddy_40fake_2dtls_2dauto_2eservice",
	} {
		if got := unitPath(unit); got != want {
			t.Errorf("unitPath(%q) = %q, want %q", unit, got, want)
		}
	}
}

// Проверка на пользовательском systemd (тот же API, что у системного): уведомление приходит
// от своего юнита и не приходит от чужого. Без пользовательской шины тест пропускается.
func TestWatchUserManager(t *testing.T) {
	if _, err := exec.LookPath("systemd-run"); err != nil {
		t.Skip("нет systemd-run")
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Skipf("нет сессионной шины: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	mine := fmt.Sprintf("watchtest-%d", os.Getpid())
	other := mine + "-other"
	ch, err := watch(ctx, conn, []string{mine})
	if err != nil {
		conn.Close()
		t.Skipf("пользовательский systemd недоступен: %v", err)
	}
	run := func(slug string) {
		t.Helper()
		if out, err := exec.Command("systemd-run", "--user", "--quiet", "--collect",
			"--unit="+Unit(slug), "true").CombinedOutput(); err != nil {
			t.Skipf("systemd-run: %v: %s", err, out)
		}
	}

	run(other)
	select {
	case <-ch:
		t.Fatal("уведомление от чужого юнита")
	case <-time.After(700 * time.Millisecond):
	}

	run(mine)
	select {
	case <-ch:
	case <-ctx.Done():
		t.Fatal("нет уведомления об изменении своего юнита")
	}

	cancel()
	for range ch { // канал должен закрыться после отмены
	}
}
