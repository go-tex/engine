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
		// ⛔ e.getCount, not e.count[m.code]. The pools are grown on write
		// (registers.go), so a counter allocated high and never assigned has a
		// legal index with nothing stored at it: the raw index panicked with
		// "index out of range [436] with length 256" the moment the pool went
		// past TeX82's 256, on a counter whose value is simply 0.
		return e.getCount(m.code)
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
//
// ⛔ THE CEILING IS registerPoolSize, NOT 300 ALLOCATIONS. This test used to
// declare 300 counters and assert that ctr299 got none, which was true only
// while the pool was TeX82's 256 — the number was written into the test, so
// raising the pool turned a passing test into a failing one without anything
// about counters changing. The allocator is driven to the brink instead, which
// asks the same question at whatever the pool is.
func TestNewcounterExhaustion(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	// ⭐ THE CONTROL THAT ONLY PASSES IF THE POOL REALLY GREW. 300 counters is
	// past TeX82's ceiling and well inside eTeX's, so every one of them must get
	// a register. Without this row a pool still stuck at 256 would pass the
	// exhaustion rows below exactly as a raised one does.
	for i := 0; i < 300; i++ {
		if _, err := e.Run(`\newcounter{ctr` + itoa(i) + `}`); err != nil {
			t.Fatal(err)
		}
	}
	if v := counterValue(e, "ctr299"); v != 0 {
		t.Errorf("counter 300 register = %d, want 0 (allocated) — the pool is %d, so 300 counters fit", v, registerPoolSize)
	}

	// At the brink: the next allocation is the last one the pool can satisfy.
	e.allocCnt = registerPoolSize - 1
	if _, err := e.Run(`\newcounter{last}\newcounter{over}`); err != nil {
		t.Fatal(err)
	}
	if v := counterValue(e, "last"); v != 0 {
		t.Errorf("last counter in the pool = %d, want 0 (allocated)", v)
	}
	if v := counterValue(e, "over"); v != -1 {
		t.Errorf("over-limit counter = %d, want -1 (no register)", v)
	}
	if e.Diagnostics().RegistersExhausted[`\count`] == 0 {
		t.Error("the refused allocation was not reported (#830)")
	}
	// Stepping an unallocated counter is a harmless no-op (must not panic).
	if _, err := e.Run(`\stepcounter{over}\setcounter{over}{5}`); err != nil {
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
