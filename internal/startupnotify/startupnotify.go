// Package startupnotify завершает startup notification (курсор «загрузка» в GNOME).
//
// GNOME Shell передаёт запущенному из меню приложению DESKTOP_STARTUP_ID (даже при
// StartupNotify=false в .desktop) и крутит курсор, пока приложение не сообщит, что запустилось,
// или до таймаута. GTK делает это сам, а X11-бэкенд GLFW в Fyne — нет, поэтому сообщение
// «remove» по протоколу startup-notification отправляем здесь.
//
// Сообщение идёт через X11 (libX11), в Wayland-сессии — через XWayland. В сборке с тегом wayland
// без x11 (чистый Wayland, без X11-библиотек) отправки нет: Complete только чистит окружение.
package startupnotify

import (
	"os"
	"strings"
)

// Complete сообщает оконному менеджеру, что запуск завершён. Вызывать после показа окна.
// Без DESKTOP_STARTUP_ID ничего не делает. Переменные удаляются из окружения, чтобы их
// не унаследовали дочерние процессы (systemctl).
func Complete() error {
	id := os.Getenv("DESKTOP_STARTUP_ID")
	os.Unsetenv("DESKTOP_STARTUP_ID")
	if os.Getenv("XDG_ACTIVATION_TOKEN") == id {
		os.Unsetenv("XDG_ACTIVATION_TOKEN")
	}
	if id == "" || os.Getenv("DISPLAY") == "" {
		return nil
	}
	return broadcast(removeMessage(id))
}

// removeMessage — `remove: ID="<id>"` с экранированием как в спецификации startup-notification
// (внутри кавычек экранируются только " и \). Нулевой байт в конце входит в сообщение.
func removeMessage(id string) []byte {
	var b strings.Builder
	b.WriteString(`remove: ID="`)
	for _, r := range id {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteString("\"\x00")
	return []byte(b.String())
}
