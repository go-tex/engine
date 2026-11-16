// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpus writes a two-paper corpus that carries, deliberately, every trap this
// command exists to avoid:
//
//   - paper 2001.00001 sets \looseness in an \input'ed chapter in a SUBDIRECTORY,
//     so a scan that walks only the list's entry file cannot see it;
//   - it also has a COMMENTED-OUT \emergencystretch, which is not a setting;
//   - and prose containing the bare word "looseness", which is not a setting;
//   - paper 2002.00002 sets nothing, and has an ESCAPED \% to prove that an
//     escaped percent does not start a comment.
//
// The list names each paper's main.tex, exactly as clean-list.txt does.
func corpus(t *testing.T) (listPath string, root string) {
	t.Helper()
	root = t.TempDir()
	write := func(p, s string) {
		t.Helper()
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("2001.00001/main.tex", "\\documentclass{article}\n"+
		"% \\emergencystretch=3em   a commented-out setting is not a setting\n"+
		"We discuss the looseness of the bound in prose.\n"+
		// A LONGER control word that BEGINS with \font, and no use of \font
		// itself. This is the real corpus line that exposed trap 5.
		"\\renewcommand\\Authfont{\\fontsize{13}{16}\\selectfont}\n"+
		"\\input{sections/one}\n")
	write("2001.00001/sections/one.tex", "\\looseness=-1\nSome text.\n")
	write("2002.00002/main.tex", "\\documentclass{article}\n"+
		"A literal percent, 50\\% of cases, does not start a comment: \\hangindent=1em\n"+
		// The ONLY real use of the \font primitive in this corpus.
		"\\font\\tenrm=cmr10\n")
	listPath = filepath.Join(root, "list.txt")
	if err := os.WriteFile(listPath, []byte(
		filepath.Join(root, "2001.00001", "main.tex")+"\n"+
			filepath.Join(root, "2002.00002", "main.tex")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return listPath, root
}

func TestTheCensusWalksEachPaperNotItsEntryFile(t *testing.T) {
	list, _ := corpus(t)
	var out, errb bytes.Buffer
	if code := run([]string{"-list", list, `\looseness`, `\emergencystretch`, `\hangindent`}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	got := out.String()

	// ⭐ THREE .tex FILES FROM TWO PAPERS. A scan walking the list's entries
	// would report two, which looks like a complete census of two papers.
	if !strings.Contains(got, "population: 2 paper(s) of 2 named, 3 .tex file(s)") {
		t.Errorf("population line wrong, so the scan did not reach the \\input'ed chapter:\n%s", got)
	}
	// \looseness lives only in the subdirectory chapter.
	if !strings.Contains(got, `\looseness: 1 of 2 paper(s)  2001.00001`) {
		t.Errorf("\\looseness not found in the chapter under sections/:\n%s", got)
	}
	// It is COMMENTED OUT in main.tex, so it is not set anywhere.
	if !strings.Contains(got, `\emergencystretch: 0 of 2 paper(s)`) {
		t.Errorf("a commented-out setting was counted as a setting:\n%s", got)
	}
	// An escaped \% does not start a comment, so what follows it is still code.
	if !strings.Contains(got, `\hangindent: 1 of 2 paper(s)  2002.00002`) {
		t.Errorf("an escaped percent was treated as a comment, hiding the rest of the line:\n%s", got)
	}
}

// ⭐ A CONTROL WORD DOES NOT ANSWER FOR THE LONGER ONES THAT BEGIN WITH IT.
//
// This is trap 5, and it is here because it produced an answer I believed for a
// minute: asked for `\font` over the real 154-paper list, a substring scan said
// 14 papers, and every matching line was `\fontsize{13}{16}`, `\fontsize{\sf@size}`
// or `\Affilfont` — not one use of the primitive. The corpus above carries both
// shapes on purpose, in two different papers, so the two behaviours give
// DIFFERENT counts: 1 with the boundary, 2 without it.
func TestAControlWordDoesNotAnswerForALongerOne(t *testing.T) {
	list, _ := corpus(t)
	var out, errb bytes.Buffer
	if code := run([]string{"-list", list, `\font`, `\fontsize`, `\selectfont`}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	got := out.String()

	// \font is used in 2002.00002 ONLY. 2001.00001 holds \fontsize and
	// \selectfont, neither of which is \font.
	if !strings.Contains(got, `\font: 1 of 2 paper(s)  2002.00002`) {
		t.Errorf("\\font answered for \\fontsize, so a control word still matches longer ones:\n%s", got)
	}
	// The control, without which the assertion above passes on a scan that
	// found nothing at all: the longer words ARE present, in the other paper.
	if !strings.Contains(got, `\fontsize: 1 of 2 paper(s)  2001.00001`) {
		t.Errorf("\\fontsize was not found, so this test is not measuring a boundary:\n%s", got)
	}
	// \selectfont ENDS in the term, which no boundary rule should hide.
	if !strings.Contains(got, `\selectfont: 1 of 2 paper(s)  2001.00001`) {
		t.Errorf("\\selectfont was not found:\n%s", got)
	}
}

// containsTerm's own table, including the shapes the corpus cannot carry.
func TestContainsTerm(t *testing.T) {
	for _, c := range []struct {
		line, term string
		want       bool
	}{
		{`\font\tenrm=cmr10`, `\font`, true},            // the primitive, followed by a backslash
		{`\font=x`, `\font`, true},                      // followed by =
		{`\font `, `\font`, true},                       // followed by a space
		{`x\font`, `\font`, true},                       // at end of line
		{`\fontsize{13}{16}`, `\font`, false},           // ⛔ the trap
		{`\Authfont{}`, `\font`, false},                 // no backslash before font
		{`\selectfont`, `\font`, false},                 // ends with it, is not it
		{`\fontsize{\sf@size}`, `\fontsize`, true},      // the longer word asked for itself
		{`\paragraph{Heading}`, `\par`, false},          // ⛔ \par must not answer for \paragraph
		{`text\par`, `\par`, true},                      // and must answer for itself
		{`a\\b`, `\\`, true},                            // a control SYMBOL takes no boundary
		{`50\% of cases`, `\%`, true},                   // nor does \%
		{`the looseness of it`, `looseness`, true},      // a plain word stays a substring
		{`\looseness=-1`, `\looseness`, true},           //
		{`\fontseries{b}\fontshape{n}`, `\font`, false}, // two longer words, still not \font
	} {
		if got := containsTerm(c.line, c.term); got != c.want {
			t.Errorf("containsTerm(%q, %q) = %v, want %v", c.line, c.term, got, c.want)
		}
	}
}

// ⭐ The barrier: a census that could not read must REFUSE, not report zero.
// A scan that read nothing and a corpus that holds nothing both print 0, and
// the only thing that separates them is the control.
func TestItRefusesWhenItCannotStandBehindAZero(t *testing.T) {
	list, root := corpus(t)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "the control term is absent, so no zero can be believed",
			args: []string{"-list", list, "-control", `\thiscannotbethere`, `\looseness`},
			want: "was found in NO paper",
		},
		{
			name: "no paper named",
			args: []string{"-list", filepath.Join(root, "empty.txt"), `\looseness`},
			want: "no paper named",
		},
		{
			name: "no term given",
			args: []string{"-list", list},
			want: "at least one term",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "no paper named" {
				if err := os.WriteFile(filepath.Join(root, "empty.txt"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errb bytes.Buffer
			code := run(tc.args, &out, &errb)
			if code != 2 {
				t.Errorf("exit %d, want 2 (refused); stdout %q stderr %q", code, out.String(), errb.String())
			}
			if !strings.Contains(errb.String(), tc.want) {
				t.Errorf("stderr %q does not say %q", errb.String(), tc.want)
			}
		})
	}
}

// A paper whose directory holds no .tex at all must not be counted as read.
func TestAPaperWithNoSourceIsNotCountedAsRead(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "2003.00003"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "2004.00004.tex"), []byte(`\documentclass{a}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"-corpus", root, `\looseness`}, &out, &errb)
	// The only paper directory holds nothing, so nothing was read: refuse.
	if code != 2 {
		t.Errorf("exit %d, want 2; stdout %q stderr %q", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "population: 0 paper(s) of 1 named, 0 .tex file(s)") {
		t.Errorf("the population line should say it read nothing:\n%s", out.String())
	}
}

func TestCommentAt(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{`no comment here`, -1},
		{`a % comment`, 2},
		{`50\% is a percent sign`, -1},
		{`50\\% is a comment, the backslash is escaped`, 4},
		// \% \% are two escaped percents (indices 0-3), " then " runs 4-9, so
		// the real comment opens at 10. Counted by hand once as 11; the code
		// was right and the expectation was not.
		{`\%\% then % here`, 10},
		{`%at the start`, 0},
	} {
		if got := commentAt(tc.in); got != tc.want {
			t.Errorf("commentAt(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
