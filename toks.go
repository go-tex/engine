// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// Token registers (\toks). A real class file's title/mark machinery stores and
// rewrites token lists in registers: amsart's \andify builds the "A, B and C"
// author line entirely with \toks@, \@temptokena, \the\toks@ and
// \toks@\expandafter{\the\toks@ …}. The kernel-helper layer deliberately avoided
// toks registers (its \g@addto@macro uses an \expandafter chain instead), so a
// class that assigns to \toks@ used to leak its braced argument onto the page and
// its \andify recursion ran away. This file adds the register class the standard
// way — a meaning kind (mToksRef) plus the \toks / \newtoks / \toksdef primitives —
// so \toks<n>{…}, \toks<n>=\other, \the\toks<n> and \newtoks\name all work.
//
// Scope: registers are stored globally (no group save-stack), which is enough for
// the single-pass title/mark assembly a class performs; TeX would scope an
// assignment to the enclosing group, but the class code rebuilds the register from
// \@emptytoks each time, so the net result is the same.

// toksValue returns a copy of token register n (empty when unset or out of range).
// A copy is returned because the caller pushes it into the input while the same
// register may be reassigned in the same expression (\toks@\expandafter{\the\toks@…}).
func (e *Engine) toksValue(n int) []tok {
	if n < 0 || n >= len(e.toks) || e.toks[n] == nil {
		// len() is the fast path, not the legality test: a legal register above the
		// current length is simply unwritten, and nil is what it holds.
		// noteBadRegisterIndex carries the bound and returns early for anything
		// inside the pool, exactly as getCount/getDimen/getSkip rely on it to.
		e.noteBadRegisterIndex(n, `\toks`)
		return nil
	}
	return append([]tok(nil), e.toks[n]...)
}

// setToks stores a token list into register n, growing the pool to reach it.
func (e *Engine) setToks(n int, ts []tok) {
	grown, ok := growPool(e.toks, n)
	if !ok {
		e.noteBadRegisterIndex(n, `\toks`)
		return
	}
	e.toks = grown
	e.toks[n] = ts
}

// readToksValue reads the right-hand side of a token-register assignment: an
// optional '=', then either a {braced token list} (grabbed unexpanded) or another
// token register whose contents are copied. \expandafter in the RHS is honoured by
// getXToken, so \toks@\expandafter{\the\toks@ x} sees the '{' after \the\toks@ has
// expanded, i.e. it assigns the old contents followed by x.
func (e *Engine) readToksValue() []tok {
	e.scanEquals()
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return nil
	}
	if !t.cs_ && t.cat == catBegin {
		return e.grabGroup()
	}
	if t.cs_ {
		if m := e.eq[t.cs]; m != nil {
			switch {
			case m.kind == mToksRef:
				return e.toksValue(m.code)
			case m.kind == mPrim && m.name == "toks":
				return e.toksValue(e.scanInt())
			}
		}
	}
	e.back(t)
	return nil
}

// toksRefAssign handles an assignment through a \newtoks/\toksdef control sequence:
// \name{…} or \name=\other.
func (e *Engine) toksRefAssign(code int) {
	e.setToks(code, e.readToksValue())
}

// loadToksPrims registers the token-register primitives. Called from New alongside
// loadClassPrims.
func (e *Engine) loadToksPrims() {
	// \toks<n>{…} / \toks<n>=\other : the indexed token register.
	e.prim("toks", func(e *Engine) {
		n := e.scanInt()
		e.setToks(n, e.readToksValue())
	})
	// \newtoks\name : allocate the next free token register and bind \name to it.
	//
	// ⛔⛔ THE BOUND WAS len(e.toks), AND THE POOL WAS A FLAT 256. #844 raised
	// \count, \dimen, \skip and \box to eTeX's 32768 and grew them on demand;
	// `toks` was a separate [][]tok allocated in full at registerBaseSize, so the
	// 247th \newtoks (the allocator starts at 10) was refused — and refused with a
	// bare `return`: no report, exit 0, and \the on the never-defined name
	// TYPESET the control sequence instead of its contents. Measured against
	// tectonic at 7dec596, 250 \newtoks then write and read the 250th:
	//
	//	             reference   ours
	//	\toks 250    ALFA250     \zzt250
	//	\toks 1      BRAVO1      BRAVO1
	//	\count 250   4242        4242     ← the control: #844 raised that pool
	//
	// ⛔ AND Diagnostics.RegistersExhausted DOCUMENTED a `\toks` key that nothing
	// ever wrote, so a consumer was told the exhaustion was reported. Documenting
	// a key the engine cannot emit is worse than documenting none: it answers the
	// question instead of leaving it open. The key is true now.
	// go-tex/engine#849.
	e.prim("newtoks", func(e *Engine) {
		name := e.scanCSName()
		if name == "" {
			return
		}
		if e.allocToks >= registerPoolSize {
			e.noteRegisterExhausted(`\toks`)
			return
		}
		e.define(name, &meaning{kind: mToksRef, code: e.allocToks}, false)
		e.allocToks++
	})
	// \toksdef\name=<n> : bind \name to token register n (plain TeX's \toksdef\toks@=0).
	e.prim("toksdef", func(e *Engine) {
		name := e.scanCSName()
		e.scanEquals()
		n := e.scanInt()
		if name != "" {
			e.define(name, &meaning{kind: mToksRef, code: n}, false)
		}
	})
}
