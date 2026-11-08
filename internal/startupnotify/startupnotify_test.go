package startupnotify

import (
	"os"
	"testing"
)

func TestRemoveMessage(t *testing.T) {
	got := string(removeMessage(`gnome-shell/Zapret Toggle/3173-19-fedora_TIME9681097`))
	want := "remove: ID=\"gnome-shell/Zapret Toggle/3173-19-fedora_TIME9681097\"\x00"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := string(removeMessage(`a"b\c`)); got != "remove: ID=\"a\\\"b\\\\c\"\x00" {
		t.Errorf("экранирование: %q", got)
	}
}

func TestCompleteClearsEnv(t *testing.T) {
	t.Setenv("DISPLAY", "") // без X-сервера — только чистка окружения
	t.Setenv("DESKTOP_STARTUP_ID", "x")
	t.Setenv("XDG_ACTIVATION_TOKEN", "x")
	if err := Complete(); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"DESKTOP_STARTUP_ID", "XDG_ACTIVATION_TOKEN"} {
		if _, ok := os.LookupEnv(k); ok {
			t.Errorf("%s не удалена", k)
		}
	}
}
