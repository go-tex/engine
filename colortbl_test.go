package engine

import "testing"

// colortbl's background commands typeset nothing, but they must still EAT their
// arguments: undefined, they put the colour name on the page. Measured against
// tectonic — 166 \cellcolor and 77 \rowcolor over 9 of the 200 corpus papers.
//
// The signatures differ, and the difference is the reference's, not a guess:
// \rowcolor and \columncolor take the two overhang arguments, \cellcolor does
// not — tectonic SETS "[1pt][2pt]" after a \cellcolor, inside a tabular as well
// as outside. Eating them everywhere would swallow text the reference keeps.
func TestColorTableCommandsEatExactlyTheirArguments(t *testing.T) {
	const pre = `\documentclass{article}\usepackage[table]{xcolor}\begin{document}`
	for _, tc := range []struct{ name, src, want string }{
		{"cellcolor simple", `A\cellcolor{gray}B`, "AB"},
		{"cellcolor avec modèle", `A\cellcolor[rgb]{1,0,0}B`, "AB"},
		{"cellcolor NE mange PAS les débordements", `A\cellcolor{gray}[2pt]B`, "A[2pt]B"},
		{"rowcolor simple", `A\rowcolor{gray}B`, "AB"},
		{"rowcolor avec modèle", `A\rowcolor[rgb]{1,0,0}B`, "AB"},
		{"rowcolor mange un débordement", `A\rowcolor{gray}[2pt]B`, "AB"},
		{"rowcolor mange les deux", `A\rowcolor{gray}[2pt][3pt]B`, "AB"},
		{"columncolor mange les deux", `A\columncolor{gray}[2pt][3pt]B`, "AB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := compile([]byte(pre+tc.src+`\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			if got := pageChars(e); got != tc.want {
				t.Errorf("page = %q, want %q", got, tc.want)
			}
		})
	}
}
