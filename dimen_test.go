package engine

import "testing"

func TestDimenScanAndThe(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\dimen0=3pt \message{\the\dimen0}`, "3.0pt"},
		{`\dimen0=1in \message{\the\dimen0}`, "72.26999pt"},
		{`\dimen0=2.5pt \message{\the\dimen0}`, "2.5pt"},
		{`\newdimen\w \w=10pt \advance\w by 5pt \message{\the\w}`, "15.0pt"},
		{`\newdimen\w \w=3pt \multiply\w by 4 \message{\the\w}`, "12.0pt"},
		{`\dimendef\x=7 \x=1pc \message{\the\x}`, "12.0pt"},
		{`\newdimen\a\newdimen\b \a=5pt \b=\a \advance\b by \a \message{\the\b}`, "10.0pt"},
	}
	for _, c := range cases {
		e := New()
		got, err := e.Run(c.src)
		if err != nil {
			t.Fatalf("%q: err %v", c.src, err)
		}
		got = trimNL(got)
		if got != c.want {
			t.Errorf("%q: got %q want %q", c.src, got, c.want)
		}
	}
}

func trimNL(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}
