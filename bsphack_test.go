package engine

import (
	"strings"
	"testing"
)

// \lastskip (tex.web §424) reads the glue at the end of the list being BUILT.
// Inside an \hbox that is the box's own list, not the paragraph around it — and
// the box's list is local to buildBoxList, so it has to be published or the
// primitive silently answers about the wrong list. It used to be a permanently
// zero \newskip, which is why nothing depending on it worked.
func TestLastSkipSeesTheBoxBeingBuilt(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"paragraphe", `word \the\lastskip`, "2.0ptplus1.0ptminus0.66667pt"},
		{"dans une hbox", `\hbox{word \the\lastskip}`, "2.0ptplus1.0ptminus0.66667pt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}\begin{document}`+tc.src+`\end{document}`),
				Options{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			got := pageChars(e)
			if !strings.Contains(got, tc.want) {
				t.Errorf("page = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// \@bsphack/\@esphack wrap a command that WRITES but typesets nothing (\label,
// \index). They make the space around the call survive as exactly one instead of
// two. Witness against tectonic: all three \hbox widths are equal on the
// reference (46.17pt); ours was 2.00pt — one interword space — wider on the
// spaced form. go-tex/engine#385.
func TestLabelEatsTheSpaceTheReferenceEats(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\setbox0\hbox{word word}\setbox1\hbox{word \label{x} word}`+
		`\setbox2\hbox{word\label{y} word}`+
		`\the\wd0|\the\wd1|\the\wd2\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	parts := strings.Split(got, "|")
	if len(parts) != 3 {
		t.Fatalf("page = %q, want three widths separated by |", got)
	}
	if parts[0] != parts[1] || parts[0] != parts[2] {
		t.Errorf("widths %q / %q / %q differ; the reference makes all three equal",
			parts[0], parts[1], parts[2])
	}
}
