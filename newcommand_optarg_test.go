package engine

import "testing"

// runTypeset runs source with a mock font and returns the typeset characters
// (inter-word spaces are glue, not chars, so they do not appear here).
func runTypeset(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return mvlText(e.mvl)
}

// TestNewcommandOptArg exercises LaTeX macros whose first argument is optional:
// \newcommand{\cmd}[nargs][default]{body}. When the call omits the [..] bracket,
// #1 takes the default; when present, #1 takes the bracket content.
func TestNewcommandOptArg(t *testing.T) {
	cases := []struct{ src, want string }{
		// default used when no bracket is supplied
		{`\newcommand{\greet}[2][Bonjour]{#1, #2!}\greet{Alice}`, "Bonjour,Alice!"},
		// bracket overrides the default
		{`\newcommand{\greet}[2][Bonjour]{#1, #2!}\greet[Salut]{Bob}`, "Salut,Bob!"},
		// single optional arg, default path
		{`\newcommand{\opt}[1][d]{[#1]}\opt`, "[d]"},
		// single optional arg, bracket path
		{`\newcommand{\opt}[1][d]{[#1]}\opt[z]`, "[z]"},
		// plain mandatory-only macro is unaffected (no optArg)
		{`\newcommand{\add}[2]{#1#2}\add{x}{y}`, "xy"},
	}
	for _, c := range cases {
		if got := runTypeset(t, c.src); got != c.want {
			t.Errorf("%q: got %q want %q", c.src, got, c.want)
		}
	}
}

// TestNewenvironmentOptArg exercises \newenvironment whose first argument is
// optional: \newenvironment{name}[nargs][default]{begin}{end}.
func TestNewenvironmentOptArg(t *testing.T) {
	cases := []struct{ src, want string }{
		// default used when \begin{note} has no bracket
		{`\newenvironment{note}[1][DEF]{<#1>}{</>}\begin{note}mid\end{note}`, "<DEF>mid</>"},
		// bracket overrides the default
		{`\newenvironment{note}[1][DEF]{<#1>}{</>}\begin{note}[X]mid\end{note}`, "<X>mid</>"},
	}
	for _, c := range cases {
		if got := runTypeset(t, c.src); got != c.want {
			t.Errorf("%q: got %q want %q", c.src, got, c.want)
		}
	}
}
