// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the xparse / LaTeX3 document-command interface —
// \NewDocumentCommand and its family — which modern papers use constantly to
// declare commands with rich argument grammars (mandatory m, optional o / O{def},
// star s, delimited r/d/D, test t). Without it those declarations are undefined:
// in lenient mode the declaration is skipped and its replacement text (with its
// #1, \setbox, \rule …) spills into the document and swallows the body. The
// argument specifiers are parsed into a small descriptor list; the declared
// command becomes a primitive that, when used, grabs its arguments per the
// descriptors and pushes the replacement text with #1…#N substituted. Absent
// optional arguments become the \gotex@NoValue marker and a star becomes a boolean
// marker, both tested by \IfNoValueTF / \IfBooleanTF (also here).

// xpKind is one argument specifier's kind.
type xpKind uint8

const (
	xpMandatory xpKind = iota // m (and +m): one balanced argument
	xpOptional                // o / O{default}: a bracketed [..] argument
	xpStar                    // s: an optional leading *
	xpDelim                   // r/R/d/D: an argument delimited by a pair of tokens
	xpTest                    // t<tok>: an optional leading <tok>
)

// xpArg describes one argument of a document command.
type xpArg struct {
	kind     xpKind
	def      []tok // default value for an absent optional/delimited argument
	d1, d2   tok   // opening/closing delimiter tokens (xpDelim)
	testTok  tok   // the token an s/t specifier tests for
	required bool  // r/R (must be present) vs d/D (optional)
}

func noValueToks() []tok  { return []tok{csTok("gotex@NoValue")} }
func boolTrueToks() []tok { return []tok{csTok("gotex@BoolTrue")} }
func boolFalseToks() []tok {
	return []tok{csTok("gotex@BoolFalse")}
}

func isMarker(arg []tok, name string) bool {
	return len(arg) == 1 && arg[0].cs_ && arg[0].cs == name
}

// specReader walks a spec token list, skipping spaces.
type specReader struct {
	toks []tok
	i    int
}

func (r *specReader) nextNonSpace() (tok, bool) {
	for r.i < len(r.toks) {
		t := r.toks[r.i]
		r.i++
		if !t.cs_ && t.cat == catSpace {
			continue
		}
		return t, true
	}
	return tok{}, false
}

// nextTokRaw returns the next token without skipping spaces (a delimiter may be a
// space, though that is unusual). Used to read the one or two delimiter tokens of
// an r/d specifier.
func (r *specReader) nextTokRaw() (tok, bool) {
	if r.i < len(r.toks) {
		t := r.toks[r.i]
		r.i++
		return t, true
	}
	return tok{}, false
}

// nextGroup reads a { … } group following an O/D specifier (its default value),
// returning the group's contents. If the next non-space token is not a brace, it
// returns nil (the default is then empty).
func (r *specReader) nextGroup() []tok {
	save := r.i
	t, ok := r.nextNonSpace()
	if !ok || t.cs_ || t.cat != catBegin {
		r.i = save
		return nil
	}
	var g []tok
	depth := 1
	for r.i < len(r.toks) {
		u := r.toks[r.i]
		r.i++
		if !u.cs_ && u.cat == catBegin {
			depth++
		} else if !u.cs_ && u.cat == catEnd {
			depth--
			if depth == 0 {
				return g
			}
		}
		g = append(g, u)
	}
	return g
}

// parseXparseSpec turns an argument specification like "s m O{0.5} m O{-0.5}" into
// a descriptor list. Unknown or unsupported specifiers (e/E embellishments, v) are
// skipped as gracefully as possible.
func parseXparseSpec(spec []tok) []xpArg {
	r := &specReader{toks: spec}
	var args []xpArg
	for {
		t, ok := r.nextNonSpace()
		if !ok {
			break
		}
		if t.cs_ {
			continue // a control sequence in a spec is not something we model
		}
		switch t.ch {
		case '+', '!', '>': // long / spacing / processor prefixes — no effect here
			continue
		case 'm':
			args = append(args, xpArg{kind: xpMandatory})
		case 's':
			args = append(args, xpArg{kind: xpStar, testTok: chTok('*', catOther)})
		case 't':
			d, _ := r.nextTokRaw()
			args = append(args, xpArg{kind: xpTest, testTok: d})
		case 'o':
			args = append(args, xpArg{kind: xpOptional, def: noValueToks()})
		case 'O':
			args = append(args, xpArg{kind: xpOptional, def: r.nextGroup()})
		case 'r', 'R':
			d1, _ := r.nextTokRaw()
			d2, _ := r.nextTokRaw()
			a := xpArg{kind: xpDelim, d1: d1, d2: d2, required: true, def: noValueToks()}
			if t.ch == 'R' {
				a.def = r.nextGroup()
			}
			args = append(args, a)
		case 'd', 'D':
			d1, _ := r.nextTokRaw()
			d2, _ := r.nextTokRaw()
			a := xpArg{kind: xpDelim, d1: d1, d2: d2, def: noValueToks()}
			if t.ch == 'D' {
				a.def = r.nextGroup()
			}
			args = append(args, a)
		case 'e', 'E':
			r.nextGroup() // embellishment token list — unsupported, consume its group
		case 'v':
			args = append(args, xpArg{kind: xpMandatory}) // verbatim: best-effort as mandatory
		default:
			// unknown specifier: ignore it
		}
	}
	return args
}

// peekChar consumes and reports the next non-space token when it is the character
// ch (catcode-agnostic), otherwise leaves the input untouched.
func (e *Engine) peekChar(ch rune) bool {
	m := e.markInput()
	e.skipOptSpace()
	if t, ok := e.getNext(); ok && !t.cs_ && t.ch == ch {
		return true
	}
	e.restoreInput(m)
	return false
}

// grabXparseArgs collects the arguments described by specs from the input.
func (e *Engine) grabXparseArgs(specs []xpArg) [][]tok {
	var out [][]tok
	for _, s := range specs {
		switch s.kind {
		case xpMandatory:
			out = append(out, e.grabUndelimited())
		case xpStar:
			if e.peekChar('*') {
				out = append(out, boolTrueToks())
			} else {
				out = append(out, boolFalseToks())
			}
		case xpTest:
			if e.peekChar(s.testTok.ch) {
				out = append(out, boolTrueToks())
			} else {
				out = append(out, boolFalseToks())
			}
		case xpOptional:
			if toks, ok := e.scanOptBracketToks(); ok {
				out = append(out, toks)
			} else {
				out = append(out, s.def)
			}
		case xpDelim:
			if e.peekChar(s.d1.ch) {
				out = append(out, e.grabDelimited([]tok{s.d2}))
			} else {
				out = append(out, s.def)
			}
		}
	}
	return out
}

// substituteParams replaces #1…#N in body with the collected arguments (the same
// rule expandMacro uses for a \def body).
func substituteParams(body []tok, args [][]tok) []tok {
	var out []tok
	for _, b := range body {
		if b.cat == catParam && !b.cs_ && b.ch >= '1' && b.ch <= '9' {
			n := int(b.ch - '1')
			if n < len(args) {
				out = append(out, args[n]...)
			}
			continue
		}
		out = append(out, b)
	}
	return out
}

// xpMode is how a \…DocumentCommand declaration treats an existing definition.
type xpMode uint8

const (
	xpNew     xpMode = iota // \NewDocumentCommand: define (LaTeX warns if it exists; we just define)
	xpRenew                 // \RenewDocumentCommand
	xpProvide               // \ProvideDocumentCommand: only if undefined
	xpDeclare               // \DeclareDocumentCommand: always
)

// doDocumentCommand implements \NewDocumentCommand and its siblings: read the
// command name, the argument spec and the replacement text, and bind the command
// to a primitive that grabs its arguments and pushes the substituted body.
func (e *Engine) doDocumentCommand(mode xpMode) {
	e.peekStar() // \NewDocumentCommand* is accepted (the star has no effect here)
	name := e.scanCmdName()
	spec := e.readBraceToksRaw() // the argument specification, read literally
	body := e.readBodyGroup()    // the replacement text, with #1… as parameter tokens
	if name == "" {
		return
	}
	if mode == xpProvide && e.eq[name] != nil {
		return
	}
	specs := parseXparseSpec(spec)
	e.eq[name] = &meaning{
		kind: mPrim,
		name: "gotex@doc@" + name, // not in expandableSet: runs in the stomach, like a \protected xparse command
		prim: func(e *Engine) {
			args := e.grabXparseArgs(specs)
			e.push(substituteParams(body, args))
		},
	}
}

// doDocumentEnvironment implements \NewDocumentEnvironment{name}{spec}{begin}{end}:
// \name grabs the arguments and runs begin-code (with #1… substituted); \endname
// runs end-code. The arguments are captured on a stack at \begin so end-code —
// which may reference the same #1… — sees them too.
func (e *Engine) doDocumentEnvironment(mode xpMode) {
	e.peekStar()
	name := e.readBraceName()
	spec := e.readBraceToksRaw()
	begin := e.readBodyGroup()
	end := e.readBodyGroup()
	if name == "" {
		return
	}
	if mode == xpProvide && e.eq[name] != nil {
		return
	}
	specs := parseXparseSpec(spec)
	e.eq[name] = &meaning{
		kind: mPrim, name: "gotex@docenv@" + name,
		prim: func(e *Engine) {
			args := e.grabXparseArgs(specs)
			e.xpEnvArgs = append(e.xpEnvArgs, args)
			e.push(substituteParams(begin, args))
		},
	}
	e.eq["end"+name] = &meaning{
		kind: mPrim, name: "gotex@docenvend@" + name,
		prim: func(e *Engine) {
			var args [][]tok
			if n := len(e.xpEnvArgs); n > 0 {
				args = e.xpEnvArgs[n-1]
				e.xpEnvArgs = e.xpEnvArgs[:n-1]
			}
			e.push(substituteParams(end, args))
		},
	}
}

// loadXparse installs the document-command interface and its argument tests.
func (e *Engine) loadXparse() {
	e.prim("NewDocumentCommand", func(e *Engine) { e.doDocumentCommand(xpNew) })
	e.prim("RenewDocumentCommand", func(e *Engine) { e.doDocumentCommand(xpRenew) })
	e.prim("ProvideDocumentCommand", func(e *Engine) { e.doDocumentCommand(xpProvide) })
	e.prim("DeclareDocumentCommand", func(e *Engine) { e.doDocumentCommand(xpDeclare) })
	e.prim("NewDocumentEnvironment", func(e *Engine) { e.doDocumentEnvironment(xpNew) })
	e.prim("RenewDocumentEnvironment", func(e *Engine) { e.doDocumentEnvironment(xpRenew) })
	e.prim("ProvideDocumentEnvironment", func(e *Engine) { e.doDocumentEnvironment(xpProvide) })
	e.prim("DeclareDocumentEnvironment", func(e *Engine) { e.doDocumentEnvironment(xpDeclare) })

	// The argument tests. \IfNoValueTF{arg}{true}{false} runs true when arg is the
	// \gotex@NoValue marker an absent optional/delimited argument carries;
	// \IfBooleanTF{arg}{true}{false} runs true when arg is the \gotex@BoolTrue an
	// s/t specifier sets. These are expandable (registered in expandableSet).
	e.prim("IfNoValueTF", func(e *Engine) { e.xpIfMarker("gotex@NoValue", true, true) })
	e.prim("IfNoValueT", func(e *Engine) { e.xpIfMarker("gotex@NoValue", true, false) })
	e.prim("IfNoValueF", func(e *Engine) { e.xpIfMarker("gotex@NoValue", false, false) })
	e.prim("IfValueTF", func(e *Engine) { e.xpIfMarker("gotex@NoValue", false, true) })
	e.prim("IfValueT", func(e *Engine) { e.xpIfMarker("gotex@NoValue", false, false) })
	e.prim("IfValueF", func(e *Engine) { e.xpIfMarker("gotex@NoValue", true, false) })
	e.prim("IfBooleanTF", func(e *Engine) { e.xpIfMarker("gotex@BoolTrue", true, true) })
	e.prim("IfBooleanT", func(e *Engine) { e.xpIfMarker("gotex@BoolTrue", true, false) })
	e.prim("IfBooleanF", func(e *Engine) { e.xpIfMarker("gotex@BoolTrue", false, false) })

	// The markers themselves expand to nothing if they ever reach the page.
	e.eq["gotex@NoValue"] = &meaning{kind: mMacro}
	e.eq["gotex@BoolTrue"] = &meaning{kind: mMacro}
	e.eq["gotex@BoolFalse"] = &meaning{kind: mMacro}
	e.eq["BooleanTrue"] = &meaning{kind: mMacro, body: []tok{csTok("gotex@BoolTrue")}}
	e.eq["BooleanFalse"] = &meaning{kind: mMacro, body: []tok{csTok("gotex@BoolFalse")}}
}

// xpIfMarker implements the \If(No)Value / \IfBoolean tests. It reads the argument
// and the branch group(s), then pushes the first branch when the "true" condition
// holds. runTWhenMarker says whether that condition is "arg IS the marker" (the
// \IfNoValueTF / \IfBooleanTF sense) or its negation (the \IfValueTF sense). two
// says whether a second (false) branch group follows — the TF forms have one, the
// T / F forms do not (and then nothing is pushed when the condition fails).
func (e *Engine) xpIfMarker(marker string, runTWhenMarker, two bool) {
	arg := e.grabUndelimited()
	tbranch := e.grabUndelimited()
	var fbranch []tok
	if two {
		fbranch = e.grabUndelimited()
	}
	if isMarker(arg, marker) == runTWhenMarker {
		e.push(tbranch)
	} else if two {
		e.push(fbranch)
	}
}
