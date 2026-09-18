package main

import (
	"os"
	"strings"
	"testing"
)

const liberation = "/usr/share/fonts/truetype/liberation/LiberationSerif-Regular.ttf"

func TestSelfMatchIsIdentity(t *testing.T) {
	if _, err := os.Stat(liberation); err != nil {
		t.Skip("fonts-liberation not installed")
	}
	m, err := read(liberation)
	if err != nil {
		t.Fatal(err)
	}
	r := rule(m, m, "X", "normal", "400", `local("X")`)
	if !strings.Contains(r, "size-adjust:100%") || !strings.Contains(r, "ascent-override:"+pct(m.ascent/m.upem)) {
		t.Errorf("self match must be identity: %s", r)
	}
	if m.upem != 2048 || m.ascent <= 0 || m.descent <= 0 {
		t.Errorf("metrics look wrong: %+v", m)
	}
}

func TestOS2(t *testing.T) {
	b, err := os.ReadFile(liberation)
	if err != nil {
		t.Skip("fonts-liberation not installed")
	}
	o, ok := readOS2(b)
	if !ok || o.typoAsc <= 0 || o.typoDesc <= 0 || o.winAsc < o.typoAsc || o.winDesc < o.typoDesc {
		t.Errorf("OS/2 metrics look wrong: %+v ok=%v", o, ok)
	}
}

func TestPct(t *testing.T) {
	for in, want := range map[float64]string{1: "100%", 0.9512: "95.12%", 0.2634567: "26.346%", 0: "0%"} {
		if got := pct(in); got != want {
			t.Errorf("pct(%v) = %s want %s", in, got, want)
		}
	}
}
