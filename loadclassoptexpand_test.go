package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ A \LoadClass option that IS a macro must reach the loaded class EXPANDED.
//
// acmart.cls:258 does not write its body size literally:
//
//	\LoadClass[\ACM@fontsize, reqno]{amsart}
//
// with \ACM@fontsize set to 9pt for manuscript/acmtog/sigconf/siggraph/sigchi. The
// engine resolved that macro for its OWN decision (resolveSizeOption, which sets the
// font scale) and handed the list to the .cls unexpanded — so amsart matched no
// \DeclareOption, kept its 10pt \@typesizes, and the body of every acmart paper was led
// at 12pt where the reference leads it at 11pt. The same expansion applied at one of two
// sites, and the site that was right made the other one hard to see: \@ptsize and the
// font scale both read 9pt.
//
// Measured on 2304.01951 (sigconf), the interline step on pages 2–9 through poppler on
// both sides: 11.96pt ours against 10.96pt the reference's, and 16 pages against 10.
// After this, 10.96 against 10.96 on every ordinary page.
func TestLoadClassExpandsAMacroOptionForTheLoadedClass(t *testing.T) {
	// The witness is the base class's OWN \DeclareOption firing, which is what was
	// missing: the engine's parallel path was already working.
	withTempDir(t, map[string]string{
		"sizebase.cls": `\DeclareOption{9pt}{\message{BASE-SAW-9PT}}` +
			`\DeclareOption{10pt}{\message{BASE-SAW-10PT}}` +
			`\ProcessOptions\relax`,
		"viamacro.cls": `\def\mysize{9pt}\LoadClass[\mysize]{sizebase}`,
	}, func() {
		out, _ := runLaTeX(t, `\documentclass{viamacro}`)
		if !strings.Contains(out, "BASE-SAW-9PT") {
			t.Errorf("la classe chargée n'a pas vu l'option développée: %q", out)
		}
		if strings.Contains(out, "BASE-SAW-10PT") {
			t.Errorf("la classe chargée a pris son option par défaut: %q", out)
		}
	})
}

// ⭐ The control: a LITERAL option must keep working exactly as before. It already did —
// that is why the defect survived — so this is the half that proves the fix did not
// trade one path for the other.
func TestLoadClassStillPassesALiteralOption(t *testing.T) {
	withTempDir(t, map[string]string{
		"sizebase.cls": `\DeclareOption{9pt}{\message{BASE-SAW-9PT}}\ProcessOptions\relax`,
		"literal.cls":  `\LoadClass[9pt]{sizebase}`,
	}, func() {
		out, _ := runLaTeX(t, `\documentclass{literal}`)
		if !strings.Contains(out, "BASE-SAW-9PT") {
			t.Errorf("l'option littérale a cessé d'arriver: %q", out)
		}
	})
}

// An option whose control sequence has NO macro behind it is handed through untouched: a
// .cls that declares an option literally named like a control sequence still receives it,
// and an undefined name must not become an empty option that matches \DeclareOption{}.
func TestLoadClassPassesAnUnresolvableOptionThrough(t *testing.T) {
	withTempDir(t, map[string]string{
		"anybase.cls": `\DeclareOption*{\message{STAR-SAW:\CurrentOption:}}\ProcessOptions\relax`,
		"ghost.cls":   `\LoadClass[\notamacro]{anybase}`,
	}, func() {
		out, _ := runLaTeX(t, `\documentclass{ghost}`)
		if !strings.Contains(out, `STAR-SAW:\notamacro:`) {
			t.Errorf("une séquence sans macro derrière doit passer telle quelle: %q", out)
		}
	})
}

// And the leading itself, end to end: the engine's \baselineskip in the body must be the
// base class's, not the default. This is the number the corpus measurement is about.
func TestAMacroSizeOptionSetsTheBodyLeading(t *testing.T) {
	withTempDir(t, map[string]string{
		// A minimal stand-in for amsart's \@typesizes: the option sets the leading
		// through \@setfontsize, exactly as a real size .clo does.
		"ldbase.cls": `\DeclareOption{9pt}{\def\gotexBLS{11}}` +
			`\DeclareOption{10pt}{\def\gotexBLS{12}}` +
			`\ExecuteOptions{10pt}\ProcessOptions\relax` +
			`\makeatletter\@setfontsize\normalsize{9}{\gotexBLS}\makeatother`,
		"ldmacro.cls": `\def\mysize{9pt}\LoadClass[\mysize]{ldbase}`,
	}, func() {
		out, _ := runLaTeX(t, `\documentclass{ldmacro}\begin{document}`+
			`\message{BLS=\the\baselineskip}\end{document}`)
		if !strings.Contains(out, "BLS=11.0pt") {
			t.Errorf("interligne du corps: %q, want 11.0pt (la table 9pt de la classe de base)", out)
		}
	})
}
