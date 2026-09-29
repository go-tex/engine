// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// runSkip compiles a document body in lenient mode (so unknown commands are
// tallied rather than aborting) and returns the skipped-command counts.
func runSkip(t *testing.T, body string) map[string]int {
	t.Helper()
	src := `\documentclass{article}\begin{document}` + body + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile %q: %v", body, err)
	}
	return e.SkippedCommands()
}

// runPics returns how many picture environments were stood in for by a framed
// box. The count lives in the FIGURE channel, not among the skipped commands:
// \tikzpicture is defined, its body is consumed whole and a box is reserved for
// it, so calling it an undefined command misreports what the engine did — and
// put it at the top of that channel over the whole corpus.
func runPics(t *testing.T, body string) int {
	t.Helper()
	src := `\documentclass{article}\begin{document}` + body + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile %q: %v", body, err)
	}
	return e.Diagnostics().FiguresDropped[pictureDropReason]
}

// noLeak asserts none of the TikZ drawing commands leaked as skipped commands
// (they were gobbled with their environment, not seen one by one).
func noLeak(t *testing.T, skip map[string]int, cmds ...string) {
	t.Helper()
	for _, c := range cmds {
		if skip[c] != 0 {
			t.Errorf("TikZ command %q leaked (skip count %d) — environment not gobbled", c, skip[c])
		}
	}
}

// TestTikzGobble_NoLeak checks a tikzpicture body is gobbled whole: no \draw,
// \node, \path or \foreach leaks, a placeholder is recorded, and the command just
// after \end{tikzpicture} is still processed (the scan stopped at the right \end).
func TestTikzGobble_NoLeak(t *testing.T) {
	skip := runSkip(t, `X \begin{tikzpicture}[scale=2] `+
		`\draw (0,0)--(1,1); \node at (0,0) {N}; \path (0,0) rectangle (1,1); `+
		`\foreach \x in {1,2} {\draw (\x,0) circle (2pt);} `+
		`\end{tikzpicture} Y \zzmarker`)
	noLeak(t, skip, "draw", "node", "path", "foreach")
	if skip["tikzpicture"] != 0 {
		t.Error("a gobbled tikzpicture is not an undefined command and must not be tallied as one")
	}
	if n := runPics(t, `X \begin{tikzpicture}[scale=2] `+
		`\draw (0,0)--(1,1); \node at (0,0) {N}; \path (0,0) rectangle (1,1); `+
		`\foreach \x in {1,2} {\draw (\x,0) circle (2pt);} `+
		`\end{tikzpicture} Y \zzmarker`); n != 1 {
		t.Errorf("picture placeholders = %d, want 1", n)
	}
	if skip["zzmarker"] != 1 {
		t.Errorf("content after \\end{tikzpicture} not processed: zzmarker=%d (want 1)", skip["zzmarker"])
	}
}

// TestTikzGobble_Nested checks a tikzpicture nested inside another is consumed by
// the outer gobble (matching \begin/\end of the same name), leaving exactly one
// placeholder and no leak, and stopping at the correct outer \end.
func TestTikzGobble_Nested(t *testing.T) {
	skip := runSkip(t, `\begin{tikzpicture} \draw a; `+
		`\begin{tikzpicture} \draw b; \end{tikzpicture} \draw c; `+
		`\end{tikzpicture} \zzafter`)
	noLeak(t, skip, "draw")
	if n := runPics(t, `\begin{tikzpicture} \draw a; `+
		`\begin{tikzpicture} \draw b; \end{tikzpicture} \draw c; `+
		`\end{tikzpicture} \zzafter`); n != 1 {
		t.Errorf("nested tikzpicture placeholder count = %d (want 1)", n)
	}
	if skip["zzafter"] != 1 {
		t.Errorf("content after nested env not processed: zzafter=%d (want 1)", skip["zzafter"])
	}
}

// TestTikzGobble_FromMacroBody is the critical case: the environment is replayed
// from a macro body, so the raw input cursor points elsewhere. A raw source scan
// would eat the real document text that follows; the token-level scan must not.
func TestTikzGobble_FromMacroBody(t *testing.T) {
	skip := runSkip(t, `\def\diagram{\begin{tikzpicture}\draw z;\end{tikzpicture}}`+
		`\zzbefore \diagram \zzafter`)
	noLeak(t, skip, "draw")
	if n := runPics(t, `\def\diagram{\begin{tikzpicture}\draw z;\end{tikzpicture}}`+
		`\zzbefore \diagram \zzafter`); n != 1 {
		t.Errorf("macro-body tikzpicture placeholders = %d, want 1", n)
	}
	if skip["zzbefore"] != 1 || skip["zzafter"] != 1 {
		t.Errorf("surrounding text eaten: zzbefore=%d zzafter=%d (want 1,1)",
			skip["zzbefore"], skip["zzafter"])
	}
}

// TestTikzGobble_PgfAndCd checks pgfpicture and tikzcd are gobbled the same way.
func TestTikzGobble_PgfAndCd(t *testing.T) {
	skip := runSkip(t, `\begin{pgfpicture}\pgfusepath{stroke}\end{pgfpicture}\zzp `+
		`\begin{tikzcd} A \arrow{r} & B \end{tikzcd}\zzc`)
	noLeak(t, skip, "pgfusepath", "arrow")
	// One bucket for every picture environment, so the two count together.
	if n := runPics(t, `\begin{pgfpicture}\pgfusepath{stroke}\end{pgfpicture}\zzp `+
		`\begin{tikzcd} A \arrow{r} & B \end{tikzcd}\zzc`); n != 2 {
		t.Errorf("pgfpicture + tikzcd placeholders = %d, want 2", n)
	}
	if skip["zzp"] != 1 || skip["zzc"] != 1 {
		t.Errorf("content after env not processed: zzp=%d zzc=%d", skip["zzp"], skip["zzc"])
	}
}

// TestTikzGobble_Unterminated checks an environment with no matching \end consumes
// the rest of the input without erroring and emits no placeholder (never closed).
func TestTikzGobble_Unterminated(t *testing.T) {
	skip := runSkip(t, `before \begin{tikzpicture}\draw q; and more text with no closing`)
	noLeak(t, skip, "draw")
	if n := runPics(t, `before \begin{tikzpicture}\draw q; and more text with no closing`); n != 0 {
		t.Errorf("unterminated env should not emit a placeholder, got %d", n)
	}
}

// TestTikzGobble_EnvNameEdges exercises gobbleEnvName's parsing branches: a
// control sequence inside the {name} group (skipped), a nested brace group inside
// it (balanced), and an \end whose name is not a brace group (pushed back).
func TestTikzGobble_EnvNameEdges(t *testing.T) {
	// \relax inside the closing name (cs branch) and a {} group inside it
	// (nested-brace branch); the letters still spell "tikzpicture", so it matches.
	skip := runSkip(t, `\begin{tikzpicture}\draw a;\end{tikz\relax{}picture} tail \zztail`)
	noLeak(t, skip, "draw")
	if n := runPics(t, `\begin{tikzpicture}\draw a;\end{tikz\relax{}picture} tail \zztail`); n != 1 {
		t.Error("edge-spelled \\end name did not close the environment")
	}
	if skip["zztail"] != 1 {
		t.Errorf("tail after edge \\end not processed: zztail=%d", skip["zztail"])
	}

	// \end not followed by a brace group: gobbleEnvName pushes the token back and
	// returns "" (no match), so the environment stays open to end of input.
	skip2 := runSkip(t, `\begin{tikzpicture}\draw b;\end zzz`)
	noLeak(t, skip2, "draw")
	if n := runPics(t, `\begin{tikzpicture}\draw b;\end zzz`); n != 0 {
		t.Errorf("non-brace \\end must not close the env: placeholder=%d", n)
	}
}

// TestTikzGobble_PlaceholderInitsFigureMap runs a bare picture in strict mode from
// a fresh engine whose figure map is still nil, so recordPictureDrop takes the
// lazy-init branch — and checks the same run leaves the skipped-command tally
// empty, since nothing here is undefined.
func TestTikzGobble_PlaceholderInitsFigureMap(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.figuresDropped = nil // force the nil-map branch
	if _, err := e.Run(`\begin{tikzpicture}\end{tikzpicture}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := e.Diagnostics().FiguresDropped[pictureDropReason]; got != 1 {
		t.Errorf("placeholder not recorded from nil map: %v", e.Diagnostics().FiguresDropped)
	}
	if len(e.SkippedCommands()) != 0 {
		t.Errorf("a handled picture was tallied as an undefined command: %v", e.SkippedCommands())
	}
}

// TestGobbleEnvName_Branches white-box tests gobbleEnvName by feeding token lists
// directly, covering: a leading space before the group, immediate end of input, a
// non-brace token (pushed back), an unterminated name group (end of input mid-
// name), and a control sequence plus a nested brace group inside the name.
func TestGobbleEnvName_Branches(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.base, e.bpos = []rune{}, 0 // no base input: getNext ends when the pushed list drains

	// Leading space, then {a}: the space-skip loop runs, name is "a".
	e.push([]tok{chTok('{', catBegin), chTok('a', catLetter), chTok('}', catEnd)})
	e.push([]tok{chTok(' ', catSpace)})
	if got := e.gobbleEnvName(); got != "a" {
		t.Errorf("space+{a}: got %q, want a", got)
	}

	// Nothing left: getNext returns !ok immediately, name is "".
	if got := e.gobbleEnvName(); got != "" {
		t.Errorf("empty input: got %q, want empty", got)
	}

	// A non-brace token is pushed back and yields "".
	e.push([]tok{csTok("foo")})
	if got := e.gobbleEnvName(); got != "" {
		t.Errorf("non-brace: got %q, want empty", got)
	}
	if tk, ok := e.getNext(); !ok || !tk.cs_ || tk.cs != "foo" {
		t.Errorf("non-brace token was not pushed back: %+v ok=%v", tk, ok)
	}

	// Unterminated name group: input ends mid-name, loop breaks on !ok.
	e.push([]tok{chTok('{', catBegin), chTok('x', catLetter)})
	if got := e.gobbleEnvName(); got != "x" {
		t.Errorf("unterminated name: got %q, want x", got)
	}

	// A control sequence (skipped) and a nested {} group (balanced) inside a name.
	e.push([]tok{
		chTok('{', catBegin), chTok('t', catLetter), csTok("z"),
		chTok('{', catBegin), chTok('}', catEnd),
		chTok('u', catLetter), chTok('}', catEnd),
	})
	if got := e.gobbleEnvName(); got != "tu" {
		t.Errorf("cs+nested-brace name: got %q, want tu", got)
	}
}
