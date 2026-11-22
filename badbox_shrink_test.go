// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ A LINE THAT SHRINKS TO FIT IS NOT OVERFULL. tex.web §664 calls an hbox
// overfull only when `-x - total_shrink[normal] > hfuzz`: the natural width
// past the measure, minus all the finite shrink the line can give. The check
// compared the natural width alone, so a line TeX sets at glue ratio -0.9 was
// reported as sticking out.
//
// Witness g1 (unblock-outliers-2.md): one justified article paragraph.
// tectonic reports no overfull box; the engine reported 3, the worst 8.61pt.
func TestShrinkableLineIsNotOverfull(t *testing.T) {
	src := `\documentclass{article}\begin{document}\noindent ` +
		`Optimal experimental design is the general formalism of sensor placement and decisions about the data collection strategy for engineered or natural experiments. This problem is prevalent in many critical fields such as battery design, numerical weather prediction, geosciences, and environmental and urban studies. State-of-the-art computational methods for experimental design, however, do not accommodate correlation structure in observational errors produced by many expensive-to-operate devices such as X-ray machines, radars, and satellites. Discarding evident data correlations leads to biased results, higher expenses, and waste of valuable resources. We present a general formulation of the design problem for Bayesian inverse problems, which accommodates correlated observation errors and is suitable for large problems governed by expensive simulation models with many parameters and many candidate sensor locations in space and in time.` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	d := e.Diagnostics()
	if d.OverfullLines != 0 {
		t.Errorf("%d overfull line(s), worst %.2fpt; tectonic reports none", d.OverfullLines, d.WorstOverfull)
	}
}
