// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements text-decoration commands that draw a thin rule across their
// content: \underline (a line below the baseline), \sout (a strike-out line through
// the text, as in the ulem package) and \textoverline (a line above). Each packs its
// {content} as an hbox — exactly like \mbox — and wraps it in a decoNode carrying
// the rule position. The line inherits the current colour, so \textcolor{red}{\sout{x}}
// strikes in red. Unlike TeX's \underline (a math-mode construct), these work in
// ordinary horizontal mode, which is what LaTeX documents expect from \underline.

const (
	decoGap  = unity       // gap between the text and an under/over line (1pt)
	decoRule = defaultRule // decoration line thickness (0.4pt)
)

// decoNode wraps an inner hbox with a rule. kind selects the rule's position:
// 'u' underline (below baseline), 's' strike-out (through the text), 'o' overline.
type decoNode struct {
	inner *boxNode
	kind  byte
	color uint32 // rule colour (0xRRGGBB; 0 = black / current)
}

func (decoNode) isNode() {}

func (dn decoNode) width() int { return dn.inner.width }

func (dn decoNode) height() int {
	if dn.kind == 'o' {
		return dn.inner.height + decoGap + decoRule
	}
	return dn.inner.height
}

func (dn decoNode) depth() int {
	if dn.kind == 'u' {
		if d := decoGap + decoRule; d > dn.inner.depth {
			return d
		}
	}
	return dn.inner.depth
}

// strikeY is the height above the baseline of the strike-out rule, roughly the
// x-height midline of the content.
func (dn decoNode) strikeY() int { return dn.inner.height * 28 / 100 }

// makeDeco reads a {content} group, packs it as an hbox and wraps it in a decoNode
// of the given kind, capturing the current colour for the rule.
func (e *Engine) makeDeco(kind byte) decoNode {
	list, _ := e.grabHboxList()
	return decoNode{inner: hpackSP(list, packNatural, 0), kind: kind, color: e.curColor}
}

// decoRuleTop returns the top of the decoration rule as an sp offset from the
// baseline in y-down space (positive = below the baseline), shared by both drivers.
func (dn decoNode) decoRuleTop() int {
	switch dn.kind {
	case 'u': // below the baseline
		return decoGap
	case 'o': // above the content
		return -(dn.inner.height + decoGap + decoRule)
	default: // 's' — through the text
		return -dn.strikeY()
	}
}
