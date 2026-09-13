package engine

import (
	"strings"
	"testing"
)

// Diagnostics carries what the document itself said. The engine collected \message
// and \typeout output all along and no caller could see it — Run returns it, but
// the rendering entry points (CompileToPDFDiag and friends) did not, so every tool
// built on them was deaf to the one channel TeX gives a source for reporting what
// it decided.
func TestDiagnosticsCarryTheDocumentsMessages(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\message{[a]}\message{[b=\the\hsize]}`); err != nil {
		t.Fatal(err)
	}
	got := e.Diagnostics().Messages
	if !strings.Contains(got, "[a]") || !strings.Contains(got, "[b=") {
		t.Errorf("Messages = %q, want both messages", got)
	}
}

// A silent document leaves it empty, so a caller can print a heading only when
// there is something under it.
func TestDiagnosticsMessagesEmptyWhenSilent(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt x\par`); err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().Messages; got != "" {
		t.Errorf("Messages = %q, want empty", got)
	}
}
