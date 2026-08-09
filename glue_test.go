package engine

import "testing"

func TestGlueScanAndThe(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\skip0=3pt \message{\the\skip0}`, "3.0pt"},
		{`\skip0=3pt plus 1pt minus 2pt \message{\the\skip0}`, "3.0pt plus 1.0pt minus 2.0pt"},
		{`\skip0=0pt plus 1fil \message{\the\skip0}`, "0.0pt plus 1.0fil"},
		{`\skip0=0pt plus 2fill \message{\the\skip0}`, "0.0pt plus 2.0fill"},
		{`\newskip\s \s=6pt plus 3fil \message{\the\s}`, "6.0pt plus 3.0fil"},
		{`\skipdef\q=5 \q=1pt plus 1pt \skip5=\q \message{\the\skip5}`, "1.0pt plus 1.0pt"},
		{`\newskip\s \s=3pt plus 2pt \advance\s by 4pt plus 1pt \message{\the\s}`, "7.0pt plus 3.0pt"},
		{`\newskip\s \s=2pt plus 1fil \multiply\s by 3 \message{\the\s}`, "6.0pt plus 3.0fil"},
		{`\newskip\s \s=1pt plus 1pt \advance\s by 0pt plus 1fil \message{\the\s}`, "1.0pt plus 1.0fil"},
	}
	for _, c := range cases {
		e := New()
		got, err := e.Run(c.src)
		if err != nil {
			t.Fatalf("%q: err %v", c.src, err)
		}
		if g := trimNL(got); g != c.want {
			t.Errorf("%q: got %q want %q", c.src, g, c.want)
		}
	}
}
