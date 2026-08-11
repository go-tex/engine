package engine

import "testing"

// countVboxesOfWidth counts every vbox of the given width reachable in a tree.
func countVboxesOfWidth(nodes []node, width int) int {
	n := 0
	for _, nd := range nodes {
		if b, ok := nd.(*boxNode); ok {
			if b.kind == vbox && b.width == width {
				n++
			}
			n += countVboxesOfWidth(b.list, width)
		}
	}
	return n
}

// \begin{minipage}{20pt} typesets its body to a 20pt vbox; a run of words wider
// than the measure wraps onto several lines.
func TestMinipage(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{minipage}{20pt}aaaa aaaa aaaa\end{minipage}`); err != nil {
		t.Fatal(err)
	}
	mp, ok := firstVboxOfWidth(e.mvl, 20*unity)
	if !ok {
		t.Fatal("no 20pt minipage vbox placed")
	}
	lines := 0
	for _, n := range mp.list {
		if _, ok := n.(*boxNode); ok {
			lines++
		}
	}
	if lines < 2 {
		t.Errorf("minipage wrapped into %d lines, want ≥2", lines)
	}
}

// Two minipages side by side both build and both produce a vbox of their measure.
func TestMinipageSideBySide(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\begin{minipage}{60pt}A B C D\end{minipage}\begin{minipage}{60pt}E F G H\end{minipage}`); err != nil {
		t.Fatal(err)
	}
	if got := countVboxesOfWidth(e.mvl, 60*unity); got != 2 {
		t.Errorf("found %d 60pt minipage vboxes, want 2", got)
	}
}

// A minipage nesting a list must not be terminated by the inner \end{itemize};
// only the matching \end{minipage} at depth 0 closes it.
func TestMinipageNestedList(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{minipage}{100pt}\begin{itemize}\item X\end{itemize}\end{minipage}`); err != nil {
		t.Fatal(err)
	}
	if _, ok := firstVboxOfWidth(e.mvl, 100*unity); !ok {
		t.Fatal("no 100pt minipage vbox placed (inner \\end{itemize} closed it early?)")
	}
}

// A minipage nesting a minipage: only the outer \end{minipage} closes the outer
// environment; the inner one is re-collected and typeset as its own vbox.
func TestMinipageNestedMinipage(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\begin{minipage}{100pt}\begin{minipage}{40pt}a b c\end{minipage}\end{minipage}`); err != nil {
		t.Fatal(err)
	}
	if _, ok := firstVboxOfWidth(e.mvl, 100*unity); !ok {
		t.Fatal("no 100pt outer minipage vbox placed")
	}
	if _, ok := firstVboxOfWidth(e.mvl, 40*unity); !ok {
		t.Fatal("no 40pt inner minipage vbox placed")
	}
}

// braceNameToks rebuilds a {name} group as explicit-begin/other.../explicit-end
// so a re-emitted \begin{other}/\end{other} tokenizes back to the same name.
func TestBraceNameToks(t *testing.T) {
	toks := braceNameToks("itemize")
	if len(toks) != len("itemize")+2 {
		t.Fatalf("got %d tokens, want %d", len(toks), len("itemize")+2)
	}
	if !toks[0].is('{', catBegin) {
		t.Errorf("first token = %v, want {", toks[0])
	}
	if !toks[len(toks)-1].is('}', catEnd) {
		t.Errorf("last token = %v, want }", toks[len(toks)-1])
	}
	var name []rune
	for _, tk := range toks[1 : len(toks)-1] {
		if tk.cs_ {
			t.Fatalf("unexpected cs token in name: %v", tk)
		}
		name = append(name, tk.ch)
	}
	if string(name) != "itemize" {
		t.Errorf("reconstructed name = %q, want itemize", string(name))
	}
}
