package engine

import (
	"strings"
	"testing"
)

// acl.sty (2311.01230) queues its layout with
//
//	\PassOptionsToPackage{a4paper,margin=2.5cm,heightrounded=true}{geometry}
//	\RequirePackage{geometry}
//
// and the load must pick those options up. It did not: geometry ran with an
// empty option list, so since the default body (#923) the page was letter with
// a 0.7-body block, and before it the class's own block — either way the
// paper broke into 19 or 16 pages where the reference has 17.
//
// Values are tectonic 0.17.0's (--print, \typeout) for the same source. The
// text height is left out: heightrounded is not implemented (704.60031pt there,
// 702.78308pt here), a separate gap.
func TestGeometryTakesOptionsPassedToIt(t *testing.T) {
	src := `\documentclass[11pt]{article}\makeatletter` +
		`\PassOptionsToPackage{a4paper,margin=2.5cm,heightrounded=true}{geometry}` +
		`\RequirePackage{geometry}\makeatother` +
		`\begin{document}TW=\the\textwidth\ OS=\the\oddsidemargin\ ` +
		`TM=\the\topmargin\ PH=\the\paperheight\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, p := range e.Pages() {
		b.WriteString(mvlText(p.list))
	}
	// The text layer joins the control words with no space.
	want := "TW=455.24411ptOS=-1.1381ptTM=-38.1381ptPH=845.04684pt"
	if got := b.String(); !strings.Contains(got, want) {
		t.Errorf("want %q (tectonic), got %q", want, got)
	}
}
