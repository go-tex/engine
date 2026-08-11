package engine

import (
	"strings"
	"testing"
)

// runMsg loads LaTeX, runs src and returns the accumulated \message output with a
// trailing newline trimmed. It fails the test on a run error.
func runMsg(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	got, err := e.Run(src)
	if err != nil {
		t.Fatal(err)
	}
	return trimNL(got)
}

// counterValue reads the raw \count value behind \c@<name>, or -1 if undefined.
func counterValue(e *Engine, name string) int {
	if m := e.eq["c@"+name]; m != nil && m.kind == mCountRef {
		return e.count[m.code]
	}
	return -1
}

// \newcounter allocates a counter; \stepcounter advances it; \arabic prints it.
func TestNewcounterStepArabic(t *testing.T) {
	got := runMsg(t, `\newcounter{foo}\stepcounter{foo}\stepcounter{foo}\message{\arabic{foo}}`)
	if got != "2" {
		t.Errorf("\\arabic{foo} = %q, want \"2\"", got)
	}
}

// \setcounter sets an absolute value; \addtocounter adds (possibly negative).
func TestSetAndAddToCounter(t *testing.T) {
	got := runMsg(t, `\newcounter{c}\setcounter{c}{5}\message{\arabic{c}}`+
		`\addtocounter{c}{3}\message{\arabic{c}}`+
		`\addtocounter{c}{-2}\message{\arabic{c}}`)
	if got != "5 8 6" {
		t.Errorf("set/add sequence = %q, want \"5 8 6\"", got)
	}
}

// \value{y} is usable as a <number>: inside \setcounter and inside \ifnum.
func TestValueAsNumber(t *testing.T) {
	got := runMsg(t, `\newcounter{x}\newcounter{y}\setcounter{y}{7}`+
		`\setcounter{x}{\value{y}}\message{\arabic{x}}`+
		`\ifnum\value{x}>0 \message{pos}\else\message{nonpos}\fi`+
		`\ifnum\value{x}>10 \message{big}\else\message{small}\fi`)
	if got != "7 pos small" {
		t.Errorf("value-as-number = %q, want \"7 pos small\"", got)
	}
}

// Each formatting command renders a known value in its own alphabet.
func TestCounterFormats(t *testing.T) {
	cases := []struct{ cmd, want string }{
		{"arabic", "4"},
		{"roman", "iv"},
		{"Roman", "IV"},
		{"alph", "d"},
		{"Alph", "D"},
	}
	for _, c := range cases {
		got := runMsg(t, `\newcounter{n}\setcounter{n}{4}\message{\`+c.cmd+`{n}}`)
		if got != c.want {
			t.Errorf("\\%s{n} at 4 = %q, want %q", c.cmd, got, c.want)
		}
	}
}

// \fnsymbol maps 1..9 onto the nine footnote symbols.
func TestFnsymbol(t *testing.T) {
	want := []string{"*", "†", "‡", "§", "¶", "‖", "**", "††", "‡‡"}
	for i, w := range want {
		src := `\newcounter{s}\setcounter{s}{` + itoa(i+1) + `}\message{\fnsymbol{s}}`
		if got := runMsg(t, src); got != w {
			t.Errorf("\\fnsymbol at %d = %q, want %q", i+1, got, w)
		}
	}
}

// \refstepcounter freezes \the<counter> into \@currentlabel, so \label captures it
// and \ref resolves to that number.
func TestRefstepcounterLabelRef(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\newcounter{foo}\stepcounter{foo}\stepcounter{foo}\stepcounter{foo}` +
		`\refstepcounter{foo}\label{k}\noindent\ref{k}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if e.labels["k"] != "4" {
		t.Errorf("label k = %q, want \"4\"", e.labels["k"])
	}
	if e.refText("k") != "4" {
		t.Errorf("refText(k) = %q, want \"4\"", e.refText("k"))
	}
	if got := mvlText(e.mvl); got != "4" {
		t.Errorf("\\ref{k} typeset %q, want \"4\"", got)
	}
}

// \newcounter{sub}[section] resets sub whenever \section steps \c@section.
func TestNewcounterWithinSection(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass{article}\begin{document}` +
		`\newcounter{sub}[section]` +
		`\section{A}\stepcounter{sub}\stepcounter{sub}` + // sub=2 in section 1
		`\section{B}` + // \section resets sub to 0
		`\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if v := counterValue(e, "sub"); v != 0 {
		t.Errorf("sub after second \\section = %d, want 0", v)
	}
	if v := counterValue(e, "section"); v != 2 {
		t.Errorf("section = %d, want 2", v)
	}
}

// A counter numbered within a plain (non-sectioning) counter resets when the
// parent is advanced with \stepcounter, exercising the \cl@<parent> reset list.
func TestNewcounterWithinPlain(t *testing.T) {
	got := runMsg(t, `\newcounter{par}\newcounter{kid}[par]`+
		`\stepcounter{kid}\stepcounter{kid}\message{\arabic{kid}}`+ // kid=2
		`\stepcounter{par}\message{\arabic{kid}}`+ // parent step resets kid to 0
		`\stepcounter{kid}\message{\arabic{kid}}`) // kid=1
	if got != "2 0 1" {
		t.Errorf("within-plain reset = %q, want \"2 0 1\"", got)
	}
}

// Existing counters interoperate: \setcounter{section}{3} then \section yields 4.
func TestInteropSetExistingCounter(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass{article}\begin{document}` +
		`\setcounter{section}{3}\section{X}\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if v := counterValue(e, "section"); v != 4 {
		t.Errorf("section after set 3 + \\section = %d, want 4", v)
	}
	// The rendered heading shows "4".
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "4") {
		t.Errorf("heading did not typeset section number 4; got %q", b.String())
	}
}

// \setcounter{equation}{0} resets equation numbering, so the next equation is (1).
func TestInteropResetEquation(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\hsize=300pt\begin{equation} a \end{equation}` +
		`\setcounter{equation}{0}\begin{equation} b \end{equation}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	// After the reset the second equation is (1) again; only "(1)" appears (twice).
	if strings.Contains(text, "(2)") {
		t.Errorf("equation counter was not reset; got %q (unexpected (2))", text)
	}
	if !strings.Contains(text, "(1)") {
		t.Errorf("equation number (1) missing after reset; got %q", text)
	}
}

// Error branches must not panic: unknown counter names and malformed arguments.
func TestCounterErrorBranches(t *testing.T) {
	srcs := []string{
		`\setcounter{missing}{3}`,          // unknown counter, well-formed value
		`\addtocounter{missing}{1}`,        // unknown counter
		`\stepcounter{missing}`,            // unknown counter (no reset list)
		`\refstepcounter{missing}`,         // unknown counter (no \the<name>)
		`\message{\arabic{missing}}`,       // format an undefined counter -> "0"
		`\message{\value{missing}}`,        // \value of an undefined counter
		`\newcounter{}`,                    // empty name
		`\newcounter{z}\setcounter{z}5`,    // malformed value: no braces (token pushed back)
		`\newcounter{q}\setcounter{q}`,     // value group missing entirely (input ends)
		`\newcounter{r}\setcounter{r}{5x}`, // trailing non-brace after the number
		`\newcounter{w}[nope]\section{}`,   // reset within a non-existent parent
	}
	for _, s := range srcs {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{}) // so any stray leftover token typesets harmlessly
		if _, err := e.Run(s); err != nil {
			// A controlled SourceError is acceptable for genuinely malformed input;
			// the point of this test is that nothing panics.
			continue
		}
	}
}

// Exhausting the \count registers must not panic; \newcounter past the limit
// simply allocates no register (its \arabic then reads as 0).
func TestNewcounterExhaustion(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300; i++ {
		if _, err := e.Run(`\newcounter{ctr` + itoa(i) + `}`); err != nil {
			t.Fatal(err)
		}
	}
	// The \count file has 256 registers, allocated from index 10, so at most 246
	// counters get one; the rest allocate none (their \c@ alias stays undefined).
	if v := counterValue(e, "ctr0"); v != 0 {
		t.Errorf("first counter register = %d, want 0 (allocated)", v)
	}
	if v := counterValue(e, "ctr299"); v != -1 {
		t.Errorf("over-limit counter = %d, want -1 (no register)", v)
	}
	// Stepping an unallocated counter is a harmless no-op (must not panic).
	if _, err := e.Run(`\stepcounter{ctr299}\setcounter{ctr299}{5}`); err != nil {
		t.Fatal(err)
	}
}

// Redefining an existing counter with \newcounter reuses its register.
func TestNewcounterRedefineReuses(t *testing.T) {
	got := runMsg(t, `\newcounter{d}\setcounter{d}{9}\newcounter{d}\message{\arabic{d}}`)
	// The second \newcounter must not allocate a new register nor clear the value.
	if got != "9" {
		t.Errorf("redefined counter value = %q, want \"9\"", got)
	}
}

// itoa is a tiny non-negative int formatter (avoids importing strconv here).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
