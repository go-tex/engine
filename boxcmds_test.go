// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// topBoxes returns the top-level *boxNode nodes of a vertical list (skipping the
// interline glue appendToPage inserts between them).
func topBoxes(nodes []node) []*boxNode {
	var out []*boxNode
	for _, n := range nodes {
		if b, ok := n.(*boxNode); ok && b != nil {
			out = append(out, b)
		}
	}
	return out
}

// leadingOffset returns the horizontal offset (sp) of the first character in a
// packed hbox: the rendered width of every glue/kern before it, under the box's
// glue-set. It measures where the content sits inside a \makebox.
func leadingOffset(b *boxNode) int {
	off := 0
	for _, n := range b.list {
		switch c := n.(type) {
		case charNode:
			return off
		case glueNode:
			off += b.setWidth(c.spec)
		case kernNode:
			off += c.width
		}
	}
	return off
}

// \bgroup and \egroup act as an implicit { and }: a box opened with \vbox\bgroup
// captures its material into the register (rather than leaking it to the page),
// and the pair opens/closes a group like braces. Real classes depend on this —
// amsart's \setbox\abstractbox=\vtop\bgroup … \egroup.
func TestBgroupEgroupImplicitBraces(t *testing.T) {
	e := New()
	if _, err := e.Run(`\setbox0=\vbox\bgroup\hbox{x}\hbox{y}\egroup`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if e.box[0] == nil {
		t.Fatal("box0 void: \\vbox\\bgroup…\\egroup did not capture into the register")
	}
	if got := len(topBoxes(e.box[0].list)); got != 2 {
		t.Errorf("box0 holds %d inner boxes, want 2 (the \\bgroup body leaked instead of being captured)", got)
	}
	// The pair scopes a definition exactly as { } would.
	e2 := New()
	if _, err := e2.Run(`\bgroup\def\gone{Y}\egroup`); err != nil {
		t.Fatalf("run2: %v", err)
	}
	if e2.eq["gone"] != nil {
		t.Error("\\def inside \\bgroup…\\egroup leaked past the group")
	}
}

// \makebox[50pt]{x} forces the box to exactly 50pt regardless of the content's
// natural 5pt width.
func TestMakeboxWidth(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\makebox[50pt]{x}`); err != nil {
		t.Fatal(err)
	}
	boxes := topBoxes(e.mvl)
	if len(boxes) != 1 {
		t.Fatalf("got %d boxes, want 1", len(boxes))
	}
	if want := 50 * unity; boxes[0].width != want {
		t.Errorf("makebox width = %d sp, want %d (50pt)", boxes[0].width, want)
	}
}

// \makebox{x} with no [width] is a natural-width hbox (the single 5pt letter).
func TestMakeboxNatural(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\makebox{x}`); err != nil {
		t.Fatal(err)
	}
	boxes := topBoxes(e.mvl)
	if len(boxes) != 1 {
		t.Fatalf("got %d boxes, want 1", len(boxes))
	}
	if want := 5 * unity; boxes[0].width != want {
		t.Errorf("natural makebox width = %d sp, want %d (5pt)", boxes[0].width, want)
	}
}

// \makebox[50pt][l]{x} left-aligns (content at offset 0, fil to the right) while
// [r] right-aligns (fil to the left pushes the 5pt letter to offset 45pt).
func TestMakeboxAlignLeftRight(t *testing.T) {
	for _, tc := range []struct {
		pos        string
		wantOffset int
	}{
		{"l", 0},
		{"r", 45 * unity},
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(`\makebox[50pt][` + tc.pos + `]{x}`); err != nil {
			t.Fatalf("[%s]: %v", tc.pos, err)
		}
		boxes := topBoxes(e.mvl)
		if len(boxes) != 1 {
			t.Fatalf("[%s]: got %d boxes, want 1", tc.pos, len(boxes))
		}
		b := boxes[0]
		if want := 50 * unity; b.width != want {
			t.Errorf("[%s]: width = %d sp, want %d", tc.pos, b.width, want)
		}
		if off := leadingOffset(b); off != tc.wantOffset {
			t.Errorf("[%s]: content offset = %d sp, want %d", tc.pos, off, tc.wantOffset)
		}
	}
}

// \makebox[50pt][c]{x} (the default) centres: a 45pt gap split as fil on both
// sides puts the letter at offset 22.5pt.
func TestMakeboxAlignCenter(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\makebox[50pt][c]{x}`); err != nil {
		t.Fatal(err)
	}
	b := topBoxes(e.mvl)[0]
	if want := 45 * unity / 2; leadingOffset(b) != want {
		t.Errorf("centred offset = %d sp, want %d", leadingOffset(b), want)
	}
}

// \raisebox{5pt}{x} raises the content box by 5pt: shift is positive-downward, so
// a raise stores shift = -5pt.
func TestRaiseboxShift(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\raisebox{5pt}{x}`); err != nil {
		t.Fatal(err)
	}
	b := topBoxes(e.mvl)[0]
	if want := -5 * unity; b.shift != want {
		t.Errorf("raisebox shift = %d sp, want %d (raised 5pt)", b.shift, want)
	}
}

// \raisebox{-4pt}{x} lowers the box: a negative lift stores a positive shift.
func TestRaiseboxLower(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\raisebox{-4pt}{x}`); err != nil {
		t.Fatal(err)
	}
	b := topBoxes(e.mvl)[0]
	if want := 4 * unity; b.shift != want {
		t.Errorf("lowered shift = %d sp, want %d", b.shift, want)
	}
}

// \raisebox{2pt}[10pt][3pt]{x} raises by 2pt and overrides the reported height and
// depth to 10pt/3pt regardless of the letter's own 7pt/2pt metrics.
func TestRaiseboxHeightDepthOverride(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\raisebox{2pt}[10pt][3pt]{x}`); err != nil {
		t.Fatal(err)
	}
	b := topBoxes(e.mvl)[0]
	if want := -2 * unity; b.shift != want {
		t.Errorf("shift = %d sp, want %d", b.shift, want)
	}
	if want := 10 * unity; b.height != want {
		t.Errorf("overridden height = %d sp, want %d (10pt)", b.height, want)
	}
	if want := 3 * unity; b.depth != want {
		t.Errorf("overridden depth = %d sp, want %d (3pt)", b.depth, want)
	}
}

// \newsavebox+\sbox+\usebox round-trips stored content: the used box has the
// stored 10pt width, two \usebox calls each yield a distinct copy, and the
// register survives both (it is copied, not consumed).
func TestSaveboxRoundTrip(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\newsavebox{\mybox}\sbox{\mybox}{ab}\usebox{\mybox}\usebox{\mybox}`); err != nil {
		t.Fatal(err)
	}
	boxes := topBoxes(e.mvl)
	if len(boxes) != 2 {
		t.Fatalf("got %d used boxes, want 2 (register reused)", len(boxes))
	}
	for i, b := range boxes {
		if want := 10 * unity; b.width != want {
			t.Errorf("usebox %d width = %d sp, want %d (stored \"ab\")", i, b.width, want)
		}
	}
	if boxes[0] == boxes[1] {
		t.Error("the two \\usebox results share a pointer; want independent copies")
	}
}

// \savebox{\name}[w][pos]{…} stores a \makebox: the used box carries the forced
// 60pt width.
func TestSaveboxMakebox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\newsavebox{\wb}\savebox{\wb}[60pt][r]{x}\usebox{\wb}`); err != nil {
		t.Fatal(err)
	}
	boxes := topBoxes(e.mvl)
	if len(boxes) != 1 {
		t.Fatalf("got %d boxes, want 1", len(boxes))
	}
	if want := 60 * unity; boxes[0].width != want {
		t.Errorf("saved makebox width = %d sp, want %d", boxes[0].width, want)
	}
}

// \newsavebox of an already-allocated handle keeps its register (and its stored
// content) rather than re-allocating.
func TestNewsaveboxReuseHandle(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\newsavebox{\r}\sbox{\r}{ab}\newsavebox{\r}\usebox{\r}`); err != nil {
		t.Fatal(err)
	}
	boxes := topBoxes(e.mvl)
	if len(boxes) != 1 || boxes[0].width != 10*unity {
		t.Fatalf("reused handle lost its content: %#v", boxes)
	}
}

// Each \newsavebox hands out a distinct register, so two saved boxes coexist.
func TestNewsaveboxDistinctRegisters(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\newsavebox{\a}\newsavebox{\b}\sbox{\a}{a}\sbox{\b}{ab}\usebox{\a}\usebox{\b}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	boxes := topBoxes(e.mvl)
	if len(boxes) != 2 {
		t.Fatalf("got %d boxes, want 2", len(boxes))
	}
	if boxes[0].width != 5*unity || boxes[1].width != 10*unity {
		t.Errorf("widths = %d,%d sp, want 5pt,10pt (distinct registers)", boxes[0].width, boxes[1].width)
	}
}

// A \makebox/\raisebox/\usebox composes inside an \hbox via boxNodeFor: the outer
// hbox's width is the sum of its inner boxes' widths.
func TestBoxCmdsInsideHbox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\newsavebox{\s}\sbox{\s}{a}\setbox0=\hbox{\makebox[20pt]{x}\raisebox{3pt}{y}\usebox{\s}}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	b := e.getBox(0)
	if b == nil {
		t.Fatal("box 0 void")
	}
	// 20pt (makebox) + 5pt (raisebox y) + 5pt (usebox a) = 30pt.
	if want := 30 * unity; b.width != want {
		t.Errorf("hbox width = %d sp, want %d", b.width, want)
	}
	// The raised inner box must be present with shift -3pt.
	var raised bool
	for _, n := range b.list {
		if ib, ok := n.(*boxNode); ok && ib.shift == -3*unity {
			raised = true
		}
	}
	if !raised {
		t.Error("no inner box raised by 3pt found in the hbox")
	}
}

// Error branches must not panic: a missing content group, a \usebox of an
// undeclared handle, a \usebox of a declared-but-void register, and a malformed
// [width] all fail gracefully.
func TestBoxCmdsErrorBranches(t *testing.T) {
	for _, src := range []string{
		`\newsavebox{\b}\sbox{\b}`,   // \sbox with no content group
		`\usebox{\undefinedhandle}`,  // handle that is not an mBoxRef
		`\newsavebox{\v}\usebox{\v}`, // declared but void register
		`\makebox[zzz]{y}`,           // malformed [width]
		`\raisebox{2pt}[qq]{y}`,      // malformed optional [height]
		`\usebox{}`,                  // empty handle group
		`\sbox`,                      // \sbox at end of input
		`\newsavebox`,                // \newsavebox at end of input
		`\newsavebox{\t}\sbox{\t`,    // handle group truncated at end of input
		`\usebox x`,                  // non-brace token where a {handle} was expected
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			// An error is acceptable for malformed input; a panic is not.
			t.Logf("%q -> %v", src, err)
		}
	}
}

// Once every \box register is allocated, a further \newsavebox is a no-op (the
// handle stays undefined) rather than panicking.
func TestNewsaveboxExhausted(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	var src string
	// allocBox starts at 10; 246 allocations exhaust registers 10..255.
	for i := 0; i < 246; i++ {
		src += `\newsavebox{\z` + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune('a'+i/676)) + `}`
	}
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if e.allocBox != 256 {
		t.Fatalf("allocBox = %d, want 256 (registers exhausted)", e.allocBox)
	}
	// One more allocation must not advance the counter or panic.
	if _, err := e.Run(`\newsavebox{\overflow}`); err != nil {
		t.Fatal(err)
	}
	if e.allocBox != 256 {
		t.Errorf("allocBox = %d after overflow, want 256", e.allocBox)
	}
	if m := e.eq["overflow"]; m != nil && m.kind == mBoxRef {
		t.Error("\\overflow was allocated a register despite exhaustion")
	}
}

// readBraceCSName returns "" when no braced group follows, and readBoxHandle then
// reports no valid handle (exercising the early-out branches directly).
func TestReadBoxHandleNoGroup(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// Prime the input with a non-brace token so getXToken returns something.
	if _, err := e.Run(`x`); err != nil {
		t.Fatal(err)
	}
	if name := e.readBraceCSName(); name != "" {
		t.Errorf("readBraceCSName = %q, want empty (no group)", name)
	}
	if _, ok := e.readBoxHandle(); ok {
		t.Error("readBoxHandle ok=true, want false (no group)")
	}
}
