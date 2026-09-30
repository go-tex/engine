package engine

import "testing"

// A body scan that leaves the input level it began in, without having found its
// \end, must stop there — not read on into whatever was underneath.
//
// This is a white-box test because the situation is one only a class can create:
// the environment is opened by code whose closer arrives another way, so the \end
// is in NO input at all. iucr.cls does exactly that — \renewenvironment{figure}
// opens \begin{minipage}{\textwidth} inside the true branch of \ifsingl@col and
// leaves the \end{minipage} to the environment's end code — and when the figure
// is read from an \input file the scan took the \else branch, the rest of the
// file, and every page after it. arXiv 2304.12934 lost its entire reference list
// that way: 445 words, the worst text deficit in the corpus, 77.1% of its
// reference's words before and 95.3% after.
func TestCollectEnvBodyStopsWhenItLeavesItsInputLevel(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.lenient = true

	// What is underneath: the document the scan must NOT eat.
	e.pushInputLevel("UNDERNEATH")
	under := len(e.levels)
	// The file the environment is read from, holding no \end{minipage}.
	e.pushInputLevel("BODYONLY")

	body := e.collectEnvBody("minipage")
	if body != nil {
		t.Errorf("the scan returned %d token(s); it must stop at the file boundary, not capture past it", len(body))
	}
	if len(e.levels) < under {
		t.Errorf("levels = %d, want at least %d: the scan popped past the level it must stop in", len(e.levels), under)
	}
	// And what it read is back in the input, so it flows as ordinary text.
	var got []rune
	for {
		tk, ok := e.getNext()
		if !ok {
			break
		}
		if !tk.cs_ {
			got = append(got, tk.ch)
		}
	}
	if s := string(got); len(s) == 0 {
		t.Error("the scan kept what it read; it must be pushed back")
	}
}
