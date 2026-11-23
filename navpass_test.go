// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \newwrite hands out stream NUMBERS, as \chardef's (latex.ltx:358). It used to
// allocate a count register, whose value — 0 — is what \openout and \write read,
// so every stream was stream 0 and each \openout replaced the one before.
func TestNewwriteAllocatesDistinctStreams(t *testing.T) {
	got := ckRun(t, `\newwrite\zza\newwrite\zzb\message{[\meaning\zza][\meaning\zzb]}`)
	parts := strings.SplitAfter(got, "]")
	if len(parts) < 2 || !strings.HasPrefix(parts[0], `[\char"`) || !strings.HasPrefix(parts[1], `[\char"`) {
		t.Fatalf("meanings = %q, want two \\char\"N stream numbers", got)
	}
	if parts[0] == parts[1] {
		t.Errorf("both streams are %s: they collide", parts[0])
	}
}

// Two streams open at once keep their own text: \@auxout is open while beamer
// opens \tf@nav, \tf@toc and \tf@snm (beamerbasemisc.sty:176-181).
func TestTwoOpenStreamsKeepTheirOwnText(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\newwrite\zza\newwrite\zzb` +
		`\immediate\openout\zza=zza.txt \immediate\openout\zzb=zzb.txt ` +
		`\immediate\write\zza{alpha}\immediate\write\zzb{beta}` +
		`\immediate\closeout\zza\immediate\closeout\zzb`); err != nil {
		t.Fatal(err)
	}
	if got := e.writtenFile["zza.txt"]; got != "alpha\n" {
		t.Errorf("zza.txt = %q, want %q", got, "alpha\n")
	}
	if got := e.writtenFile["zzb.txt"]; got != "beta\n" {
		t.Errorf("zzb.txt = %q, want %q", got, "beta\n")
	}
}

// \par in an \hbox does nothing (tex.web:21178-21185: end_graf acts only in
// unrestricted horizontal mode). It used to end the ENCLOSING paragraph, so the two
// 60pt boxes below went on two lines of a 100pt measure — 27.2pt tall where
// tectonic measures 0pt — and beamer's \color@endgroup (\endgraf\endgroup) did the
// same to a split headline's two colour boxes (go-tex/engine#926).
func TestParInAnHboxDoesNotEndTheParagraph(t *testing.T) {
	got := ckRun(t, `\setbox1\vbox{\hsize=100pt \leavevmode\hbox to 60pt{\par}\hbox to 60pt{\par}}\message{[\the\ht1]}`)
	if got != "[0.0pt]" {
		t.Errorf("height = %s, want [0.0pt] (one line of two empty boxes)", got)
	}
}

// A \vbox inside an \hbox is vertical mode again: \par there ends ITS paragraph.
func TestParInAVboxInsideAnHboxStillEndsItsParagraph(t *testing.T) {
	const r = `\noindent\vrule width5pt height5pt depth0pt `
	one := ckRun(t, `\setbox2\hbox{\vbox{\hsize=200pt `+r+r+`}}\message{[\the\ht2]}`)
	two := ckRun(t, `\setbox2\hbox{\vbox{\hsize=200pt `+r+`\par`+r+`}}\message{[\the\ht2]}`)
	if one != "[5.0pt]" {
		t.Fatalf("one line measured %s, want [5.0pt]", one)
	}
	if one == two {
		t.Errorf("rule\\par rule measured %s, the same as one line: the \\par did nothing", two)
	}
}

// \colorbox pads its content by \fboxsep as it stands (color.sty:163-167). beamer
// sets it to zero for every coloured box (beamerbasecolor.sty:223); the constant
// 3pt made each beamercolorbox 6pt wider and 6pt taller than asked.
func TestColorboxReadsFboxsep(t *testing.T) {
	for _, c := range []struct{ sep, want string }{
		{`0pt`, "[10.0pt/10.0pt]"},
		{`2pt`, "[14.0pt/12.0pt]"},
	} {
		got := ckRun(t, `\fboxsep=`+c.sep+`\relax\setbox1\hbox{\colorbox{red}{\vrule width10pt height10pt depth0pt}}\message{[\the\wd1/\the\ht1]}`)
		if got != c.want {
			t.Errorf("\\fboxsep=%s: \\colorbox = %s, want %s", c.sep, got, c.want)
		}
	}
}

// navClass does, in miniature, what beamer does with its .nav file: read the
// previous run's at \begin{document} (beamerbasemisc.sty:87-92) and open \tf@nav
// at \end{document} (:176-177), so that LaTeX's second reading of the .aux writes
// the \addtocontents{nav} lines into it (latex.ltx:10879-10886, :10941-10949).
const navClass = `\newdimen\navseen
\AtBeginDocument{\@input{\jobname.nav}}
\AtEndDocument{\newwrite\tf@nav\immediate\openout\tf@nav\jobname.nav\relax}
`

// The document's \addtocontents{nav} reaches the NEXT run through the .nav, and
// compile runs that second pass: beamer's navigation bars are sized from it, and
// without it a split headline measured no sections at all (go-tex/engine#926).
func TestANavFileReachesTheNextPass(t *testing.T) {
	src := `\documentclass{zznav}\begin{document}\addtocontents{nav}{\global\navseen=2pt}x\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Resolve: func(name string) ([]byte, bool) {
		if name == "zznav.cls" {
			return []byte(navClass), true
		}
		return nil, false
	}})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := e.namedDimen("navseen")
	if !ok {
		t.Fatal(`\navseen is not defined: the class did not load`)
	}
	if got != 2*unity {
		t.Errorf(`\navseen = %.2fpt after compile, want 2pt read back from the .nav`, spToPt(got))
	}
}

// A document that writes no .nav is compiled once, as before.
func TestNoNavNoSecondPass(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.wroteNav() {
		t.Error("an article wrote a .nav")
	}
}
