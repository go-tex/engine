package engine

import "testing"

func TestNewenvironment(t *testing.T) {
	cases := []struct{ src, want string }{
		// no-arg environment
		{`\newenvironment{note}{[B]}{[E]}\message{\begin{note}mid\end{note}}`, "[B]mid[E]"},
		// one-arg environment (begin uses #1)
		{`\newenvironment{tag}[1]{<#1>}{</>}\message{\begin{tag}{x}body\end{tag}}`, "<x>body</>"},
		// nested environments
		{`\newenvironment{a}{(}{)}\newenvironment{b}{[}{]}\message{\begin{a}\begin{b}z\end{b}\end{a}}`, "([z])"},
	}
	for _, c := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		got, err := e.Run(c.src)
		if err != nil {
			t.Errorf("%q: err %v", c.src, err)
			continue
		}
		if trimNL(got) != c.want {
			t.Errorf("%q: got %q want %q", c.src, trimNL(got), c.want)
		}
	}
}
