package engine

import (
	"strings"
	"testing"
)

// A biber .bbl is DATA, not a ready thebibliography block: \entry/\name/\field.
const blSample = `% $ biblatex auxiliary file $
% $ biblatex bbl format version 3.2 $
\refsection{0}
  \datalist[entry]{nty/global//global/global}
    \entry{zhang2018}{article}{}
      \name{author}{2}{}{%
        {{hash=ZJ}{%
           family={Zhang},
           familyi={Z\bibinitperiod},
           given={J.},
           giveni={J\bibinitperiod},
        }}%
        {{hash=SSS}{%
           family={Sp{\"a}th},
           familyi={S\bibinitperiod},
           given={S.\bibnamedelima S.},
           giveni={S\bibinitperiod\bibinitdelim S\bibinitperiod},
        }}%
      }
      \list{publisher}{1}{%
        {Oxford}%
      }
      \field{title}{Characterization of cancer genomic heterogeneity}
      \field{journaltitle}{Precision Clinical Medicine}
      \field{volume}{1}
      \field{pages}{29\bibrangedash 48}
      \field{date}{2018-03}
    \endentry
  \enddatalist
\endrefsection
`

// The delimiter macros biber writes INSIDE its data are spacing instructions, not
// document commands, and must never reach the page.
//
// This is the test the work needed first: the cleaner looked right and did
// nothing, because the pattern had been written through a layer that turned its
// \b into a BACKSPACE byte. Nothing in the rendered page said so — the delimiters
// are undefined, so lenient mode skipped them and the names read correctly — and
// only the undefined-command tally showed 50 of them.
func TestBiblatexNameDelimitersAreNotCommands(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`S.\bibnamedelima S.`, "S. S."},
		{`S\bibinitperiod`, "S."},
		{`29\bibrangedash 48`, "29--48"},
		{`A\bibinitperiod\bibinitdelim B\bibinitperiod`, "A. B."},
		{`plain text`, "plain text"},
	} {
		if got := blClean(c.in); got != c.want {
			t.Errorf("blClean(%q) = %q, want %q", c.in, got, c.want)
		}
		if strings.Contains(blClean(c.in), `\bib`) {
			t.Errorf("blClean(%q) left a biber macro in %q", c.in, blClean(c.in))
		}
	}
}

// The entry's fields reach the formatter under the names it reads.
func TestParseBiblatexBBL(t *testing.T) {
	ents := parseBiblatexBBL(blSample)
	if len(ents) != 1 {
		t.Fatalf("got %d entries, want 1", len(ents))
	}
	e := ents[0]
	for _, c := range []struct{ field, want string }{
		{"author", `Zhang, J. and Sp{\"a}th, S. S.`},
		{"title", "Characterization of cancer genomic heterogeneity"},
		{"journal", "Precision Clinical Medicine"},
		{"volume", "1"},
		{"pages", "29--48"},
		{"year", "2018"}, // from \field{date}{2018-03}
		{"publisher", "Oxford"},
	} {
		if got := e.field(c.field); got != c.want {
			t.Errorf("%s = %q, want %q", c.field, got, c.want)
		}
	}
	if e.typ != "article" || e.key != "zhang2018" {
		t.Errorf("entry is %q/%q, want article/zhang2018", e.typ, e.key)
	}
}

// A .bbl that BibTeX wrote is NOT biber data and must go down the splice path —
// \printbibliography in such a document is \bibliography by another name.
func TestABibTeXBBLIsNotTakenForBiberData(t *testing.T) {
	if isBiblatexBBL(`\begin{thebibliography}{9}\bibitem{a}X\end{thebibliography}`) {
		t.Error("a BibTeX .bbl was taken for biber data")
	}
	if !isBiblatexBBL(blSample) {
		t.Error("biber data was not recognised")
	}
}
