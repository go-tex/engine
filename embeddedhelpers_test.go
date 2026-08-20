package engine

import (
	"strings"
	"testing"
)

// beamer.cls requires these and its own distribution does not contain them.
// Measured over 500 real talks with beamer.cls on the search path and nothing
// else: 1983 pages alone, 3313 with etoolbox and the iftex family, 4286 with
// keyval as well — and every document COMPILED in all three cases, so the loss
// is silent. That is why they are embedded rather than fetched.
func TestSupportPackagesBeamerNeedsAreEmbedded(t *testing.T) {
	for _, name := range []string{
		"etoolbox.sty", "keyval.sty", "iftex.sty",
		"ifetex.sty", "ifluatex.sty", "ifpdf.sty", "ifvtex.sty", "ifxetex.sty",
	} {
		data, ok := embeddedTeXFile(name)
		if !ok {
			t.Errorf("%s n'est pas embarqué", name)
			continue
		}
		// Verbatim upstream files keep their own preamble; a stripped copy would
		// lose the licence notice the LPPL requires to travel with them.
		if !strings.Contains(string(data), "\\ProvidesPackage") {
			t.Errorf("%s ne déclare pas \\ProvidesPackage — copie tronquée ?", name)
		}
	}
}

// A copy on the search path still wins over the embedded one, so a document
// shipping its own etoolbox is not silently overridden by ours.
func TestSearchPathStillBeatsTheEmbeddedHelpers(t *testing.T) {
	opt := Options{Resolve: func(name string) ([]byte, bool) {
		if name == "etoolbox.sty" {
			return []byte("\\def\\zzetoolboxmark{FOURNI}\n"), true
		}
		return nil, false
	}}
	e, err := buildEngine(opt, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run("\\usepackage{etoolbox}\\message{[\\zzetoolboxmark]}")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "[FOURNI]" {
		t.Errorf("sortie = %q — la copie fournie par l'hôte devrait primer sur l'embarquée", out)
	}
}

// And with nothing supplied, the embedded etoolbox is what loads: one of its own
// macros must be defined afterwards.
func TestEmbeddedEtoolboxActuallyLoads(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run("\\usepackage{etoolbox}\\ifdef{\\newrobustcmd}{\\message{[OUI]}}{\\message{[NON]}}")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "[OUI]" {
		t.Errorf("sortie = %q — etoolbox embarqué ne s'est pas chargé", out)
	}
}
