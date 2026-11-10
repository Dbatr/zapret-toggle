package service

import (
	"slices"
	"testing"
)

func TestParseUnits(t *testing.T) {
	out := "zapret-dy@general.service loaded active running zapret DPI bypass for YouTube and Discord (strategy general)\n" +
		"zapret-dy@fake-tls-auto.service loaded activating auto-restart zapret DPI bypass\n" +
		"sshd.service loaded active running OpenSSH\n" +
		"zapret-dy@Bad.service loaded active running x\n" +
		"\n"
	got := parseUnits(out)
	want := []unitState{{"general", "active"}, {"fake-tls-auto", "activating"}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestErrorJournalCmd(t *testing.T) {
	e := &Error{Slug: "alt2"}
	if got := e.JournalCmd(); got != "journalctl -u zapret-dy@alt2.service -n 50" {
		t.Fatal(got)
	}
}

func TestFilterStates(t *testing.T) {
	units := []unitState{{"general", "inactive"}, {"alt2", "active"}, {"alt", "failed"}}
	if got := filterStates(slices.Clone(units), []string{"active"}); len(got) != 1 || got[0].slug != "alt2" {
		t.Errorf("active: %v", got)
	}
	if got := filterStates(slices.Clone(units), nil); len(got) != 3 || got[0].slug != "alt" || got[2].slug != "general" {
		t.Errorf("все: %v", got)
	}
}
