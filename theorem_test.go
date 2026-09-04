package engine

import (
	"strings"
	"testing"
)

// hasRuleNode reports whether a filled rectangle (the QED box) was placed
// anywhere in the node tree.
func hasRuleNode(nodes []node) bool {
	for _, n := range nodes {
		switch v := n.(type) {
		case ruleNode:
			return true
		case *boxNode:
			if hasRuleNode(v.list) {
				return true
			}
		case frameNode:
			if v.inner != nil && hasRuleNode(v.inner.list) {
				return true
			}
		}
	}
	return false
}

// treeText reads back every typeset character in the main vertical list.
func treeText(e *Engine) string {
	var b strings.Builder
	collectChars(e.mvl, &b)
	return b.String()
}

// Two theorems of the same environment number 1 then 2, each freezing its number
// into \@currentlabel so \label captures it.
func TestTheoremNumbering(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\newtheorem{theorem}{Theorem}
\begin{theorem}\label{t:a} First.\end{theorem}
\begin{theorem}\label{t:b} Second.\end{theorem}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["t:a"]; got != "1" {
		t.Errorf("t:a label = %q, want \"1\"", got)
	}
	if got := e.labels["t:b"]; got != "2" {
		t.Errorf("t:b label = %q, want \"2\"", got)
	}
	if txt := treeText(e); !strings.Contains(txt, "Theorem") {
		t.Errorf("heading text not typeset; got %q", txt)
	}
}

// \newtheorem{env}{Heading}[within] numbers within the section counter: after
// \section, the first theorem is 1.1.
func TestTheoremWithin(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\newtheorem{thm}{Theorem}[section]
\section{One}
\begin{thm}\label{t:1} A.\end{thm}
\begin{thm}\label{t:2} B.\end{thm}
\section{Two}
\begin{thm}\label{t:3} C.\end{thm}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["t:1"]; got != "1.1" {
		t.Errorf("t:1 label = %q, want \"1.1\"", got)
	}
	if got := e.labels["t:2"]; got != "1.2" {
		t.Errorf("t:2 label = %q, want \"1.2\"", got)
	}
	if got := e.labels["t:3"]; got != "2.1" {
		t.Errorf("t:3 label = %q, want \"2.1\" (counter must reset on \\section)", got)
	}
}

// \newtheorem{env}[shared]{Heading} shares another environment's counter: a
// theorem then a lemma number 1 then 2 off the same counter.
func TestTheoremSharedCounter(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\newtheorem{theorem}{Theorem}
\newtheorem{lemma}[theorem]{Lemma}
\begin{theorem}\label{s:t} X.\end{theorem}
\begin{lemma}\label{s:l} Y.\end{lemma}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["s:t"]; got != "1" {
		t.Errorf("theorem label = %q, want \"1\"", got)
	}
	if got := e.labels["s:l"]; got != "2" {
		t.Errorf("lemma label = %q, want \"2\" (shared counter)", got)
	}
}

// \ref resolves a theorem's number through the label table.
func TestTheoremRef(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\newtheorem{theorem}{Theorem}
\begin{theorem}\label{r:a} Body.\end{theorem}
see \ref{r:a}.`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.refText("r:a"); got != "1" {
		t.Fatalf("refText(r:a) = %q, want \"1\"", got)
	}
	if txt := treeText(e); !strings.Contains(txt, "see1.") { // interword spaces are glue, not chars
		t.Errorf("\\ref did not typeset the number; got %q", txt)
	}
}

// The optional note of \begin{theorem}[note] is typeset in the heading.
func TestTheoremNote(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\newtheorem{theorem}{Theorem}
\begin{theorem}[Pythagoras]\label{n:a} Body.\end{theorem}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["n:a"]; got != "1" {
		t.Errorf("label = %q, want \"1\"", got)
	}
	if txt := treeText(e); !strings.Contains(txt, "Pythagoras") {
		t.Errorf("optional note not typeset; got %q", txt)
	}
}

// The proof environment prints a "Proof." head and a QED box.
func TestProofQED(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{proof} Trivial.\end{proof}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if txt := treeText(e); !strings.Contains(txt, "Proof.") {
		t.Errorf("proof head not typeset; got %q", txt)
	}
	if !hasRuleNode(e.mvl) {
		t.Error("no QED box (ruleNode) placed by \\end{proof}")
	}
}

// \begin{proof}[Proof of X] overrides the head text.
func TestProofOptionalHead(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{proof}[Proof of the claim] Body.\end{proof}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := treeText(e)
	if !strings.Contains(txt, "Proofoftheclaim.") { // spaces are glue; letters concatenate
		t.Errorf("custom proof head not typeset; got %q", txt)
	}
}

// \newtheorem with an empty environment name is ignored (error branch).
func TestNewtheoremEmptyName(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run("\\newtheorem{}{Nothing}"); err != nil {
		t.Fatal(err)
	}
	if e.eq["the"] != nil && e.eq["the"].kind == mMacro {
		t.Error("empty-name \\newtheorem should define nothing")
	}
}

// digitToks renders integers as their decimal digit tokens (incl. zero and
// multi-digit values); stringToToks maps letters/spaces/other to the right cats.
func TestTheoremTokenHelpers(t *testing.T) {
	for _, c := range []struct {
		n    int
		want string
	}{{0, "0"}, {7, "7"}, {42, "42"}, {105, "105"}} {
		var b strings.Builder
		for _, tk := range digitToks(c.n) {
			b.WriteRune(tk.ch)
		}
		if b.String() != c.want {
			t.Errorf("digitToks(%d) = %q, want %q", c.n, b.String(), c.want)
		}
	}
	toks := stringToToks("Ab .")
	if len(toks) != 4 {
		t.Fatalf("stringToToks len = %d, want 4", len(toks))
	}
	if toks[0].cat != catLetter || toks[1].cat != catLetter {
		t.Errorf("letters not catLetter: %+v", toks[:2])
	}
	if toks[2].cat != catSpace {
		t.Errorf("space not catSpace: %+v", toks[2])
	}
	if toks[3].cat != catOther {
		t.Errorf("'.' not catOther: %+v", toks[3])
	}
}

// Two environments both numbered within=section hook \@nsection only once (the
// second hookReset sees it already wrapped and returns), and both reset on a
// section step.
func TestTheoremWithinTwiceHooks(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\newtheorem{thm}{Theorem}[section]
\newtheorem{prop}{Proposition}[section]
\section{One}
\begin{thm}\label{h:t} A.\end{thm}
\begin{prop}\label{h:p} B.\end{prop}
\section{Two}
\begin{thm}\label{h:t2} C.\end{thm}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["h:t"]; got != "1.1" {
		t.Errorf("h:t = %q, want \"1.1\"", got)
	}
	if got := e.labels["h:p"]; got != "1.1" {
		t.Errorf("h:p = %q, want \"1.1\" (independent counter, same section)", got)
	}
	if got := e.labels["h:t2"]; got != "2.1" {
		t.Errorf("h:t2 = %q, want \"2.1\" (reset once, not twice)", got)
	}
	// \@nsection must carry exactly one \cl@section call.
	body := e.eq["@nsection"].body
	n := 0
	for _, tk := range body {
		if tk.cs_ && tk.cs == "cl@section" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("\\@nsection hooked %d times, want 1", n)
	}
}

// within=subsection nests on and resets with the subsection counter.
func TestTheoremWithinSubsection(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\newtheorem{thm}{Theorem}[subsection]
\section{S}\subsection{A}
\begin{thm}\label{ss:1} P.\end{thm}
\subsection{B}
\begin{thm}\label{ss:2} Q.\end{thm}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["ss:1"]; got != "1.1.1" {
		t.Errorf("ss:1 = %q, want \"1.1.1\"", got)
	}
	if got := e.labels["ss:2"]; got != "1.2.1" {
		t.Errorf("ss:2 = %q, want \"1.2.1\" (reset on subsection)", got)
	}
}

// When the parent sectioning macro is not a plain macro, hookReset declines to
// wrap it (defensive guard) — numbering still nests but no reset is installed.
func TestTheoremWithinNonMacroParent(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\let\@nsection\relax
\newtheorem{thm}{Theorem}[section]`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if m := e.eq["@nsection"]; m == nil || m.kind == mMacro {
		t.Errorf("\\@nsection should remain the \\relax primitive, got %+v", m)
	}
}

// hookReset only wraps the sectioning macro of a supported parent (section /
// subsection); an unsupported parent (here the existing "equation" counter)
// exercises hookReset's default branch: numbering still nests, but there is no
// automatic reset hook.
func TestNewtheoremUnsupportedWithin(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\newtheorem{thm}{Theorem}[equation]
\begin{thm}\label{c:a} Body.\end{thm}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	// c@equation exists (value 0 here), so the number nests as "0.1".
	if got := e.labels["c:a"]; got != "0.1" {
		t.Errorf("label = %q, want \"0.1\"", got)
	}
}

// \newtheorem*{env}{Heading} is amsthm's unnumbered form. Unhandled, the star
// stopped readBraceName dead: nothing was defined and the star and the heading were
// left in the stream to be TYPESET — one arXiv paper opened on a spurious page
// carrying "*theoremTheorem *namedconjectureConjecture".
func TestNewtheoremStarDefinesAnUnnumberedTheorem(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\newtheorem*{thm}{Theorem}\newtheorem{lem}{Lemma}` +
		`\begin{thm}corps\end{thm}\begin{lem}autre\end{lem}\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if strings.Contains(txt, "*") {
		t.Errorf("the star was typeset: %q", txt)
	}
	if !strings.Contains(txt, "Theorem") || !strings.Contains(txt, "corps") {
		t.Errorf("the unnumbered theorem did not print its heading and body: %q", txt)
	}
	// Unnumbered: no "Theorem 1", while the numbered sibling still counts.
	if strings.Contains(txt, "Theorem1") || strings.Contains(txt, "Theorem 1") {
		t.Errorf("a \\newtheorem* environment must carry no number: %q", txt)
	}
	if !strings.Contains(strings.ReplaceAll(txt, " ", ""), "Lemma1") {
		t.Errorf("the numbered sibling lost its number: %q", txt)
	}
}

// The starred form keeps the optional note: \begin{thm}[Smith] heads "Theorem (Smith)."
func TestNewtheoremStarKeepsItsNote(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\newtheorem*{thm}{Theorem}\begin{thm}[Smith]corps\end{thm}\par`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "Smith") {
		t.Errorf("the note was lost: %q", txt)
	}
}
