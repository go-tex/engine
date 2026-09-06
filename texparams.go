// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file gives the engine TeX's named parameters — \tracinglostchars,
// \hbadness, \spaceskip, \maxdepth and the rest of that long list. They are not
// features anyone asks for by name, but a real package sets them constantly
// (around a measurement it does not want warnings from, or to neutralise a
// setting it must not inherit), and a single undefined one halts a package that
// would otherwise run.
//
// Each is a register of its own type, so it behaves like the parameter it stands
// for: assignable with or without =, readable with \the, scoped and restored by
// grouping, and arithmetic (\advance, \multiply) applies. The engine does not yet
// act on most of them — it has its own line-breaking tolerances and spacing — so
// what a package writes is what it reads back, and nothing is silently dropped.
// A parameter the engine *does* act on (\parindent, \hsize, \baselineskip, …) is
// a primitive of its own elsewhere and is deliberately absent from these lists.

// texIntParams are TeX's integer parameters, with the values plain TeX starts
// from.
var texIntParams = map[string]int{
	"tracingcommands": 0, "tracingmacros": 0, "tracingonline": 0, "tracingoutput": 0,
	"tracingparagraphs": 0, "tracingpages": 0, "tracingrestores": 0, "tracingstats": 0,
	"tracinglostchars": 0, "showboxbreadth": -1, "showboxdepth": -1,
	"errorcontextlines": -1, "escapechar": '\\', "endlinechar": 13, "newlinechar": -1,
	"globaldefs": 0, "mag": 1000, "language": 0, "lefthyphenmin": 2, "righthyphenmin": 3,
	"uchyph": 1, "time": 0, "day": 0, "month": 0, "year": 0,
	"hbadness": 0, "vbadness": 0, "pretolerance": 100, "tolerance": 200, "looseness": 0,
	"linepenalty": 10, "hyphenpenalty": 50, "exhyphenpenalty": 50,
	"adjdemerits": 10000, "doublehyphendemerits": 10000, "finalhyphendemerits": 5000,
	"widowpenalty": 150, "clubpenalty": 150, "brokenpenalty": 100,
	"interlinepenalty": 0, "displaywidowpenalty": 50, "predisplaypenalty": 10000,
	// Two integer parameters the table lacked. Undefined, the command was skipped
	// and its VALUE stayed in the input: IEEEtran writes
	// \interdisplaylinepenalty=10000 and \interfootnotelinepenalty=2500 in its
	// preamble, and "=10000 =2500" was typeset there — on a page of its own.
	"interdisplaylinepenalty": 100, "interfootnotelinepenalty": 100,
	"postdisplaypenalty": 0, "floatingpenalty": 0, "outputpenalty": 0,
	"binoppenalty": 700, "relpenalty": 500, "delimiterfactor": 901,
	"maxdeadcycles": 25, "deadcycles": 0, "insertpenalties": 0, "badness": 0,
	"defaulthyphenchar": -1, "defaultskewchar": -1, "holdinginserts": 0,
	"fam": -1, "spacefactor": 1000, "prevgraf": 0, "hangafter": 1,
	"errorstopmode": 0, "pausing": 0,
}

// maxDimenSP is TeX's \maxdimen (16383.99998pt) in scaled points: 2^30 − 1.
const maxDimenSP = 1073741823

// texDimenParams are TeX's dimension parameters.
var texDimenParams = []string{
	"maxdepth", "splitmaxdepth", "boxmaxdepth", "delimitershortfall",
	"nulldelimiterspace", "scriptspace", "mathsurround", "predisplaysize",
	"displaywidth", "displayindent", "overfullrule", "hangindent",
	"emergencystretch", "lineskiplimit", "hoffset", "voffset",
	"pagegoal", "pagetotal", "pagestretch", "pagefilstretch", "pagefillstretch",
	"pagefilllstretch", "pageshrink", "pagedepth", "prevdepth",
}

// texGlueParams are TeX's glue parameters. Inter-word spacing comes from the font
// here, so these hold what a package sets without changing how text is set.
var texGlueParams = []string{
	"spaceskip", "xspaceskip", "parfillskip", "lineskip", "topskip",
	"splittopskip", "abovedisplayskip", "belowdisplayskip",
	"abovedisplayshortskip", "belowdisplayshortskip", "parskip", "tabskip",
	"thinmuskip", "medmuskip", "thickmuskip",
}

// loadTeXParams allocates a register for every named parameter and binds it. A
// name already defined (a parameter the engine models itself, or one a macro
// layer has claimed) is left alone.
func (e *Engine) loadTeXParams() {
	e.endlineReg = -1
	for name, def := range texIntParams {
		if e.eq[name] != nil || e.allocCnt >= 256 {
			continue
		}
		e.count[e.allocCnt] = def
		e.define(name, &meaning{kind: mCountRef, code: e.allocCnt}, true)
		e.allocCnt++
	}
	// The mouth reads \endlinechar at every line end (see mouthChar); cache the
	// register it landed on rather than looking the name up each time.
	if m := e.eq["endlinechar"]; m != nil && m.kind == mCountRef {
		e.endlineReg = m.code
	}
	// \escapechar is read whenever a control-sequence NAME is printed; cache it
	// the same way (see escapeStr).
	if m := e.eq["escapechar"]; m != nil && m.kind == mCountRef {
		e.escapeReg = m.code
	}
	for _, name := range texDimenParams {
		if e.eq[name] != nil || e.allocDim >= 256 {
			continue
		}
		// \pagegoal is \maxdimen while no page is being built — TeX's value for an
		// empty page. The engine gathers the whole document into one vertical list
		// and splits it into pages at the very end, so no output routine ever runs
		// and, from a class's point of view, the page never fills. Leaving \pagegoal
		// at 0 makes a class that sizes a title/box against the "free space left on
		// the page" (WileyNJD's \ComputeFreeSpaceOnPage) think there is NO room and
		// enter a \vsplit-to-fit \loop that never terminates — 142841 near-empty
		// pages. Initialise it to \maxdimen so such code sees a full, empty page.
		if name == "pagegoal" {
			e.dimen[e.allocDim] = maxDimenSP
		}
		e.define(name, &meaning{kind: mDimenRef, code: e.allocDim}, true)
		e.allocDim++
	}
	for _, name := range texGlueParams {
		if e.eq[name] != nil || e.allocSkp >= 256 {
			continue
		}
		// \parfillskip is the one of these with a non-zero value in every format:
		// 0pt plus 1fil (latex.ltx:546, and plain.tex the same). It closes every
		// paragraph (tex.web:16084), so a register left at zero justifies the last
		// line of every paragraph to the full measure. Allocated and never set is
		// exactly how it stood while the paragraph builder had the value written
		// into it and never read the register at all.
		if name == "parfillskip" {
			e.skip[e.allocSkp] = glueSpec{stretch: unity, stretchOrder: 1}
		}
		e.define(name, &meaning{kind: mSkipRef, code: e.allocSkp}, true)
		e.allocSkp++
	}
}
