package engine

import "testing"

// glyphSizePt returns the point size recorded on the first charNode for rune ch in
// the engine's main vertical list (0 if none) — the size at which that glyph is set.
func glyphSizePt(e *Engine, ch rune) int {
	var found int
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			if found != 0 {
				return
			}
			switch c := n.(type) {
			case charNode:
				if c.ch == ch {
					found = c.size
				}
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	return found
}

// The size commands \tiny…\Huge (and \normalsize) each set their clo point size and
// leading, and the ACTIVE size clo — size10/size11/size12.clo — decides the table,
// so an [11pt]/[12pt] class gets its own larger sizes. A \Large in a 12pt document
// is the 12pt clo's \Large (17pt on 22pt leading), not the 10pt one. Before this
// fix every command was a no-op: the clo defines each via \@setfontsize, which the
// engine gobbles, so headings/captions/footnotes all set at the body size.
func TestSizeCloCommands(t *testing.T) {
	type sl struct {
		cmd    string
		size   int
		leadPt float64
	}
	for _, base := range []struct {
		opt   string
		table []sl
	}{
		{"10pt", []sl{
			{"tiny", 5, 6}, {"scriptsize", 7, 8}, {"footnotesize", 8, 9.5}, {"small", 9, 11},
			{"normalsize", 10, 12}, {"large", 12, 14}, {"Large", 14, 18}, {"LARGE", 17, 22},
			{"huge", 20, 25}, {"Huge", 25, 30},
		}},
		{"11pt", []sl{
			{"tiny", 6, 7}, {"scriptsize", 8, 9.5}, {"footnotesize", 9, 11}, {"small", 10, 12},
			{"normalsize", 11, 13.6}, {"large", 12, 14}, {"Large", 14, 18}, {"LARGE", 17, 22},
			{"huge", 20, 25}, {"Huge", 25, 30},
		}},
		{"12pt", []sl{
			{"tiny", 6, 7}, {"scriptsize", 8, 9.5}, {"footnotesize", 10, 12}, {"small", 11, 13.6},
			{"normalsize", 12, 14.5}, {"large", 14, 18}, {"Large", 17, 22}, {"LARGE", 20, 25},
			{"huge", 25, 30}, {"Huge", 25, 30},
		}},
	} {
		for _, tc := range base.table {
			src := `\documentclass[` + base.opt + `]{article}\begin{document}\` + tc.cmd + ` Q\end{document}`
			e, err := compile([]byte(src), Options{})
			if err != nil {
				t.Fatalf("[%s] \\%s: %v", base.opt, tc.cmd, err)
			}
			if got := glyphSizePt(e, 'Q'); got != tc.size {
				t.Errorf("[%s] \\%s glyph = %dpt, want %dpt", base.opt, tc.cmd, got, tc.size)
			}
			if want := ptToSP(tc.leadPt); e.baselineskip != want {
				t.Errorf("[%s] \\%s \\baselineskip = %d sp, want %d sp (%.1fpt)",
					base.opt, tc.cmd, e.baselineskip, want, tc.leadPt)
			}
		}
	}
}

// A size change inside a group is undone when the group closes: {\footnotesize …}
// and {\Large …} each restore both the font and \baselineskip, so the text after
// the group is body text again. (\normalsize is not rewired — resetting is done by
// grouping, which is how the size commands are meant to be scoped.)
func TestSizeCloGrouping(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt]{article}\begin{document}`+
		`A {\footnotesize f} B {\Large g} C\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := glyphSizePt(e, 'A'); got != 10 {
		t.Errorf("body A = %dpt, want 10", got)
	}
	if got := glyphSizePt(e, 'f'); got != 8 {
		t.Errorf("footnotesize f = %dpt, want 8", got)
	}
	if got := glyphSizePt(e, 'B'); got != 10 {
		t.Errorf("B after grouped footnotesize = %dpt, want 10 (restored)", got)
	}
	if got := glyphSizePt(e, 'g'); got != 14 {
		t.Errorf("grouped \\Large g = %dpt, want 14", got)
	}
	if got := glyphSizePt(e, 'C'); got != 10 {
		t.Errorf("C after grouped \\Large = %dpt, want 10 (restored)", got)
	}
	if e.baselineskip != 12*unity {
		t.Errorf("final \\baselineskip = %d sp, want %d (restored)", e.baselineskip, 12*unity)
	}
}

// A section heading is set with \Large (its clo size), which is the point of the
// fix: the heading now occupies its true, larger vertical space.
func TestSizeCloSectionHeading(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt]{article}\begin{document}`+
		`\section{Zoo}Body text long enough to wrap onto a line.\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := glyphSizePt(e, 'Z'); got != 14 {
		t.Errorf("\\section heading Z = %dpt, want 14 (\\Large)", got)
	}
}

// A \protected size command survives a moving context (a section title flowing into
// the table of contents) without corrupting the scan: the document compiles in full
// rather than being truncated, which is why the command is driven through the font
// system and marked \protected rather than making \@setfontsize a live primitive.
func TestSizeCloMovingContext(t *testing.T) {
	e, err := compile([]byte(`\documentclass[10pt]{article}\begin{document}`+
		`\tableofcontents\section{Alpha}First body paragraph with several words to set.`+
		`\section{Beta}Second body paragraph, also with enough words to wrap here.\end{document}`), Options{})
	if err != nil {
		t.Fatalf("section+toc compile failed: %v", err)
	}
	// The bodies must reach the page — a corrupted scan would swallow them.
	for _, ch := range []rune{'F', 'S'} {
		if glyphSizePt(e, ch) == 0 {
			t.Errorf("body starting %q missing — moving context truncated the document", ch)
		}
	}
}

// Without a size clo (no \documentclass loading one), the kernel's own \large etc.
// are not \@setfontsize switches, so rewireSizeCommands leaves them untouched.
func TestSizeCloUntouchedWithoutClo(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	before := e.eq["Large"]
	e.rewireSizeCommands()
	if e.eq["Large"] != before {
		t.Errorf("\\Large redefined though its body is not a clo \\@setfontsize switch")
	}
}

// rewireSizeCommands skips a size name that is absent or not a macro, and clamps a
// sub-1pt clo size to at least 1pt.
func TestRewireSizeCommandsEdges(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(scaleMock{px: 10}) // base 10pt, scalable
	// tiny is not a macro (a primitive here) — left untouched.
	e.eq["tiny"] = &meaning{kind: mPrim, name: "tiny"}
	// small is absent — skipped, not defined.
	delete(e.eq, "small")
	// large carries a sub-1pt size {0.4} → the px rounds to 0 and is clamped to 1pt.
	e.eq["large"] = &meaning{kind: mMacro, body: []tok{csTok("@setfontsize"), csTok("large"),
		chTok('{', catBegin), chTok('0', catOther), chTok('.', catOther), chTok('4', catOther), chTok('}', catEnd),
		chTok('{', catBegin), chTok('6', catOther), chTok('}', catEnd)}}
	e.rewireSizeCommands()
	if e.eq["tiny"].kind != mPrim {
		t.Errorf("non-macro \\tiny was rewired")
	}
	if e.eq["small"] != nil {
		t.Errorf("absent \\small was defined")
	}
	if _, err := e.Run(`\large Q`); err != nil {
		t.Fatal(err)
	}
	if e.curFont.sizePt() != 1 {
		t.Errorf("sub-1pt \\large clamped to %dpt, want 1", e.curFont.sizePt())
	}
}

// parseSetfontsize reads the (size, leading) a clo baked into a size command; the
// size and leading may each be a braced number or a \@NNpt macro, and a body that
// does not begin with \@setfontsize is rejected.
func TestParseSetfontsize(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	// \@setfontsize\Large\@xivpt{18}
	body := []tok{csTok("@setfontsize"), csTok("Large"), csTok("@xivpt"),
		chTok('{', catBegin), chTok('1', catOther), chTok('8', catOther), chTok('}', catEnd)}
	if s, l, _, ok := e.parseSetfontsize(body); !ok || s != 14 || l != 18 {
		t.Errorf("size/leading = %v/%v ok=%v, want 14/18 true", s, l, ok)
	}
	// leading as a macro: \@setfontsize\tiny\@vpt\@vipt
	body2 := []tok{csTok("@setfontsize"), csTok("tiny"), csTok("@vpt"), csTok("@vipt")}
	if s, l, _, ok := e.parseSetfontsize(body2); !ok || s != 5 || l != 6 {
		t.Errorf("macro leading: size/leading = %v/%v ok=%v, want 5/6 true", s, l, ok)
	}
	// decimal braced leading: {9.5}
	body3 := []tok{csTok("@setfontsize"), csTok("footnotesize"), csTok("@viiipt"),
		chTok('{', catBegin), chTok('9', catOther), chTok('.', catOther), chTok('5', catOther), chTok('}', catEnd)}
	if _, l, _, ok := e.parseSetfontsize(body3); !ok || l != 9.5 {
		t.Errorf("decimal leading = %v ok=%v, want 9.5 true", l, ok)
	}
	// not a \@setfontsize body
	if _, _, _, ok := e.parseSetfontsize([]tok{csTok("gotexsize"), chTok('1', catOther)}); ok {
		t.Errorf("non-\\@setfontsize body accepted")
	}
	// empty body
	if _, _, _, ok := e.parseSetfontsize(nil); ok {
		t.Errorf("empty body accepted")
	}
	// unresolvable size macro
	bad := []tok{csTok("@setfontsize"), csTok("x"), csTok("@notdefined"), chTok('{', catBegin), chTok('1', catOther), chTok('}', catEnd)}
	if _, _, _, ok := e.parseSetfontsize(bad); ok {
		t.Errorf("undefined size macro accepted")
	}
}

// evalNumToks reads a number from a digit run, a decimal, or one macro level; it
// rejects a control sequence in a multi-token run and a non-numeric body.
func TestEvalNumToks(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	if v, ok := e.evalNumToks([]tok{chTok('1', catOther), chTok('4', catOther)}); !ok || v != 14 {
		t.Errorf("14 -> %v %v", v, ok)
	}
	if v, ok := e.evalNumToks([]tok{csTok("@xivpt")}); !ok || v != 14 {
		t.Errorf("\\@xivpt -> %v %v", v, ok)
	}
	if _, ok := e.evalNumToks([]tok{chTok('1', catOther), csTok("relax")}); ok {
		t.Errorf("digit+cs run accepted")
	}
	if _, ok := e.evalNumToks([]tok{chTok('x', catLetter)}); ok {
		t.Errorf("non-numeric accepted")
	}
	if _, ok := e.evalNumToks([]tok{csTok("relax")}); ok {
		t.Errorf("non-macro single cs accepted")
	}
	// digit/'.' characters that do not form a number ("..") fail to parse.
	if _, ok := e.evalNumToks([]tok{chTok('.', catOther), chTok('.', catOther)}); ok {
		t.Errorf("unparseable \"..\" accepted")
	}
}

// grabTokArg reads one undelimited argument from a token slice: a single token, a
// braced group (balanced), or an unbalanced tail; leading spaces are skipped.
func TestGrabTokArg(t *testing.T) {
	body := []tok{chTok(' ', catSpace), csTok("a"),
		chTok('{', catBegin), chTok('x', catLetter), chTok('{', catBegin), chTok('y', catLetter), chTok('}', catEnd), chTok('}', catEnd)}
	a1, i := grabTokArg(body, 0)
	if len(a1) != 1 || !a1[0].cs_ || a1[0].cs != "a" {
		t.Errorf("arg1 = %v, want [\\a]", a1)
	}
	a2, _ := grabTokArg(body, i)
	if len(a2) != 4 { // x { y }
		t.Errorf("arg2 len = %d, want 4 (balanced inner group)", len(a2))
	}
	// past the end
	if a, j := grabTokArg(body, len(body)); a != nil || j != len(body) {
		t.Errorf("past-end grab returned %v %d", a, j)
	}
	// unbalanced group runs to the end
	ub := []tok{chTok('{', catBegin), chTok('z', catLetter)}
	if a, _ := grabTokArg(ub, 0); len(a) != 1 || a[0].ch != 'z' {
		t.Errorf("unbalanced grab = %v, want [z]", a)
	}
}
