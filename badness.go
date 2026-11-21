// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// TeX's three badness landmarks, on ONE scale and with their own names, because the
// page builder's cost rule distinguishes all three (tex.web:19556-19560) and a
// literal 10000 in that rule reads like a penalty.
//
//	@d inf_bad = 10000                    {infinitely bad value}      (tex.web:2335)
//	@d deplorable==100000                 {more than |inf_bad|, but
//	                                       less than |awful_bad|}     (tex.web:18983)
//	@d awful_bad==@'7777777777            {more than a billion demerits} (:16435)
//
// 0o7777777777 is 2³⁰−1, not the decimal that octal looks like.
const (
	infBad     = 10000
	deplorable = 100000
	awfulBad   = 0o7777777777
)

// badness(t, s) is tex.web §108's, and lives in go-typeset/linebreak
// (TeXBadness, libs.go): the page builder and the line breaker share ONE
// implementation. The three branches of §108 are overflow guards, not
// approximations, and s <= 0 is inf_bad, not an error; its doc comment there
// says why.
