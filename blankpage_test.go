// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// tex.web §1000 makes a page non-empty only for an hlist, vlist or rule node. A
// whatsit is contributed without doing so, and glue, kerns and penalties reaching a
// page that holds no box are discarded — so \newpage before any box ships nothing.
// A \special in a class preamble (oupau.cls writes \special{papersize=…}) otherwise
// put a blank sheet in front of every document that class set.
func TestSpecialAloneDoesNotShipAPage(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\special{papersize=210mm,297mm}\par\penalty-10000 CORPS\par`); err != nil {
		t.Fatal(err)
	}
	if n := len(e.Pages()); n != 1 {
		t.Errorf("a \\special before \\newpage shipped a blank page: %d pages, want 1", n)
	}
}

// A box on the page makes it shippable, so a forced break after one still yields two
// pages. Without this the fix above would silently swallow legitimate blank pages.
func TestBoxBeforeForcedBreakStillShipsTwoPages(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\null\par\penalty-10000 CORPS\par`); err != nil {
		t.Fatal(err)
	}
	if n := len(e.Pages()); n != 2 {
		t.Errorf("a forced break after a box lost its page: %d pages, want 2", n)
	}
}
