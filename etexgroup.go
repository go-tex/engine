// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strconv"

// e-TeX lets a file ask where it stands in the grouping stack:
// \currentgrouplevel is how many groups are open, \currentgrouptype is what
// opened the innermost one. Both are read-only internal integers, so they are
// read by \the, by \number, and directly by \ifnum — never assigned.
//
// A package uses them to tell "am I inside the group I opened?" from "has
// someone else opened one since?", which is the difference between restoring a
// value and clobbering a caller's. pgf's own scope handling asks, and so does
// any code that has to decide whether \aftergroup will fire where it means to.
//
// The type codes are tex.web's group codes, which e-TeX exposes unchanged. The
// engine distinguishes the three kinds of group it actually keeps apart, and
// reports each with the code TeX gives it; a level opened by \hbox and one
// opened by \vbox share a frame here, so both answer with hbox's code rather
// than inventing a distinction the engine does not make.
const (
	groupTypeBottom     = 0  // no group is open
	groupTypeSimple     = 1  // { … }
	groupTypeHbox       = 2  // \hbox{ … } and, here, \vbox{ … } too
	groupTypeSemiSimple = 14 // \begingroup … \endgroup
)

// currentGroupType reports the innermost open group's tex.web group code.
func (e *Engine) currentGroupType() int {
	k, ok := e.curGroupKind()
	if !ok {
		return groupTypeBottom
	}
	switch k {
	case boxGroup:
		return groupTypeHbox
	case semiSimpleGroup:
		return groupTypeSemiSimple
	default:
		return groupTypeSimple
	}
}

// groupQuery reports the value of a read-only grouping integer, and whether the
// name is one. Keeping the two names in one place lets the number scanner, \the
// and the internal-integer test agree without repeating the mapping.
func (e *Engine) groupQuery(name string) (int, bool) {
	switch name {
	case "currentgrouplevel":
		return len(e.groups), true
	case "currentgrouptype":
		return e.currentGroupType(), true
	}
	return 0, false
}

// installGroupQueries registers the two primitives. Executing one where a number
// is not being read is an error in TeX; here it simply contributes its digits,
// which is what a file that writes \currentgrouplevel into a message expects.
func (e *Engine) installGroupQueries() {
	for _, n := range []string{"currentgrouplevel", "currentgrouptype"} {
		name := n
		e.prim(name, func(e *Engine) {
			v, _ := e.groupQuery(name)
			e.pushString(strconv.Itoa(v))
		})
	}
}

// isGroupQuery reports whether a primitive name is one of the two.
func isGroupQuery(name string) bool {
	return name == "currentgrouplevel" || name == "currentgrouptype"
}
