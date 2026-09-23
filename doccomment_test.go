package engine

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A doc comment must open with the name of the function it precedes (the Go
// convention, and what godoc renders). When it opens with the name of a DIFFERENT
// function, the usual cause is that a function was inserted between a comment and
// its own body: the comment is then read as documenting code it does not describe.
//
// Sixteen of these had accumulated. The sixteenth was added on 2026-09-23 by the
// commit that introduced Engine.lastSkip, which was inserted directly above
// Engine.place and took place's doc comment with it — which is why this test
// exists rather than a one-time cleanup.
func TestDocCommentNamesItsOwnFunction(t *testing.T) {
	fn := regexp.MustCompile(`^func (?:\([^)]*\) )?([A-Za-z_][A-Za-z0-9_]*)`)
	// The first word of a doc comment, when the comment opens "name ...".
	opens := regexp.MustCompile(`^// ([a-z][A-Za-z0-9_]*) `)

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(src), "\n")
		for i := 0; i < len(lines); i++ {
			if !strings.HasPrefix(lines[i], "//") {
				continue
			}
			j := i
			for j < len(lines) && strings.HasPrefix(lines[j], "//") {
				j++
			}
			if j < len(lines) {
				if m := fn.FindStringSubmatch(lines[j]); m != nil {
					if o := opens.FindStringSubmatch(lines[i]); o != nil && o[1] != m[1] {
						t.Errorf("%s:%d: the doc comment opens with %q but documents %q — "+
							"a function was probably inserted between %q's comment and its body",
							f, i+1, o[1], m[1], o[1])
					}
				}
			}
			i = j
		}
	}
}
