package engine

import (
	"strings"
	"testing"
)

// algorithm2e's \KwIn, \KwOut, \KwData and \KwResult print "Label: text". They
// were all undefined, because \SetKwInput — the command the package DECLARES them
// with — was a gobbler, so a \KwIn was skipped with its argument. That argument is
// prose: one corpus paper writes about fifty words of it per use, 13 uses over 6
// papers.
//
// The expectation is the REFERENCE's, measured with [ruled]algorithm2e: the lines
// read "Input: alpha beta gamma" and "Output: delta epsilon".
func TestKwInputPrintsItsLabelAndItsText(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\KwIn{alpha beta gamma}\KwOut{delta epsilon}`); err != nil {
		t.Fatal(err)
	}
	text := mvlText(e.mvl)
	for _, want := range []string{"Input:alphabetagamma", "Output:deltaepsilon"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
}

// \SetKwInput defines what it is given, which is how the four above exist at all —
// and how a document that declares its own keyword gets it. algorithm2e's own
// \SetKwInput{\KwNote}{Note} is the shape.
func TestSetKwInputDefinesWhatItIsGiven(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\SetKwInput{\KwNote}{Note}\KwNote{careful}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); !strings.Contains(got, "Note:careful") {
		t.Errorf("\\SetKwInput did not define its command: %q", got)
	}
}
