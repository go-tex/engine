package engine

import "testing"

// \rule{w}{h} produces a filled rectangle of the given dimensions.
func TestRuleCommand(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.Run(`\setbox0=\hbox{a\rule{4pt}{6pt}b}`)
	var r *ruleNode
	for _, n := range e.box[0].list {
		if rn, ok := n.(ruleNode); ok {
			rr := rn
			r = &rr
		}
	}
	if r == nil {
		t.Fatal("no rule node produced by \\rule")
	}
	if r.width != 4*unity || r.height != 6*unity || r.depth != 0 {
		t.Errorf("\\rule{4pt}{6pt} = w%d h%d d%d, want 4pt/6pt/0", r.width, r.height, r.depth)
	}
	// box width includes a(5) + rule(4) + b(5) = 14pt
	if e.box[0].width != 14*unity {
		t.Errorf("box width %d want 14pt", e.box[0].width)
	}
}

// \rule[lift]{w}{h} lifts the rule (depth = lift).
func TestRuleLift(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.Run(`\setbox0=\hbox{\rule[2pt]{3pt}{6pt}}`)
	r := e.box[0].list[0].(ruleNode)
	if r.depth != 2*unity || r.height != 4*unity {
		t.Errorf("\\rule[2pt]{3pt}{6pt}: h%d d%d want height 4pt depth 2pt", r.height, r.depth)
	}
}
