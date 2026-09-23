package engine

import (
	"strings"
	"testing"
)

// A configuration command the engine does not implement must still CONSUME its
// arguments. Undefined, \DeclareCaptionLabelFormat{adja-page}{…} printed its own
// name and body on 2408.02845's first page — "adja-page", "#1 #2 (previous page)"
// and an \hrulefill drawn across the sheet — and pushed the title to page 2.
//
// The shapes below are each package's own (caption3.sty:725, kvoptions.sty:370 and
// 619, keyval.sty:81, titletoc.sty:188 and 203), not a guess: getting the argument
// count wrong leaves the REST on the page, which is the defect being fixed.
func TestConfigurationCommandsConsumeTheirArguments(t *testing.T) {
	for _, src := range []string{
		`\DeclareCaptionLabelFormat{adja-page}{\hrulefill\\#1 #2 (previous page)}`,
		`\DeclareVoidOption{draft}{\@gtxdraft}`,
		`\ProcessKeyvalOptions*`,
		`\ProcessKeyvalOptions{Gin}`,
		`\define@key{Gin}{width}{\def\Gin@ewidth{#1}}`,
		`\define@key{Gin}{scale}[1]{\def\Gin@scale{#1}}`,
		`\contentsmargin{0cm}`,
		`\contentsmargin[1em]{0cm}`,
		`\titlecontents{section}[2em]{\small}{\thecontentslabel}{}{\thecontentspage}[]`,
		`\titlecontents*{subsubsection}[2em]{\footnotesize}{}{}{}[ \textbullet ]`,
		`\defaultbibliography{main}`,
		`\defaultbibliographystyle{naturemag}`,
		`\phantomsection`,
		// titletoc's partial tables of contents (titletoc.sty:455, 465, 472, 497).
		`\startcontents[sections]`,
		`\startcontents`,
		`\stopcontents[sections]`,
		`\resumecontents[sections]`,
		`\printcontents[sections]{l}{1}{\setcounter{tocdepth}{2}}`,
		`\printcontents{l}{1}{}`,
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(src + `\message{[after]}`); err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got := trimNL(e.out.String()); got != "[after]" {
			t.Errorf("%s\n  left %q on the page, want nothing before [after]", src, got)
		}
	}
}

// The commands must also leave no TEXT behind — the message test above catches a
// stray \message, this one catches characters reaching the page.
func TestConfigurationCommandsTypesetNothing(t *testing.T) {
	src := `\documentclass{article}\begin{document}` +
		`\DeclareCaptionLabelFormat{adja-page}{\hrulefill #1 #2 (previous page)}` +
		`\contentsmargin{0cm}` +
		`\titlecontents{section}[2em]{\small}{\thecontentslabel}{}{\thecontentspage}[]` +
		`MARKER\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
	for _, bad := range []string{"adja-page", "previous", "section", "0cm"} {
		if strings.Contains(pages[0], ">"+bad) || strings.Contains(pages[0], bad+"<") {
			t.Errorf("%q reached the page", bad)
		}
	}
}
