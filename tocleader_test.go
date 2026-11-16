// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// An aligned leader (\leaders, as \@dottedtocline writes it) lays its tiles on a grid
// anchored to the left edge of the enclosing box, and sets a tile only if it fits
// whole (tex.web §626). The engine tiled from the glue's own start, so the first dot
// hugged the title ("Un détail. . .") and each line carried a tile too many where
// tectonic sets "Un détail . . .".
func TestDotLeaderTilesSitOnTheEnclosingBoxGrid(t *testing.T) {
	// A glue starting 20.3 into the box, 30 wide, 7.75 tiles: the grid is 0, 7.75,
	// 15.5, 23.25, 31 ..., so tiles start at 23.25 and 31 and the next, at 38.75,
	// ends at 46.5 < 50.3 — three tiles; a fourth, 46.5 + 7.75 = 54.25, does not fit.
	got := dotLeaderOrigins(0, 20.3, 30, 7.75)
	want := []float64{23.25, 31, 38.75}
	if len(got) != len(want) {
		t.Fatalf("tiles = %v, want %v", got, want)
	}
	for i := range want {
		if d := got[i] - want[i]; d > 1e-9 || d < -1e-9 {
			t.Errorf("tile %d at %g, want %g", i, got[i], want[i])
		}
	}
	// Two glues on the same grid line their dots up whatever their own starts are.
	a, b := dotLeaderOrigins(10, 12, 40, 5), dotLeaderOrigins(10, 13, 39, 5)
	if a[0] != b[0] {
		t.Errorf("tiles do not share a grid: %v against %v", a, b)
	}
	if dotLeaderOrigins(0, 5, 0, 7) != nil || dotLeaderOrigins(0, 5, 10, 0) != nil {
		t.Error("an empty glue or a zero tile must set nothing")
	}
}

// Witness testdata/parity/tocleader.tex: tectonic 0.17.0 sets 31 and 33 dots on the
// two subsection lines (gotex-linediff: leader=31 and leader=33, pitch 7.75pt); the
// engine set 32 and 34.
func TestParityTocLeaderDotCounts(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "tocleader.tex"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := os.ReadFile(filepath.Join("testdata", "parity", "tocleader.tectonic.leaders.txt"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox {
				x := 0.0
				for _, c := range b.list {
					switch v := c.(type) {
					case glueNode:
						w := spToPt(b.setWidth(v.spec))
						if v.leader == leaderDots {
							got = append(got, strconv.Itoa(len(dotLeaderOrigins(0, x, w, spToPt(v.cell)))))
						}
						x += w
					case charNode:
						x += spToPt(v.width)
					case kernNode:
						x += spToPt(v.width)
					case *boxNode:
						x += spToPt(v.width)
					}
				}
			}
			walk(b.list)
		}
	}
	for _, p := range e.Pages() {
		walk([]node{p})
	}
	if g, w := strings.Join(got, " "), strings.TrimSpace(string(ref)); g != w {
		t.Errorf("dots on the leader lines = %q, tectonic sets %q", g, w)
	}
}
