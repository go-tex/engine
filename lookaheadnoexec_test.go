package engine

import "testing"

// A glue assignment reads its dimension and then LOOKS AHEAD for "plus", so the
// tokens after the value are expanded before the assignment is complete. TeX's
// get_x_token expands macros there but never executes: it returns any command
// code <= max_command as it stands (tex.web §380), so scan_keyword meets the
// \begingroup inside LaTeX's \begin, backs it up, and the assignment finishes
// OUTSIDE the environment's group.
//
// Ours reaches the same work through \gotex@checkenv, an expandable primitive
// that opens \begin's group and sets \@currenvir. Executed during the lookahead,
// it opened the group mid-assignment: the value was saved into the environment
// and restored at \end, so \abovecaptionskip=40pt\begin{figure} left 10pt behind.
//
// The expected values below are a real TeX's (tectonic), not a guess.
func TestAssignmentBeforeBeginSurvivesTheEnvironment(t *testing.T) {
	cases := []struct{ src, want string }{
		// The defect itself: a skip, whose scan looks ahead for "plus".
		{`\newskip\s\s=40pt\begin{center}\end{center}\message{[\the\s]}`, "[40.0pt]"},
		// A dimen scan looks ahead for a unit's continuation the same way.
		{`\newdimen\d\d=40pt\begin{center}\end{center}\message{[\the\d]}`, "[40.0pt]"},
		// An integer scan looks ahead for another digit.
		{`\newcount\n\n=40\begin{center}\end{center}\message{[\the\n]}`, "[40]"},
		// \relax stops the lookahead, so this case was already right — it is the
		// control that told the two apart.
		{`\newskip\s\s=40pt\relax\begin{center}\end{center}\message{[\the\s]}`, "[40.0pt]"},
		// Inside the environment the value is the assigned one too.
		{`\newskip\s\s=40pt\begin{center}\message{[\the\s]}\end{center}`, "[40.0pt]"},
		// A group the document opens itself still scopes the assignment.
		{`\newskip\s\s=10pt{\s=40pt}\message{[\the\s]}`, "[10.0pt]"},
	}
	for _, c := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Run(c.src); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if got := trimNL(e.out.String()); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}
