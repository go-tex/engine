// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \NewDocumentCommand with the common specifiers (star, optional-with-default,
// mandatory) grabs each argument correctly and \IfBooleanTF / \IfNoValueTF test
// the resulting markers. Each call is probed with \message so the exact expansion
// is checked, not just that something rendered.
func TestNewDocumentCommandArguments(t *testing.T) {
	src := `\NewDocumentCommand{\gr}{s O{world} m}{\message{[\IfBooleanTF{#1}{BANG}{soft}|#2|#3]}}` +
		`\gr{A}\gr*[all]{B}\gr[named]{C}`
	out, _ := runLaTeX(t, src)
	for _, want := range []string{"[soft|world|A]", "[BANG|all|B]", "[soft|named|C]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in \\message output %q", want, out)
		}
	}
}

// A plain optional argument o (no default) yields the \gotex@NoValue marker when
// absent, which \IfNoValueTF detects; \IfValueTF is its complement.
func TestNewDocumentCommandOptionalNoValue(t *testing.T) {
	src := `\NewDocumentCommand{\x}{o m}{\message{[\IfNoValueTF{#1}{none}{got:#1}|\IfValueT{#1}{V}|#2]}}` +
		`\x{p}\x[q]{r}`
	out, _ := runLaTeX(t, src)
	for _, want := range []string{"[none||p]", "[got:q|V|r]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in \\message output %q", want, out)
		}
	}
}

// The definition body of an undefined \DeclareDocumentCommand used to spill into
// the document (its #1, \setbox, \rule … typeset as text and swallowed the body).
// With xparse implemented the command is defined and the body that follows renders
// normally.
func TestDocumentCommandDefinitionDoesNotSwallowBody(t *testing.T) {
	src := `\DeclareDocumentCommand{\faktor}{s m O{0.5} m O{-0.5}}{\setbox0=\hbox{#2}\raisebox{#3pt}{#2}/#4}` +
		`BODYMARKER text after the definition.`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "BODYMARKER") {
		t.Errorf("definition swallowed the body; want BODYMARKER, got %q", got)
	}
}

// The test (t<tok>), required-delimited (r<a><b>) and optional-delimited
// (d<a><b> / D<a><b>{default}) specifiers grab as xparse specifies: a present t
// token becomes a boolean, a delimited argument is taken between its delimiters,
// and an absent optional-delimited argument falls back to its default (or the
// \gotex@NoValue marker).
func TestDocumentCommandDelimitedSpecifiers(t *testing.T) {
	src := `\NewDocumentCommand{\q}{t+ r() d[] D<>{def}}` +
		`{\message{[\IfBooleanTF{#1}{plus}{noplus}|#2|\IfNoValueTF{#3}{no3}{#3}|#4]}}` +
		`\q+(aa)[bb]<cc>` +
		`\q(dd)`
	out, _ := runLaTeX(t, src)
	for _, want := range []string{"[plus|aa|bb|cc]", "[noplus|dd|no3|def]"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in \\message output %q", want, out)
		}
	}
}

// \RenewDocumentCommand redefines; \ProvideDocumentCommand defines only when the
// name is free (an existing definition wins); and the single-branch tests
// (\IfNoValueF, \IfValueF, \IfBooleanF) run their branch on the complementary side.
func TestDocumentCommandModesAndSingleBranchTests(t *testing.T) {
	src := `\NewDocumentCommand{\a}{}{\message{A-first}}\RenewDocumentCommand{\a}{}{\message{A-second}}` +
		`\ProvideDocumentCommand{\a}{}{\message{A-third}}` + // \a already exists → keep "A-second"
		`\NewDocumentCommand{\b}{o s m}{\message{[\IfNoValueF{#1}{has1}\IfValueF{#1}{val-missing}|\IfBooleanF{#2}{noStar}|#3]}}` +
		`\b{X}\b[y]*{Z}\a`
	out, _ := runLaTeX(t, src)
	for _, want := range []string{"[val-missing|noStar|X]", "[has1||Z]", "A-second"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in \\message output %q", want, out)
		}
	}
	if strings.Contains(out, "A-third") {
		t.Errorf("\\ProvideDocumentCommand clobbered an existing definition; output %q", out)
	}
}

// \NewDocumentEnvironment grabs its arguments at \begin and makes them available
// to both the begin-code and the end-code.
func TestNewDocumentEnvironment(t *testing.T) {
	src := `\NewDocumentEnvironment{box}{O{plain} m}{\message{<begin:#1:#2>}}{\message{<end:#1:#2>}}` +
		`\begin{box}[fancy]{Title}\end{box}`
	out, _ := runLaTeX(t, src)
	for _, want := range []string{"<begin:fancy:Title>", "<end:fancy:Title>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in \\message output %q", want, out)
		}
	}
}
