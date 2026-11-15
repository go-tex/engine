package engine

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-tex/texmf"
)

// ⛔⛔ TEN TESTS IN THIS PACKAGE HAD NEVER RUN, ANYWHERE. Eight TikZ assertions
// and two beamer ones skipped on `GOTEX_TEXMF` being unset, and nothing in this
// repository's workflows sets it or installs a texmf tree — so every runner
// skipped them, every package still printed `ok`, and no count could see it
// (#783; the font-face instance of the same shape was #590, fixed in #782).
//
// The tree they want is what `go-tex/texmf` exists to produce, and this engine
// already depends on it: `cmd/gotex` fetches bundles by digest and hands them to
// the engine. These tests now ask the same module for the same bundles.
//
// ⛔ FROM THE CACHE ONLY — `Offline: true`. A unit test must not reach the
// network: it would be slow, it would fail where there is none, and a fetch
// inside an assertion is a fetch nobody can see fail. CI warms the cache in its
// own named step, so a download that breaks turns THAT step red instead of
// quietly turning ten assertions back into skips.
//
// ⚠ AND IT IS STILL A SKIP WHEN THE CACHE IS EMPTY, which is the honest limit:
// the test cannot run without the sources. What changes is that it runs wherever
// the cache exists — which is every machine that has compiled one real document
// — instead of only where somebody exported a variable by hand. The remaining
// skip names the bundle and how to get it, so it is actionable rather than
// mysterious, and `testwhy --skipped` makes it a finding rather than silence.
func texmfPath(t *testing.T, b texmf.Bundle) string {
	t.Helper()
	// A host that points the engine at its own tree keeps doing so, unchanged:
	// a real TeX installation, a vendored copy, an air-gapped machine.
	if d := os.Getenv("GOTEX_TEXMF"); d != "" {
		return d
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// Dependencies first and the bundle last, which is the order a TDS search
	// path would have: beamer cannot load without translator, and a package
	// sometimes ships a copy of a file its dependency also ships.
	var dirs []string
	for _, d := range texmf.WithDependencies(b) {
		tree, err := texmf.Open(ctx, d, texmf.Options{Offline: true})
		if err != nil {
			// ⛔⛔ SKIP LOCALLY, FAIL ON CI. A skip is a convenience for a developer
			// who has not compiled a real document yet. On CI it is the defect this
			// change exists to remove, so there it must be RED.
			//
			// And it has to be an assertion rather than a hope, because the warming
			// step cannot be trusted to fail on its own: `gotex` reports a bundle it
			// could not fetch and carries on with its own emulation, exiting 0 —
			// correct for a user rendering a talk, useless as a gate. So if the fetch
			// failed silently, THIS is what says so.
			//
			// ⛔⛔ AND IT KEYS ON THE WARMING STEP, NOT ON `CI`. My first version read
			// CI, which GitHub sets in EVERY job — so it demanded the cache in the
			// qemu architecture lanes and the wasm build, which do not and should not
			// warm it, and thirty tests failed in lanes that were never meant to have
			// a tree. An assertion may only hold where its precondition was
			// established, and the step that establishes it is the one that says so:
			// the warming step exports GOTEX_TEXMF_WARMED into $GITHUB_ENV.
			//
			// That is not the "opt-in variable nobody sets" this lesson warns about:
			// it is set three lines from where it is read, in the same job, by the
			// step whose success it reports.
			if os.Getenv("GOTEX_TEXMF_WARMED") != "" {
				t.Fatalf("%s@%s is not in the texmf cache (%v) although the warming step "+
					"ran in this job, so this is a FAILURE and not a skip: `gotex` exits 0 "+
					"when a fetch fails, and ten assertions in this package depend on the "+
					"tree (#783)", d.Name, d.Version, err)
			}
			t.Skipf("%s@%s is not in the texmf cache (%v) — compile one document that "+
				"needs it with `gotex`, or point GOTEX_TEXMF at a tree holding its files",
				d.Name, d.Version, err)
		}
		dirs = append(dirs, tree.Dir())
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

// withTeXMF puts the bundle's tree on the engine's search path for one test.
func withTeXMF(t *testing.T, b texmf.Bundle) {
	t.Helper()
	t.Setenv("GOTEX_TEXMF", texmfPath(t, b))
}
