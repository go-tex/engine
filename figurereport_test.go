// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"encoding/base64"
	"testing"
)

// A figure whose file cannot be loaded is reported by CAUSE, not as a skipped
// \includegraphics. The command is defined and did its job — it reserved the
// placeholder box — so naming it sends a reader looking for a missing macro. On a
// 333-page corpus book this turned "63 undefined \includegraphics" (a lie that
// costs an hour) into "63 PDF figures, no rasteriser wired" (a cause).
func TestFigureDropIsReportedByCauseNotAsUndefinedCommand(t *testing.T) {
	// The sources are a data: URI and a plain relative name — never a temp-directory
	// PATH. A t.TempDir() path is read here as TeX source, where a "#" (which Go puts
	// in a subtest's directory name) is a parameter character and a Windows "\\Users"
	// is a control sequence: the figure then fails to resolve and every cause comes
	// back "file not found". That passed on macOS and failed on Linux and Windows.
	pdf := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\n%%EOF\n"))
	for _, c := range []struct {
		name string
		file string
		want string
	}{
		{"an absent file", "no-such-figure-here.png", "file not found"},
		{"a PDF with no rasteriser", pdf, "PDF figure, no rasteriser wired"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.LoadLaTeX()
			e.SetFont(spMock{})
			e.lenient = true
			if _, err := e.Run(`\noindent A\includegraphics[width=1cm]{` + c.file + `}B`); err != nil {
				t.Fatal(err)
			}
			d := e.Diagnostics()
			if d.FiguresDropped[c.want] != 1 {
				t.Errorf("FiguresDropped = %v, want one %q", d.FiguresDropped, c.want)
			}
			// The command must NOT be blamed: that is the whole point.
			if n := d.Skipped["includegraphics"]; n != 0 {
				t.Errorf("\\includegraphics reported as skipped %d time(s); it is defined and ran", n)
			}
			// The text on both sides still flows around the reserved box.
			if got := mvlText(e.mvl); got != "AB" {
				t.Errorf("surrounding text = %q, want %q", got, "AB")
			}
		})
	}
}

// A figure that loads is reported not at all.
func TestLoadableFigureIsNotReported(t *testing.T) {
	uri := pngDataURI(t, 4, 3) // a real PNG, handed in as a data: URI
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.lenient = true
	if _, err := e.Run(`\noindent\includegraphics[width=1cm]{` + uri + `}`); err != nil {
		t.Fatal(err)
	}
	if d := e.Diagnostics(); len(d.FiguresDropped) != 0 {
		t.Errorf("a figure that loaded was reported dropped: %v", d.FiguresDropped)
	}
}
