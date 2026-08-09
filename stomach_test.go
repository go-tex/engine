package engine

import "testing"

func TestBoxDimensions(t *testing.T) {
	cases := []struct{ src, want string }{
		// hbox natural width = sum of kerns
		{`\setbox0=\hbox{\kern3pt\kern5pt}\message{\the\wd0}`, "8.0pt"},
		{`\setbox0=\hbox{\kern3pt\kern5pt}\message{\the\ht0}`, "0.0pt"},
		// \hbox to <dimen> forces the width
		{`\setbox0=\hbox to 20pt{\kern3pt}\message{\the\wd0}`, "20.0pt"},
		// \hbox spread adds to natural
		{`\setbox0=\hbox spread 5pt{\kern3pt}\message{\the\wd0}`, "8.0pt"},
		// glue does not change a `to` box width
		{`\setbox0=\hbox to 10pt{\kern2pt\hfil}\message{\the\wd0}`, "10.0pt"},
		// \vrule with explicit dims sets all three box dimensions
		{`\setbox0=\hbox{\vrule width2pt height3pt depth1pt}\message{\the\wd0}`, "2.0pt"},
		{`\setbox0=\hbox{\vrule width2pt height3pt depth1pt}\message{\the\ht0}`, "3.0pt"},
		{`\setbox0=\hbox{\vrule width2pt height3pt depth1pt}\message{\the\dp0}`, "1.0pt"},
		// vbox: heights of stacked rules accumulate
		{`\setbox0=\vbox{\hrule height2pt \hrule height3pt}\message{\the\ht0}`, "5.0pt"},
		{`\setbox0=\vbox to 10pt{\hrule height2pt}\message{\the\ht0}`, "10.0pt"},
		// nested hbox contributes its packed width
		{`\setbox0=\hbox{\hbox{\kern4pt}\kern1pt}\message{\the\wd0}`, "5.0pt"},
		// \wd assignment overrides
		{`\setbox0=\hbox{\kern3pt}\wd0=100pt \message{\the\wd0}`, "100.0pt"},
		// \copy leaves the source, \box empties it
		{`\setbox0=\hbox{\kern7pt}\setbox1=\copy0 \message{\the\wd1}`, "7.0pt"},
		{`\setbox0=\hbox{\kern7pt}\setbox1=\box0 \message{\the\wd0}`, "0.0pt"},
		// register+dimen interplay: box width feeds a \dimen
		{`\setbox0=\hbox{\kern6pt\kern4pt}\dimen5=\wd0 \message{\the\dimen5}`, "10.0pt"},
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
