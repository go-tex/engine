// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// captionHead renders a document and returns the characters of its caption heads.
func captionHead(t *testing.T, src string) string {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	return b.String()
}

// \captionsetup[<type>]{name=<text>} renames the label a caption is headed with.
// acmart does exactly this at acmart.cls:934 for its journal formats, which is
// why an ACM journal paper's figures read "Fig. 1". Checked against tectonic:
//
//	\captionsetup[figure]{name={Fig.}}   ->  Fig. 1: une figure
//	\captionsetup[table]{name={Tbl.}}    ->  Tbl. 1: un tableau
//	\captionsetup[figure]{name=Fig.}     ->  Fig. 1   (braces are optional)
func TestCaptionsetupName(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{
			"braced",
			`\hsize=300pt\captionsetup[figure]{name={Fig.}}` +
				`\begin{figure}\caption{x}\end{figure}`,
			"Fig.1:x",
		},
		{
			"unbraced",
			`\hsize=300pt\captionsetup[figure]{name=Fig.}` +
				`\begin{figure}\caption{x}\end{figure}`,
			"Fig.1:x",
		},
		{
			"table",
			`\hsize=300pt\captionsetup[table]{name={Tbl.}}` +
				`\begin{table}\caption{y}\end{table}`,
			"Tbl.1:y",
		},
		{
			// A type it was not given keeps its own name.
			"other type untouched",
			`\hsize=300pt\captionsetup[figure]{name={Fig.}}` +
				`\begin{table}\caption{y}\end{table}`,
			"Table1:y",
		},
		{
			// Styling options are still gobbled, and a name alongside them is
			// still read: acmart writes several keys in one call.
			"name among other keys",
			`\hsize=300pt\captionsetup[figure]{labelfont={sf, small},name={Fig.},margin=0pt}` +
				`\begin{figure}\caption{x}\end{figure}`,
			"Fig.1:x",
		},
		{
			// No name key: nothing changes, and nothing leaks onto the page.
			"no name key",
			`\hsize=300pt\captionsetup[figure]{position=top,labelsep=colon}` +
				`\begin{figure}\caption{x}\end{figure}`,
			"Figure1:x",
		},
		{
			// With no [type] the caption package uses \@captype, which is
			// undefined in a preamble — real caption raises "Undefined control
			// sequence" there (checked against tectonic). Here it is ignored.
			"no type outside a float",
			`\hsize=300pt\captionsetup{name={Thing}}` +
				`\begin{figure}\caption{x}\end{figure}`,
			"Figure1:x",
		},
	} {
		if got := captionHead(t, c.src); got != c.want {
			t.Errorf("%s: caption head %q, want %q", c.name, got, c.want)
		}
	}
}

// stripOuterBraceToks removes one outer pair and leaves an inner group alone, so
// name={\textbf{Fig.}} keeps the braces its own macro needs.
func TestStripOuterBraceToks(t *testing.T) {
	brace := func(s string) []tok {
		var out []tok
		for _, r := range s {
			switch r {
			case '{':
				out = append(out, chTok('{', catBegin))
			case '}':
				out = append(out, chTok('}', catEnd))
			default:
				out = append(out, chTok(r, catLetter))
			}
		}
		return out
	}
	for _, c := range []struct{ in, want string }{
		{"{Fig}", "Fig"},
		{"Fig", "Fig"},
		{"{a}{b}", "{a}{b}"}, // the first brace closes early: not one outer group
		{"{{a}}", "{a}"},
		{"{}", ""},
	} {
		e := New()
		if got := e.toksToString(stripOuterBraceToks(brace(c.in))); got != c.want {
			t.Errorf("stripOuterBraceToks(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// An ACM JOURNAL format heads its figures "Fig." even when acmart.cls is not
// bundled and the class is emulated; the sigconf family keeps "Figure".
//
// ⛔ manuscript is excluded although acmart counts it as a journal and makes it
// the default: two corpus papers write \documentclass[STYLE]{acmart}, an
// unsubstituted placeholder, and BOTH references print "Figure". So the name
// changes only when the document names a journal format.
func TestAcmartJournalFigureName(t *testing.T) {
	for _, c := range []struct {
		opts []string
		want string
	}{
		{[]string{"acmsmall"}, "Fig."},
		{[]string{"acmlarge", "review"}, "Fig."},
		{[]string{"format=acmtog"}, "Fig."},
		{[]string{"acmcp"}, "Fig."},
		{[]string{"sigconf"}, "Figure"},
		{[]string{"sigplan", "screen"}, "Figure"},
		{[]string{"manuscript"}, "Figure"},
		{[]string{"STYLE"}, "Figure"},
		{nil, "Figure"},
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		e.applyAcmartFigureName(c.opts)
		if got := e.toksToString(e.expandList([]tok{csTok("figurename")})); got != c.want {
			t.Errorf("acmart%v: figurename = %q, want %q", c.opts, got, c.want)
		}
	}
}
