package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// A package written after 2020 asks the format how old it is before it does
// anything else, and takes a completely different code path on the answer.
// etoolbox ends with \IfFormatAtLeastTF{2020-10-01}{…\endinput}{…}: the modern
// branch asks the format for hooks, the old one PATCHES the kernel's \document
// and \enddocument. With no \IfFormatAtLeastTF the engine fell through to the old
// branch, whose patches cannot match this kernel — they failed, their code leaked
// onto the page, and the rest of the document was swallowed.
//
// The date arithmetic is the kernel's own, and both spellings of a LaTeX date are
// read, because the format switched from slashes to dashes and package code still
// uses both. Values checked against real TeX, boundary included: a date is "at
// least" itself.
func TestFormatVersionComparison(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"at-least-itself", `\makeatletter\message{[\@ifl@t@r{1994/06/01}{1994/06/01}{Y}{N}]}`, "[Y]"},
		{"earlier-is-not", `\makeatletter\message{[\@ifl@t@r{1994/06/01}{1994/06/02}{Y}{N}]}`, "[N]"},
		{"later-is", `\makeatletter\message{[\@ifl@t@r{1994/07/01}{1994/06/02}{Y}{N}]}`, "[Y]"},
		{"dashes", `\makeatletter\message{[\@ifl@t@r{2021-11-15}{2020-10-01}{Y}{N}]}`, "[Y]"},
		{"mixed-spellings", `\makeatletter\message{[\@ifl@t@r{2021-11-15}{1994/06/01}{Y}{N}]}`, "[Y]"},
		// The parse itself: both spellings collapse to one comparable number. The
		// TRAILING SPACE is the real kernel's and is load-bearing — it is what ends
		// the <number> scan of the \ifnum that reads this expansion.
		{"parse-dashes", `\makeatletter\message{[\@parse@version 2020-10-01//00\@nil]}`, "[20201001 ]"},
		{"parse-slashes", `\makeatletter\message{[\@parse@version 1994/06/01//00\@nil]}`, "[19940601 ]"},
		// The engine offers the 2020-10-01 interfaces and says so.
		{"engine-answers-yes", `\message{[\IfFormatAtLeastTF{2020-10-01}{Y}{N}]}`, "[Y]"},
		{"engine-answers-no-to-later", `\message{[\IfFormatAtLeastTF{2999-01-01}{Y}{N}]}`, "[N]"},
		{"T-form", `\message{[\IfFormatAtLeastT{2020-10-01}{Y}]}`, "[Y]"},
		{"F-form", `\message{[\IfFormatAtLeastF{2999-01-01}{N}]}`, "[N]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// A hook is a named token list the format runs at a defined moment. Code is
// appended to it, possibly before the hook exists, and \UseHook runs it.
func TestHooksCollectAndRun(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"add-then-use", `\AddToHook{myhook}{\message{A}}\UseHook{myhook}`, "A"},
		{"order-of-addition", `\AddToHook{h}{\message{1}}\AddToHook{h}{\message{2}}\UseHook{h}`, "1 2"},
		{"declared-first", `\NewHook{h}\AddToHook{h}{\message{A}}\UseHook{h}`, "A"},
		// A label is read (so it cannot reach the page) and then ignored.
		{"label-is-consumed", `\AddToHook{h}[mypkg]{\message{A}}\UseHook{h}`, "A"},
		// An unused hook runs nothing; using a hook nobody wrote to is a no-op.
		{"unused-hook-is-silent", `\AddToHook{h}{\message{A}}\message{B}`, "B"},
		{"unknown-hook-is-a-no-op", `\UseHook{nobodys}\message{B}`, "B"},
		// \AddToHookNext fires once, then the hook is back to its standing content.
		{"next-fires-once", `\AddToHook{h}{\message{S}}\AddToHookNext{h}{\message{N}}\UseHook{h}\UseHook{h}`, "S N S"},
		{"remove-empties-the-hook", `\AddToHook{h}{\message{A}}\RemoveFromHook{h}\UseHook{h}\message{B}`, "B"},
		{"empty-test", `\message{[\IfHookEmptyTF{h}{E}{F}]}`, "[E]"},
		{"non-empty-test", `\AddToHook{h}{\message{A}}\message{[\IfHookEmptyTF{h}{E}{F}]}`, "[F]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// The document hooks are wired to the moments they name. \AtBeginDocument keeps
// working alongside them, and the order is the format's: begindocument/before,
// then the \AtBeginDocument code, then begindocument/end, then the document
// environment's own env/document/begin.
func TestDocumentHooksFireInOrder(t *testing.T) {
	const src = `\AddToHook{begindocument/before}{\message{1}}` +
		`\AtBeginDocument{\message{2}}` +
		`\AddToHook{begindocument/end}{\message{3}}` +
		`\AddToHook{env/document/begin}{\message{4}}` +
		`\AddToHook{enddocument}{\message{5}}` +
		`\begin{document}\end{document}`
	if got := ckRun(t, src); got != "1 2 3 4 5" {
		t.Errorf("document hooks = %q, want %q", got, "1 2 3 4 5")
	}
}

// A package can register code to run around ANOTHER file's loading — beamer's
// overlay layer does exactly this with package/amsmath/after. The loader fires
// the four hooks that name a file being loaded.
func TestPackageAndFileHooksFireAroundALoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "probe.sty"), []byte(`\message{BODY}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	const src = `\AddToHook{package/probe/before}{\message{PB}}` +
		`\AddToHook{file/probe.sty/before}{\message{FB}}` +
		`\AddToHook{package/probe/after}{\message{PA}}` +
		`\AddToHook{file/probe.sty/after}{\message{FA}}` +
		`\usepackage{probe}`
	if got := ckRun(t, src); got != "FB PB BODY PA FA" {
		t.Errorf("hooks around a package load = %q, want %q", got, "FB PB BODY PA FA")
	}
}

// \RequirePackage{keyval}[1997/11/10] names the oldest acceptable release of the
// package. The engine loads whatever it finds, but the date must be CONSUMED:
// left behind it is typeset, and beamer's title page carried a stray
// "[1997/11/10]".
//
// It is eaten AFTER the file, where the date is the next thing in the input.
// Reading it before splicing the file put the token that is NOT a "[" back onto
// the token stack, ahead of the file about to be spliced into the character
// buffer — which silently swallowed everything after the \usepackage. Both halves
// are checked here.
func TestOptionalDateAfterAPackageName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dated.sty"), []byte(`\message{LOADED}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	cases := []struct{ name, src, want string }{
		{"date-is-eaten", `\usepackage{dated}[1997/11/10]\message{AFTER}`, "LOADED AFTER"},
		{"no-date-loses-nothing", `\usepackage{dated}\message{AFTER}`, "LOADED AFTER"},
		{"date-after-a-class", `\documentclass{dated}[1997/11/10]\message{AFTER}`, "AFTER"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}
