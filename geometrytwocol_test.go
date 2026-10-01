package engine

import "testing"

// hsizeAfter compiles a preamble and returns the measure paragraphs are set at.
func hsizeAfter(t *testing.T, preamble string) int {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := preamble + `\begin{document}` +
		`Un paragraphe assez long pour que la mesure de colonne soit prise.\par` +
		`\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run: %v", err)
	}
	return e.hsize
}

// geometry must not undo a [twocolumn] class option.
//
// \documentclass[twocolumn] halves the measure as soon as it is seen, saving the
// one-column width to halve from. A geometry loaded AFTER it assigns the new text
// width straight to \hsize — the full width, correctly, since that is what
// geometry computes — and the halving was simply gone, twoColApplied still set so
// nothing took it again.
//
// The kernel \let's \linewidth and \columnwidth to \hsize, so everything measured
// from them was then twice too wide. arXiv 2311.15028 draws nine
// \includegraphics[width=\linewidth] at 542pt into 262pt columns: they overlap
// each other and bury five of its figure legends.
func TestGeometryKeepsTheTwoColumnMeasure(t *testing.T) {
	const twocol = `\documentclass[twocolumn]{article}`
	const onecol = `\documentclass{article}`
	const geom = `\usepackage[margin=0.5in]{geometry}`

	full := hsizeAfter(t, onecol+geom)
	col := hsizeAfter(t, twocol+geom)

	// LaTeX: \columnwidth = (\textwidth - \columnsep)/2, \columnsep = 10pt.
	want := (full - 10*unity) / 2
	if col != want {
		t.Errorf("two-column measure with geometry = %d sp (%.2fpt), want %d sp (%.2fpt)",
			col, float64(col)/float64(unity), want, float64(want)/float64(unity))
	}
	// And it really is narrower than the page — the failure was it being the full width.
	if col >= full {
		t.Errorf("the column measure (%d) is not narrower than the text width (%d)", col, full)
	}
}

// Without the class option, geometry still sets the full width: the retake must
// not fire on a one-column document.
func TestGeometryLeavesAOneColumnDocumentAtFullWidth(t *testing.T) {
	const geom = `\usepackage[margin=0.5in]{geometry}`
	with := hsizeAfter(t, `\documentclass{article}`+geom)
	// 612pt paper less two 36pt margins.
	if want := 540 * unity; with < want-unity || with > want+3*unity {
		t.Errorf("one-column measure = %.2fpt, want about 540pt", float64(with)/float64(unity))
	}
}
