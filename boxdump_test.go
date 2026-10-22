// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"strings"
)

// dumpBoxes renders a vertical list as an indented tree of boxes with their kind,
// width and the characters they carry. A width assertion that fails says only that
// SOME box is too wide; this says WHICH, and what is in it — the difference between
// "the lines merged" and "the container is packed to the outer measure", which look
// identical from a single number.
func dumpBoxes(ns []node, depth int) string {
	var b strings.Builder
	for _, n := range ns {
		bx, ok := n.(*boxNode)
		if !ok {
			continue
		}
		kind := "vbox"
		if bx.kind == hbox {
			kind = "hbox"
		}
		txt := mvlText([]node{bx})
		if len(txt) > 40 {
			txt = txt[:40] + "…"
		}
		fmt.Fprintf(&b, "%s%s w=%.2fpt %q\n", strings.Repeat("  ", depth), kind,
			float64(bx.width)/unity, txt)
		b.WriteString(dumpBoxes(bx.list, depth+1))
	}
	return b.String()
}
