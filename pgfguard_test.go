package engine

import "testing"

// pgf ships far more than the three headline names: pgfcore, pgfmath, pgfpages,
// pgffor, pgfkeys, pgfsys, pgfrcs. beamer REQUIRES pgfpages, pgfmath and pgfcore
// directly, so guarding only {tikz, pgf, pgfplots} meant that the moment pgf's
// sources were findable — easy now that a host can hand the engine a texmf tree —
// beamer pulled the real pgfcore through its own \RequirePackage, whatever
// GOTEX_PGF said.
//
// Measured over 500 real talks with beamer.cls present: 4286 pages with pgf out
// of reach, 1487 with its sources on the search path and GOTEX_PGF UNSET, and
// 4286 again with the family recognised. Reachability was the switch, not the
// flag.
func TestPGFFamilyIsRecognised(t *testing.T) {
	for _, name := range []string{
		"pgf", "pgfplots", "pgfcore", "pgfmath", "pgfpages",
		"pgffor", "pgfkeys", "pgfsys", "pgfrcs", "pgfcalendar", "tikz",
	} {
		if !isPGFFamily(name) {
			t.Errorf("%q devrait appartenir à la famille pgf", name)
		}
	}
	// Names that merely look adjacent must not be swept in.
	for _, name := range []string{"graphicx", "xcolor", "etoolbox", "beamer", "article", "tikzscale"} {
		if isPGFFamily(name) {
			t.Errorf("%q ne devrait PAS être traité comme pgf", name)
		}
	}
}

// The guard is what decides, and it follows GOTEX_PGF: emulated by default,
// loadable when the flag is set — so the opt-in path used to bring the real
// sources up still works.
func TestPGFFamilyIsEmulatedUnlessOptedIn(t *testing.T) {
	for _, name := range []string{"pgfcore", "pgfmath", "pgfpages", "tikz"} {
		if !emulateOnly(name) {
			t.Errorf("%q devrait être émulé par défaut", name)
		}
	}
	t.Setenv("GOTEX_PGF", "1")
	for _, name := range []string{"pgfcore", "pgfmath", "pgfpages", "tikz"} {
		if emulateOnly(name) {
			t.Errorf("%q devrait être chargeable avec GOTEX_PGF=1", name)
		}
	}
	// The unrelated never-load list is untouched by the flag.
	if !emulateOnly("hyperref") {
		t.Error("hyperref devrait rester sur la liste des non-chargés")
	}
}
