package engine

import (
	"strings"
	"testing"
)

// The description environment sets each \item[term] as a bold term (the \bf is a
// no-op without a bound bold font, so the glyphs are plain but present) placed in
// the left margin, followed by the item text indented like the other lists.
func TestDescriptionTerms(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{description}\item[Chat] félin\item[Chien] canin\end{description}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := mvlText(e.mvl), "ChatfélinChiencanin"; got != want {
		t.Fatalf("description typeset %q, want %q", got, want)
	}
}

// Each description line is indented by \leftskip (24pt) exactly like itemize and
// enumerate, so the two terms land on two distinct indented lines.
func TestDescriptionIndent(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{description}\item[Chat] félin\item[Chien] canin\end{description}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	lines := lineLeftskips(e.mvl)
	if len(lines) != 2 {
		t.Fatalf("expected 2 description lines, got %d: %+v", len(lines), lines)
	}
	const pt = 65536
	want := []struct {
		skip int
		text string
	}{
		{24 * pt, "Chatfélin"},
		{24 * pt, "Chiencanin"},
	}
	for i, w := range want {
		if lines[i].skip != w.skip || lines[i].text != w.text {
			t.Errorf("line %d = {skip %d, %q}, want {skip %d, %q}",
				i, lines[i].skip, lines[i].text, w.skip, w.text)
		}
	}
}

// In an itemize, \item without a bracket keeps its default bullet, while
// \item[!] replaces the bullet with the supplied label — no regression to the
// default case.
func TestItemizeCustomLabel(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{itemize}\item normal\item[!] custom\end{itemize}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	got := mvlText(e.mvl)
	if got != "•normal!custom" {
		t.Fatalf("itemize custom-label typeset %q, want %q", got, "•normal!custom")
	}
	if !strings.Contains(got, "!custom") {
		t.Fatalf("expected custom label %q before its text, got %q", "!", got)
	}
	if !strings.HasPrefix(got, "•normal") {
		t.Fatalf("expected first item to keep its bullet, got %q", got)
	}
}

// In an enumerate, \item[X] replaces the running number with the supplied label,
// while unbracketed items keep their numbers.
func TestEnumerateCustomLabel(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{enumerate}\item one\item[X] two\item three\end{enumerate}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := mvlText(e.mvl), "1.oneXtwo2.three"; got != want {
		t.Fatalf("enumerate custom-label typeset %q, want %q", got, want)
	}
}
