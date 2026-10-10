package engine

import (
	"strings"
	"testing"
)

// \citeauthor{k} prints the author label alone — no number, no brackets. It is
// PROSE: a sentence reads "as \citeauthor{knuth} showed", and skipping the command
// left "as showed". 4 papers and 14 uses of the fidelity corpus write it into a
// sentence that way.
func TestCiteauthorPrintsTheAuthorAlone(t *testing.T) {
	base := writeBib(t)
	src := `\noindent As \citeauthor{knuth} showed.\bibliography{` + base + `}`
	e := runTwoPass(t, src)
	text := mvlText(e.mvl)
	if !strings.Contains(text, "AsKnuthshowed") {
		t.Errorf("\\citeauthor did not put the author in the sentence: %q", text)
	}
	// And no number came with it: \citeauthor is not \citet.
	if strings.Contains(text, "Knuth[1]") {
		t.Errorf("\\citeauthor printed \\citet's number too: %q", text)
	}
}

// A key with no carried author prints nothing rather than a stray separator: the
// label is what this command has to say, and it has none.
func TestCiteauthorWithNoAuthorIsSilent(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.labels = map[string]string{"a": "3"}
	if _, err := e.Run(`\noindent x\citeauthor{a}y`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "xy" {
		t.Errorf("\\citeauthor with no author typeset %q, want %q", got, "xy")
	}
	// It still counts as a citation, as natbib's does: the entry must appear in the
	// bibliography even when the inline label is empty.
	if !e.citedKeys["a"] {
		t.Error("\\citeauthor did not record its key as cited")
	}
}

// \citenum{k} prints the number alone, unbracketed — what a document writes when
// it builds its own brackets or a superscript around it.
func TestCitenumPrintsTheBareNumber(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.labels = map[string]string{"a": "1", "b": "2"}
	if _, err := e.Run(`\noindent[\citenum{a},\citenum{b}]`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "[1,2]" {
		t.Errorf("\\citenum typeset %q, want %q — the brackets are the DOCUMENT's", got, "[1,2]")
	}
}
