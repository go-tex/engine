package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInputSplicesFile(t *testing.T) {
	dir := t.TempDir()
	inc := filepath.Join(dir, "inc.tex")
	os.WriteFile(inc, []byte(`\count5=42 `), 0644)
	e := New()
	// \input runs the file, then the rest of the line continues.
	got, err := e.Run(`\input ` + inc + ` \message{\the\count5}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if trimNL(got) != "42" {
		t.Errorf("got %q want 42", trimNL(got))
	}
}

func TestInputAddsTexExtension(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "m.tex"), []byte(`\def\hi{HELLO}`), 0644)
	e := New()
	got, _ := e.Run(`\input ` + filepath.Join(dir, "m") + ` \message{\hi}`)
	if trimNL(got) != "HELLO" {
		t.Errorf("got %q want HELLO", trimNL(got))
	}
}

func TestFontPrimitiveSelectsFont(t *testing.T) {
	font := "/System/Library/Fonts/Supplemental/Georgia.ttf"
	if _, err := os.Stat(font); err != nil {
		t.Skip("system font not present")
	}
	e := New()
	// \font defines \rm; using \rm sets the current font; text then measures.
	if _, err := e.Run(`\font\rm=` + font + ` at 10pt \rm\setbox0=\hbox{Hi}`); err != nil {
		t.Fatalf("err: %v", err)
	}
	b := e.box[0]
	if b == nil || b.width <= 0 {
		t.Fatalf("box0 has no measured width: %+v", b)
	}
}
