// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// badness is TeX's badness(t, s): how hard glue with stretchability (or
// shrinkability) s must work to take up a shortfall (or excess) of t, as an integer
// from 0 to inf_bad.
//
// tex.web §108, transcribed:
//
//	function badness(t,s:scaled):halfword; {compute badness, given t>=0}
//	begin if t=0 then badness:=0
//	else if s<=0 then badness:=inf_bad
//	else  begin if t<=7230584 then r:=(t*297) div s
//	  else if s>=1663497 then r:=t div (s div 297)
//	  else r:=t;
//	  if r>1290 then badness:=inf_bad
//	  else badness:=(r*r*r+@'400000) div @'1000000;
//	  end;
//	end;
//
// ⛔ THE THREE BRANCHES ARE OVERFLOW GUARDS, NOT APPROXIMATIONS, and the thresholds
// are not round numbers to be tidied: 297³ = 99.94 × 2¹⁸ is the scaling that makes the
// cube come out as 100·(t/s)³, 7230584 is where t×297 would overflow, and 1290 is the
// largest r whose cube fits (1290³ < 2³¹ < 1291³). Writing this as a float
// 100*(t/s)**3 would agree on ordinary values and diverge exactly where TeX's own
// rounding decides a close page break.
//
// ⛔ s <= 0 IS inf_bad, NOT a division by zero to guard and move past: glue that cannot
// stretch gives a page that cannot be filled, and TeX says so with the worst finite
// value rather than with an error.
//
// @'400000 and @'1000000 are OCTAL in WEB — 0o400000 = 131072 = 2¹⁷ (the rounding
// half) and 0o1000000 = 262144 = 2¹⁸. Reading them as decimal is a silent factor of
// four.
func badness(t, s int) int {
	const infBad = 10000
	if t == 0 {
		return 0
	}
	if s <= 0 {
		return infBad
	}
	var r int
	switch {
	case t <= 7230584:
		r = (t * 297) / s
	case s >= 1663497:
		r = t / (s / 297)
	default:
		r = t
	}
	if r > 1290 {
		return infBad
	}
	return (r*r*r + 0o400000) / 0o1000000
}
