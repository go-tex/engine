package engine

import "testing"

// packedHTotal sums an hbox's children at their set widths; it must equal the
// box's target width when the glue-set correctly absorbs the excess.
func packedHTotal(b *boxNode) int {
	tot := 0
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			tot += c.width
		case glueNode:
			tot += b.setWidth(c.spec)
		case ruleNode:
			if !c.widthRun {
				tot += c.width
			}
		case *boxNode:
			tot += c.width
		}
	}
	return tot
}

// buildHBox runs a \setbox0=\hbox... and returns the packed box.
func buildHBox(t *testing.T, src string) *boxNode {
	e := New()
	if _, err := e.Run(src); err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	b := e.box[0]
	if b == nil {
		t.Fatalf("%q: box0 void", src)
	}
	return b
}

func TestGlueSetStretch(t *testing.T) {
	b := buildHBox(t, `\setbox0=\hbox to 10pt{\kern2pt\hskip1pt plus 1pt}`)
	if b.glueSign != 1 || b.glueSet != 7.0 {
		t.Fatalf("sign=%d set=%v want stretching 7.0", b.glueSign, b.glueSet)
	}
	if got := packedHTotal(b); got != 10*unity {
		t.Fatalf("packed total %d sp want %d", got, 10*unity)
	}
}

func TestGlueSetShrink(t *testing.T) {
	b := buildHBox(t, `\setbox0=\hbox to 4pt{\kern2pt\hskip3pt minus 2pt}`)
	if b.glueSign != 2 || b.glueSet != 0.5 {
		t.Fatalf("sign=%d set=%v want shrinking 0.5", b.glueSign, b.glueSet)
	}
	if got := packedHTotal(b); got != 4*unity {
		t.Fatalf("packed total %d sp want %d", got, 4*unity)
	}
}

func TestGlueSetInfiniteOrderDominates(t *testing.T) {
	// A fil (order 1) present ⇒ finite (order 0) stretch gets nothing.
	b := buildHBox(t, `\setbox0=\hbox to 10pt{\kern2pt\hskip0pt plus 1pt\hskip0pt plus 1fil}`)
	if b.glueSign != 1 || b.glueOrder != 1 {
		t.Fatalf("sign=%d order=%d want stretching order 1", b.glueSign, b.glueOrder)
	}
	if got := packedHTotal(b); got != 10*unity {
		t.Fatalf("packed total %d sp want %d", got, 10*unity)
	}
	// The finite-stretch glue must remain unstretched (order mismatch).
	if w := b.setWidth(glueSpec{width: 0, stretch: unity, stretchOrder: 0}); w != 0 {
		t.Fatalf("finite glue got %d sp want 0", w)
	}
}

func TestGlueSetShrinkCapped(t *testing.T) {
	// Asking to shrink 5pt with only 2pt of shrink caps glueSet at 1.0.
	b := buildHBox(t, `\setbox0=\hbox to 0pt{\kern5pt\hskip0pt minus 2pt}`)
	if b.glueSign != 2 || b.glueSet != 1.0 {
		t.Fatalf("sign=%d set=%v want shrinking capped 1.0", b.glueSign, b.glueSet)
	}
}
