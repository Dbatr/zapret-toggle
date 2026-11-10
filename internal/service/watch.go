package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	systemdDest     = "org.freedesktop.systemd1"
	systemdPath     = dbus.ObjectPath("/org/freedesktop/systemd1")
	systemdUnitPath = "/org/freedesktop/systemd1/unit/"
)

// Watch сообщает об изменении состояния юнитов zapret-dy@<slug> для перечисленных slugs
// по сигналам systemd через системную шину D-Bus — вместо того чтобы раз в пару секунд
// запускать systemctl. Сигналы одной пачки (запуск даёт их несколько) склеиваются:
// в канале лежит не больше одного уведомления. Канал закрывается при отмене ctx или обрыве
// соединения с шиной — тогда вызывающему стоит вернуться к опросу.
func Watch(ctx context.Context, slugs []string) (<-chan struct{}, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	ch, err := watch(ctx, conn, slugs)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ch, nil
}

// watch подписывается на изменения юнитов на уже открытом conn и забирает его себе:
// conn закрывается вместе с каналом.
func watch(ctx context.Context, conn *dbus.Conn, slugs []string) (<-chan struct{}, error) {
	// Без Subscribe systemd не рассылает сигналы об изменениях юнитов. Отписка — сама,
	// когда закрывается соединение.
	if err := conn.Object(systemdDest, systemdPath).
		CallWithContext(ctx, systemdDest+".Manager.Subscribe", 0).Err; err != nil {
		return nil, fmt.Errorf("systemd Subscribe: %w", err)
	}
	// Точные пути юнитов: просыпаемся только от своих, а не от всех юнитов системы.
	// ActiveState/SubState меняются в интерфейсе Unit (arg0 сигнала PropertiesChanged).
	for _, slug := range slugs {
		if err := conn.AddMatchSignalContext(ctx,
			dbus.WithMatchSender(systemdDest),
			dbus.WithMatchObjectPath(unitPath(Unit(slug))),
			dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
			dbus.WithMatchMember("PropertiesChanged"),
			dbus.WithMatchArg(0, systemdDest+".Unit"),
		); err != nil {
			return nil, fmt.Errorf("D-Bus AddMatch: %w", err)
		}
	}

	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		defer conn.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case s, ok := <-signals:
				if !ok {
					return // соединение с шиной закрылось
				}
				if !strings.HasPrefix(string(s.Path), systemdUnitPath) {
					continue
				}
				select {
				case out <- struct{}{}:
				default: // уведомление уже ждёт — этого достаточно
				}
			}
		}
	}()
	return out, nil
}

// listUnitsDBus — listUnits через общее соединение с системной шиной (godbus держит его
// открытым и подключается заново, если оно оборвалось).
func listUnitsDBus(ctx context.Context, slugs, states []string) ([]unitState, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	return listUnitsOn(ctx, conn, slugs, states)
}

func listUnitsOn(ctx context.Context, conn *dbus.Conn, slugs, states []string) ([]unitState, error) {
	// ListUnitsByNames отвечает только про названные юниты (незагруженный — inactive),
	// ListUnitsByPatterns перебирает все юниты системы. Элемент ответа: (ssssssouso).
	method, args := "ListUnitsByPatterns", []any{[]string{}, []string{unitPrefix + "*"}}
	if len(slugs) > 0 {
		names := make([]string, len(slugs))
		for i, slug := range slugs {
			names[i] = Unit(slug)
		}
		method, args = "ListUnitsByNames", []any{names}
	}
	var rows []struct {
		Name, Description, Load, Active, Sub, Following string
		Path                                            dbus.ObjectPath
		JobID                                           uint32
		JobType                                         string
		JobPath                                         dbus.ObjectPath
	}
	if err := conn.Object(systemdDest, systemdPath).
		CallWithContext(ctx, systemdDest+".Manager."+method, 0, args...).Store(&rows); err != nil {
		return nil, fmt.Errorf("systemd %s: %w", method, err)
	}
	units := make([]unitState, 0, len(rows))
	for _, r := range rows {
		if slug, ok := unitSlug(r.Name); ok {
			units = append(units, unitState{slug: slug, active: r.Active})
		}
	}
	return filterStates(units, states), nil
}

// unitPath — объектный путь юнита в systemd (как bus_label_escape: всё, кроме букв
// и не первых цифр, заменяется на _XX).
func unitPath(unit string) dbus.ObjectPath {
	var b strings.Builder
	b.WriteString(systemdUnitPath)
	for i := 0; i < len(unit); i++ {
		c := unit[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "_%02x", c)
		}
	}
	return dbus.ObjectPath(b.String())
}
