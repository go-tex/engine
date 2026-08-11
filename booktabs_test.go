package engine

import "testing"

// fullRules returns the full-width horizontal rule nodes that sit directly in a
// tabular vbox (booktabs \toprule/\midrule/\bottomrule and \hline), in order.
func fullRules(tb *boxNode) []ruleNode {
	var rs []ruleNode
	for _, n := range tb.list {
		if r, ok := n.(ruleNode); ok {
			rs = append(rs, r)
		}
	}
	return rs
}

// partialRuleBox returns the first \cmidrule/\cline box in a vbox (a natural hbox
// holding exactly a left kern followed by a rule), or nil when none is present.
func partialRuleBox(tb *boxNode) *boxNode {
	for _, n := range tb.list {
		b, ok := n.(*boxNode)
		if !ok || b.kind != hbox || len(b.list) != 2 {
			continue
		}
		if _, ok := b.list[0].(kernNode); !ok {
			continue
		}
		if _, ok := b.list[1].(ruleNode); ok {
			return b
		}
	}
	return nil
}

// maxRowWidth returns the width of the widest data row (skipping the two-node
// \cmidrule boxes), i.e. the table width the full-width rules must match.
func maxRowWidth(tb *boxNode) int {
	w := 0
	for _, n := range tb.list {
		b, ok := n.(*boxNode)
		if !ok || b.kind != hbox {
			continue
		}
		if len(b.list) == 2 { // a partial-rule box, not a data row
			if _, ok := b.list[0].(kernNode); ok {
				if _, ok := b.list[1].(ruleNode); ok {
					continue
				}
			}
		}
		if b.width > w {
			w = b.width
		}
	}
	return w
}

// \toprule and \bottomrule draw the heavier rule; \midrule the lighter one. All
// three span the full table width and are wrapped in bookRuleSep breathing glue.
func TestBooktabsFullRules(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.Run(`\begin{tabular}{ll}\toprule A & B \\ \midrule c & d \\ e & f \\ \bottomrule\end{tabular}`)
	tb := lastVbox(e)
	if tb == nil {
		t.Fatal("no tabular vbox")
	}
	rules := fullRules(tb)
	if len(rules) != 3 {
		t.Fatalf("got %d full-width rules, want 3 (top/mid/bottom)", len(rules))
	}
	// Exact weights: top and bottom heavy, mid light, and heavy != light.
	if rules[0].height != heavyRuleWidth {
		t.Errorf("\\toprule weight = %d sp, want %d (heavy)", rules[0].height, heavyRuleWidth)
	}
	if rules[1].height != lightRuleWidth {
		t.Errorf("\\midrule weight = %d sp, want %d (light)", rules[1].height, lightRuleWidth)
	}
	if rules[2].height != heavyRuleWidth {
		t.Errorf("\\bottomrule weight = %d sp, want %d (heavy)", rules[2].height, heavyRuleWidth)
	}
	if rules[0].height == rules[1].height {
		t.Errorf("top and mid rules must differ in weight, both %d sp", rules[0].height)
	}
	if heavyRuleWidth != 2*lightRuleWidth {
		t.Errorf("heavy rule should be twice the light rule: %d vs %d", heavyRuleWidth, lightRuleWidth)
	}
	// Every full rule spans the whole table width.
	want := maxRowWidth(tb)
	for i, r := range rules {
		if r.width != want {
			t.Errorf("full rule %d width = %d sp, want table width %d", i, r.width, want)
		}
	}
	// Each full rule is wrapped in bookRuleSep glue above and below (airier look).
	for i, n := range tb.list {
		if _, ok := n.(ruleNode); !ok {
			continue
		}
		before, _ := tb.list[i-1].(glueNode)
		after, _ := tb.list[i+1].(glueNode)
		if before.spec.width != bookRuleSep || after.spec.width != bookRuleSep {
			t.Errorf("rule at %d not wrapped in %d-sp glue: before=%d after=%d",
				i, bookRuleSep, before.spec.width, after.spec.width)
		}
	}
	// Exactly the three real data rows survive as multi-node hboxes.
	rows := 0
	for _, n := range tb.list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox && len(b.list) > 2 {
			rows++
		}
	}
	if rows != 3 {
		t.Errorf("got %d data rows, want 3", rows)
	}
}

// partialColRule geometry: \cmidrule mirrors \cline (same kern/width) at the light
// weight, and (l)/(r)/(lr) trimming shaves a fixed cmidTrim off the named side(s).
func TestCmidrulePartialGeometry(t *testing.T) {
	colw := []int{20 * unity, 30 * unity, 40 * unity}
	noRule := func(int) bool { return false }

	// {2-3}: kern past column 1 (2·6+20 = 32pt); span columns 2+3 (2·(2·6)+30+40 = 94pt).
	n := partialColRule(2, 3, 3, colw, noRule, lightRuleWidth, false, false)
	box, ok := n.(*boxNode)
	if !ok {
		t.Fatalf("partialColRule returned %T, want *boxNode", n)
	}
	if k := box.list[0].(kernNode); k.width != 32*unity {
		t.Errorf("cmidrule left kern = %d sp, want 32pt", k.width)
	}
	if r := box.list[1].(ruleNode); r.width != 94*unity {
		t.Errorf("cmidrule width = %d sp, want 94pt", r.width)
	} else if r.height != lightRuleWidth {
		t.Errorf("cmidrule weight = %d sp, want %d (light)", r.height, lightRuleWidth)
	}

	// (lr) trims both sides: kern += cmidTrim, width -= 2·cmidTrim.
	nb := partialColRule(2, 3, 3, colw, noRule, lightRuleWidth, true, true).(*boxNode)
	if k := nb.list[0].(kernNode); k.width != 32*unity+cmidTrim {
		t.Errorf("(lr) left kern = %d sp, want %d", k.width, 32*unity+cmidTrim)
	}
	if r := nb.list[1].(ruleNode); r.width != 94*unity-2*cmidTrim {
		t.Errorf("(lr) width = %d sp, want %d", r.width, 94*unity-2*cmidTrim)
	}

	// (l) trims only the left; (r) only the right.
	if k := partialColRule(2, 3, 3, colw, noRule, lightRuleWidth, true, false).(*boxNode).list[0].(kernNode); k.width != 32*unity+cmidTrim {
		t.Errorf("(l) left kern = %d sp, want %d", k.width, 32*unity+cmidTrim)
	}
	if r := partialColRule(2, 3, 3, colw, noRule, lightRuleWidth, false, true).(*boxNode).list[1].(ruleNode); r.width != 94*unity-cmidTrim {
		t.Errorf("(r) width = %d sp, want %d", r.width, 94*unity-cmidTrim)
	}

	// With vertical rules everywhere, each | before column `from` widens the left
	// kern and each interior | widens the rule (defaultRule apiece).
	all := func(int) bool { return true }
	nr := partialColRule(2, 3, 3, colw, all, lightRuleWidth, false, false).(*boxNode)
	if k := nr.list[0].(kernNode); k.width != 32*unity+2*defaultRule {
		t.Errorf("ruled left kern = %d sp, want %d", k.width, 32*unity+2*defaultRule)
	}
	if r := nr.list[1].(ruleNode); r.width != 94*unity+defaultRule {
		t.Errorf("ruled width = %d sp, want %d", r.width, 94*unity+defaultRule)
	}

	// A `to` past the last column is clamped to the last column.
	if r := partialColRule(2, 5, 3, colw, noRule, lightRuleWidth, false, false).(*boxNode).list[1].(ruleNode); r.width != 94*unity {
		t.Errorf("clamped {2-5} width = %d sp, want 94pt (columns 2..3)", r.width)
	}

	// Degenerate ranges render nothing.
	if partialColRule(0, 0, 3, colw, noRule, lightRuleWidth, false, false) != nil {
		t.Error("empty range should render nil")
	}
	if partialColRule(3, 2, 3, colw, noRule, lightRuleWidth, false, false) != nil {
		t.Error("reversed range should render nil")
	}
	if partialColRule(4, 5, 3, colw, noRule, lightRuleWidth, false, false) != nil {
		t.Error("out-of-range start should render nil")
	}
}

// The rule width floors at zero: a degenerate (negative-aggregate) column must
// never produce a negative-width rule (defensive width floor).
func TestCmidruleTrimClamp(t *testing.T) {
	noRule := func(int) bool { return false }
	r := partialColRule(1, 1, 1, []int{-100 * unity}, noRule, lightRuleWidth, false, false).(*boxNode).list[1].(ruleNode)
	if r.width != 0 {
		t.Errorf("negative aggregate width must floor at 0, got %d", r.width)
	}
}

// End to end: \cmidrule{2-3} in a 3-column table of single letters (each column
// 5pt) kerns 2·6+5 = 17pt and spans two 17pt slots = 34pt at the light weight.
func TestCmidruleEngine(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.Run(`\begin{tabular}{lll}\toprule A & B & C \\ \cmidrule{2-3} a & b & c \\ \bottomrule\end{tabular}`)
	tb := lastVbox(e)
	box := partialRuleBox(tb)
	if box == nil {
		t.Fatal("no \\cmidrule partial rule box in vbox")
	}
	if k := box.list[0].(kernNode); k.width != 17*unity {
		t.Errorf("cmidrule kern = %d sp, want 17pt", k.width)
	}
	r := box.list[1].(ruleNode)
	if r.width != 34*unity {
		t.Errorf("cmidrule width = %d sp, want 34pt", r.width)
	}
	if r.height != lightRuleWidth {
		t.Errorf("cmidrule weight = %d sp, want %d", r.height, lightRuleWidth)
	}
}

// The optional (lr) trimming and [weight] override are honoured end to end.
func TestCmidruleTrimAndWeightEngine(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.Run(`\begin{tabular}{lll}A & B & C \\ \cmidrule[0.6pt](lr){2-3} a & b & c\end{tabular}`)
	tb := lastVbox(e)
	box := partialRuleBox(tb)
	if box == nil {
		t.Fatal("no \\cmidrule box")
	}
	if k := box.list[0].(kernNode); k.width != 17*unity+cmidTrim {
		t.Errorf("(lr) cmidrule kern = %d sp, want %d", k.width, 17*unity+cmidTrim)
	}
	r := box.list[1].(ruleNode)
	if r.width != 34*unity-2*cmidTrim {
		t.Errorf("(lr) cmidrule width = %d sp, want %d", r.width, 34*unity-2*cmidTrim)
	}
	if r.height != parseDimenStr("0.6pt") {
		t.Errorf("cmidrule weight override = %d sp, want %d", r.height, parseDimenStr("0.6pt"))
	}
}

// \toprule[<dimen>] overrides the rule weight.
func TestTopruleWeightOverride(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.Run(`\begin{tabular}{ll}\toprule A & B \\ x & y\end{tabular}`)
	base := fullRules(lastVbox(e))
	if len(base) != 1 || base[0].height != heavyRuleWidth {
		t.Fatalf("baseline \\toprule weight = %v, want one heavy rule", base)
	}

	e2 := New()
	if err := e2.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e2.SetFont(spMock{})
	e2.Run(`\begin{tabular}{ll}\toprule[1.5pt] A & B \\ x & y\end{tabular}`)
	over := fullRules(lastVbox(e2))
	if len(over) != 1 {
		t.Fatalf("got %d rules, want 1", len(over))
	}
	if over[0].height != parseDimenStr("1.5pt") {
		t.Errorf("overridden \\toprule weight = %d sp, want %d", over[0].height, parseDimenStr("1.5pt"))
	}
	if over[0].height == heavyRuleWidth {
		t.Error("override should change the weight away from the heavy default")
	}
}

// Malformed \cmidrule ranges (reversed, empty, missing braces) draw nothing and
// never panic.
func TestBooktabsMalformed(t *testing.T) {
	cases := []string{
		`\begin{tabular}{lll}\toprule a & b & c \\ \cmidrule{9-2} d & e & f\end{tabular}`,
		`\begin{tabular}{lll}\toprule a & b & c \\ \cmidrule{} d & e & f\end{tabular}`,
		`\begin{tabular}{lll}\toprule a & b & c \\ \cmidrule d & e & f\end{tabular}`,
	}
	for _, src := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		e.Run(src) // must not panic
		tb := lastVbox(e)
		if tb == nil {
			t.Fatalf("no vbox for %q", src)
		}
		if partialRuleBox(tb) != nil {
			t.Errorf("malformed \\cmidrule should draw no partial rule: %q", src)
		}
	}
}

// A \multirow spanning across a booktabs rule must include that rule's weight and
// breathing glue in the spanned extent (exercises spannedExtent's booktabs path)
// and must not panic.
func TestMultirowAcrossBooktabsRule(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.Run(`\begin{tabular}{ll}\multirow{2}{*}{X} & a \\ \midrule & b\end{tabular}`)
	tb := lastVbox(e)
	if tb == nil {
		t.Fatal("no vbox")
	}
	// The mid rule is present at the light weight between the two spanned rows.
	rules := fullRules(tb)
	if len(rules) != 1 || rules[0].height != lightRuleWidth {
		t.Fatalf("want one light \\midrule, got %v", rules)
	}
}

// The optional-argument readers back the stream out cleanly when the delimiter is
// absent, honour l/r selection when present, and survive a truncated stream.
func TestBooktabsOptArgHelpers(t *testing.T) {
	// Absent optional argument on an exhausted stream: nothing consumed, false.
	e := New()
	if _, ok := e.readOptDelimited('[', ']'); ok {
		t.Error("readOptDelimited on empty input should report absent")
	}
	if _, ok := e.readOptBracketDimen(); ok {
		t.Error("readOptBracketDimen on empty input should report absent")
	}
	if l, r := e.readCmidTrim(); l || r {
		t.Error("readCmidTrim on empty input should trim nothing")
	}

	// Present but never closed (opener then EOF): returns true with the text so far.
	e2 := New()
	e2.back(tok{ch: '(', cat: catOther}) // lone opener, then end of input
	l, r := e2.readCmidTrim()
	if l || r {
		t.Errorf("unterminated empty (): got l=%v r=%v, want neither", l, r)
	}

	// pushToks feeds tokens so that getNext reads them left to right.
	pushToks := func(e *Engine, toks ...tok) {
		for i := len(toks) - 1; i >= 0; i-- {
			e.back(toks[i])
		}
	}
	sp := tok{ch: ' ', cat: catSpace}
	oth := func(r rune) tok { return tok{ch: r, cat: catOther} }

	// l/r selection, with a leading space to exercise the space-skip path.
	e3 := New()
	pushToks(e3, sp, oth('('), oth('l'), oth(')'))
	if l, r := e3.readCmidTrim(); !l || r {
		t.Errorf("(l) trim = (l=%v r=%v), want (true,false)", l, r)
	}
	e4 := New()
	pushToks(e4, oth('('), oth('r'), oth(')'))
	if l, r := e4.readCmidTrim(); l || !r {
		t.Errorf("(r) trim = (l=%v r=%v), want (false,true)", l, r)
	}

	// Leading space then a non-opener: consume nothing, report absent.
	e5 := New()
	pushToks(e5, sp, oth('x'))
	if _, ok := e5.readOptDelimited('(', ')'); ok {
		t.Error("space then non-opener should report absent")
	}
	if tk, _ := e5.getNext(); tk.cat != catSpace {
		t.Error("readOptDelimited must restore the skipped space")
	}

	// Leading space then EOF: report absent (top back-out path with skipped tokens).
	e6 := New()
	pushToks(e6, sp)
	if _, ok := e6.readOptDelimited('(', ')'); ok {
		t.Error("space then EOF should report absent")
	}
}
