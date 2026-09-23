package engine

import (
	"strings"
	"testing"
)

// \ifthenelse was undefined, and an undefined command in the document keeps its
// arguments — so the test SOURCE and BOTH branches were typeset. Each case here
// was checked against tectonic, which produces the "want" column.
func TestIfthenelseTakesOneBranch(t *testing.T) {
	const pre = `\documentclass{article}\usepackage{ifthen}` +
		`\newcounter{n}\setcounter{n}{3}` +
		`\newboolean{flag}\setboolean{flag}{true}\newboolean{off}` +
		`\begin{document}`
	for _, tc := range []struct{ name, src, want string }{
		{"equal, vrai", `\ifthenelse{\equal{x}{x}}{ALPHA}{BETA}`, "ALPHA"},
		{"equal, faux", `\ifthenelse{\equal{x}{y}}{GAMMA}{DELTA}`, "DELTA"},
		{"nombre >", `\ifthenelse{\value{n} > 2}{EPSILON}{ZETA}`, "EPSILON"},
		{"nombre =", `\ifthenelse{\value{n} = 9}{ETA}{THETA}`, "THETA"},
		{"booléen vrai", `\ifthenelse{\boolean{flag}}{IOTA}{KAPPA}`, "IOTA"},
		{"booléen faux", `\ifthenelse{\boolean{off}}{LAMBDA}{MU}`, "MU"},
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

// The test's own source must never reach the page: \equal{x}{x} used to set "xx"
// and \value{n} > 2 used to set "> 2".
func TestIfthenelseDoesNotTypesetItsTest(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{ifthen}`+
		`\newcounter{n}\setcounter{n}{3}\begin{document}`+
		`A\ifthenelse{\equal{zz}{zz}}{B}{C}\ifthenelse{\value{n} > 2}{D}{E}F`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); got != "ABDF" {
		t.Errorf("page = %q, want ABDF", got)
	}
}

// A test shape the engine does not evaluate must SAY so rather than guess in
// silence: the census entry is what tells anyone the answer may be wrong.
func TestIfthenelseNamesATestItCannotEvaluate(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{ifthen}\begin{document}`+
		`\ifthenelse{\isodd{3}}{ODD}{EVEN}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	var found string
	for k := range e.skippedCS {
		if strings.Contains(k, "not evaluated") {
			found = k
		}
	}
	if found == "" {
		t.Fatal("an unevaluated test was not reported at all")
	}
	if !strings.Contains(found, `\isodd`) {
		t.Errorf("the report says %q, which does not name the test", found)
	}
}
