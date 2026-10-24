package strategies

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	in := "general\tgeneral\nalt2\tgeneral (ALT2)\n\nfake-tls-auto\tgeneral (FAKE TLS AUTO)\nalt2\tdup\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []Strategy{
		{"general", "general"},
		{"alt2", "general (ALT2)"},
		{"fake-tls-auto", "general (FAKE TLS AUTO)"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParseRejectsBadSlug(t *testing.T) {
	for _, in := range []string{"../x\tx\n", "Alt\tx\n", "a b\tx\n", "\n"} {
		if _, err := Parse(strings.NewReader(in)); err == nil {
			t.Errorf("Parse(%q): ожидалась ошибка", in)
		}
	}
}
