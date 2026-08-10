package engine

import (
	"os"
	"testing"
)

// With a bold font in Options, \bf is bound to it and \textbf switches to it —
// so the bold face's glyph metrics differ from the roman face for the same text.
func TestFontFamilyBold(t *testing.T) {
	reg := "/System/Library/Fonts/Supplemental/Georgia.ttf"
	bold := "/System/Library/Fonts/Supplemental/Georgia Bold.ttf"
	if _, err := os.Stat(bold); err != nil {
		t.Skip("no bold system font")
	}
	rb, _ := os.ReadFile(reg)
	bb, _ := os.ReadFile(bold)
	e, err := buildEngine(Options{Font: rb, BoldFont: bb, Size: 20}, true) // latex layer
	if err != nil {
		t.Fatal(err)
	}
	// \bf must be bound to a font (not the kernel's no-op macro)
	if m := e.eq["bf"]; m == nil || m.kind != mFont {
		t.Fatalf("\\bf not bound to a font: %+v", e.eq["bf"])
	}
	// \textbf{VA} should measure with the bold face; compare against roman VA
	e.Run(`\setbox0=\hbox{VA}\setbox1=\hbox{\textbf{VA}}`)
	rw, bw := e.box[0].width, e.box[1].width
	if rw == bw {
		t.Errorf("bold VA width == roman VA width (%d); expected the bold face to differ", rw)
	}
}
