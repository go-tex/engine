// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \lastbox is tex.web:20965-20991 (`last_box_code`): the box at the END of the
// current list, REMOVED from it, or void.
//
//	begin cur_box:=null;
//	if abs(mode)=mmode then
//	  begin you_cant; help1("Sorry; this \lastbox will be void."); error; end
//	else if (mode=vmode)and(head=tail) then
//	  begin you_cant;
//	  help2("Sorry...I usually can't take things from the current page.")
//	    ("This \lastbox will therefore be void."); error; end
//	else  begin if not is_char_node(tail) then
//	    if (type(tail)=hlist_node)or(type(tail)=vlist_node) then
//	      <Remove the last box, unless it's part of a discretionary>;
//	  end;
//	end
//
// and §20981-20991 for the removal itself:
//
//	cur_box:=tail; shift_amount(cur_box):=0;
//	tail:=p; link(p):=null;
//
// ⛔ IT REMOVES. \lastbox is not a reader: the list is one node shorter
// afterwards, and that is the whole point of the kernel's use of it.
// ltlists.dtx:818-820 is \@doendpe,
//
//	\everypar{{\setbox\z@\lastbox}\everypar{}\@endpefalse}
//
// which \endtrivlist installs so that the paragraph after a list environment is
// NOT indented: the paragraph has just begun, its list holds exactly one node —
// the \parindent box — and \lastbox takes it away and drops it on the floor. A
// \lastbox that removed nothing left that box in place, and measured against
// tectonic every paragraph after every itemize, enumerate and quote began
// 14.95pt (one \parindent) further right than the reference's. The kernel also
// reaches for it in ltoutput.dtx:1710, ltsect.dtx, ltpara.dtx and ltmath.dtx,
// and go-tex/engine#430 counts 26 of 200 arXiv papers writing it themselves.
//
// ⚠ Wiring \@doendpe into this engine's list environments is NOT done here and
// is what actually moves those paragraphs; this is the primitive they will need.
func (e *Engine) takeLastBox() *boxNode {
	list := e.currentList()
	// ⛔⛔ THE OUTER VERTICAL LIST IS REFUSED, AND THE LITERAL TEST WOULD NOT DO
	// IT. tex.web asks `(mode=vmode)and(head=tail)` — outer vertical mode with an
	// EMPTY list — because in TeX the page builder has already moved every
	// completed line onto the current page, so there is nothing left to take and
	// §20972 says as much: "I usually can't take things from the current page."
	//
	// This engine does not work that way. It gathers the whole document into one
	// vertical list and splits it into pages at the very end (see
	// texparams.go:126-136, where the same architecture is why \pagegoal has to
	// be initialised to \maxdimen). So e.mvl is NOT empty mid-document, the
	// literal test would pass, and \lastbox in outer vertical mode would delete a
	// real line of the document — silently, since the primitive returns a value
	// rather than an error.
	//
	// Porting the REASON rather than the three tokens: refuse only when we are
	// really on the page. A box being built and a paragraph in progress are both
	// still open, which is why \@doendpe — running inside \everypar, in
	// horizontal mode — keeps working.
	//
	// ⛔⛔ AND "THE CURRENT LIST IS e.mvl" IS NOT THAT TEST. buildVBoxList SWAPS
	// e.mvl for the \vbox's own list, precisely so the page machinery fills the
	// box; inside a \vbox the current list IS e.mvl and must be served. A guard
	// written `list == &e.mvl` passes every test that uses the main vertical list
	// and silently refuses \lastbox in a \vbox — which is the case LaTeX's
	// \@doendpe and ltoutput.dtx:1710 actually use. e.internalVertical is the
	// discriminator, and it exists for this.
	if list == &e.mvl && e.internalVertical == 0 {
		return nil
	}
	if len(*list) == 0 {
		return nil
	}
	// §20975-20977: only an hlist or a vlist ends the search. A char, a glue, a
	// kern, a penalty or a rule at the tail yields a VOID box and removes
	// nothing — the type guard is the same one deleteLast documents for \unskip.
	b, ok := (*list)[len(*list)-1].(*boxNode)
	if !ok || b == nil {
		return nil
	}
	*list = (*list)[:len(*list)-1]
	// §20990, shift_amount(cur_box):=0. The shift belonged to the box's place in
	// the list it has just left, not to the box; a caller that puts it back
	// somewhere else must not inherit it. Copied rather than mutated in place,
	// so nothing else holding this box sees the change.
	//
	// ⚠ boxNode.shift is not populated yet in this engine (\raise/\moveleft are
	// listed as future work where it is declared), so this line asserts nothing
	// today. It is here because the day that field starts carrying a value is not
	// the day anyone will remember to come back for it.
	c := *b
	c.shift = 0
	return &c
}

// \unskip and friends are the neighbouring surgery, and deleteLast carries the
// discretionary note that applies here too: tex.web walks the list to find the
// tail's predecessor and declines when the tail belongs to a discretionary's
// replace list (§20984-20987). Our discNode keeps its pre-break text as a
// string rather than as following nodes, so no node in these lists can be owned
// by a discretionary and that guard has nothing to test.
