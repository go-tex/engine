// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Command gotex-abdiff compares TWO REVISIONS OF THIS ENGINE over a fixed corpus
// of real papers, and says what a change did to pagination.
//
// It is the complement of gotex-refdiff, which compares the engine against a real
// LaTeX engine (how faithful are we?). This asks the other question: did this
// change move anything, and toward the reference or away from it? That is the
// measurement every corpus-affecting commit needs, and doing it by hand produced
// the same three wrong answers often enough to be worth a tool:
//
//   - a BASE THAT WAS NOT THE BASE. Building it by switching the working tree
//     carries uncommitted changes across, so both arms hold the treatment and the
//     table comes back clean. Here each side is built in a worktree of its own
//     and the report prints the commit it resolved (build.go).
//   - a PAGE COUNT THAT WAS A GREP. Counting "/Type /Page" in the raw bytes reads
//     zero for a PDF with object streams and zero for a file that does not exist,
//     so "334 pages", "no output" and "I cannot read this" all arrived as the same
//     number. Here the PDF is parsed, and anything unreadable is an error the
//     report counts and refuses to score (pages.go).
//   - A REFERENCE READ BY KEY. The reference page counts are paired with the
//     corpus list BY POSITION; read as a map they match nothing, and the result
//     "0 exact, Σ 0" looks like a perfect score (corpus.go).
//
// Example:
//
//	GOWORK=off go run ./cmd/gotex-abdiff \
//	  -list /path/clean-list.txt -refs /path/clean-refs.txt \
//	  -base origin/main -head HEAD -texmf /path/texmf
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

type result struct {
	paper      Paper
	base, head int
	baseErr    error
	headErr    error
}

func main() {
	var (
		list    = flag.String("list", "", "corpus list: one paper path per line")
		refs    = flag.String("refs", "", "reference page counts, one per line, PAIRED BY POSITION with -list")
		repo    = flag.String("repo", ".", "the engine repository to build from")
		base    = flag.String("base", "origin/main", "revision to compare against; empty ranks -head alone, worst-first")
		head    = flag.String("head", "HEAD", "revision under test")
		texmf   = flag.String("texmf", "", "value for GOTEX_TEXMF when compiling papers")
		timeout = flag.Duration("timeout", 300*time.Second, "per-paper compile timeout")
		top     = flag.Int("top", 12, "how many movers to list")
		keep    = flag.Bool("keep-going", false, "report even when a paper failed to compile on one side")
	)
	flag.Parse()
	if *list == "" || *refs == "" {
		fmt.Fprintln(os.Stderr, "gotex-abdiff: -list and -refs are required")
		os.Exit(2)
	}
	if *base == "" {
		if err := rank(*list, *refs, *repo, *head, *texmf, *timeout, *top); err != nil {
			fmt.Fprintln(os.Stderr, "gotex-abdiff:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*list, *refs, *repo, *base, *head, *texmf, *timeout, *top, *keep); err != nil {
		fmt.Fprintln(os.Stderr, "gotex-abdiff:", err)
		os.Exit(1)
	}
}

func run(list, refs, repo, base, head, texmf string, timeout time.Duration, top int, keep bool) error {
	papers, err := loadCorpus(list, refs)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "abdiff-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	baseBin := filepath.Join(work, "gotex-base")
	headBin := filepath.Join(work, "gotex-head")
	baseCommit, err := buildSide(repo, base, baseBin)
	if err != nil {
		return err
	}
	headCommit, err := buildSide(repo, head, headBin)
	if err != nil {
		return err
	}
	fmt.Printf("base %s  %s\nhead %s  %s\n%d papers%s\n\n", base, shortCommit(baseCommit), head, shortCommit(headCommit), len(papers), renderNote())
	if baseCommit == headCommit {
		fmt.Println("⚠ both sides resolve to the same commit: nothing can move.")
	}
	// ⛔ HEAD MUST CONTAIN BASE, or the comparison also carries whatever landed on
	// base meanwhile — with the sign reversed, which reads as a regression in the
	// change under test. Measured the hard way: a branch cut before a 44-page
	// improvement merged reported "Σ 293 -> 336 (+43)" for a change that touches
	// nothing those papers use. The commits printed above are what made it
	// visible; this says it outright.
	if !contains(repo, baseCommit, headCommit) {
		fmt.Printf("⛔ %s is NOT an ancestor of %s: this comparison includes everything\n"+
			"   that landed on the base since the head was cut, with its sign reversed.\n"+
			"   Rebase the head onto the base and run again.\n\n", base, head)
	}

	results := make([]result, 0, len(papers))
	for i, p := range papers {
		r := result{paper: p}
		r.base, r.baseErr = compileAndCount(baseBin, p.Dir, texmf, filepath.Join(work, "b.pdf"), timeout)
		r.head, r.headErr = compileAndCount(headBin, p.Dir, texmf, filepath.Join(work, "h.pdf"), timeout)
		results = append(results, r)
		progress(i+1, len(papers))
	}
	progressDone()
	return report(results, top, keep)
}

// rank measures ONE revision and lists the papers worst-first by how far their
// page count is from the reference's.
//
// It is the other half of the loop this tool serves: a comparison says whether a
// change helped, and a ranking says what to look at next. Doing it with the A/B
// path and a throwaway second revision would work and would compile every paper
// twice for an answer that needs one side.
//
// ⛔ A paper that produces no readable PDF is listed FIRST, with its error, and
// is not scored. It is worse than any page deviation and it must not vanish into
// a total.
func rank(list, refs, repo, head, texmf string, timeout time.Duration, top int) error {
	papers, err := loadCorpus(list, refs)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "abdiff-rank-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	bin := filepath.Join(work, "gotex")
	commit, err := buildSide(repo, head, bin)
	if err != nil {
		return err
	}
	fmt.Printf("%s  %s\n%d papers%s\n\n", head, shortCommit(commit), len(papers), renderNote())

	type row struct {
		id       string
		got, ref int
		err      error
	}
	rows := make([]row, 0, len(papers))
	for i, p := range papers {
		n, err := compileAndCount(bin, p.Dir, texmf, filepath.Join(work, "r.pdf"), timeout)
		rows = append(rows, row{p.ID, n, p.RefPage, err})
		progress(i+1, len(papers))
	}
	progressDone()

	var broken int
	for _, r := range rows {
		if r.err != nil {
			broken++
			fmt.Printf("  NO PDF  %-14s %v\n", r.id, r.err)
		}
	}
	var sum, exact int
	scored := make([]row, 0, len(rows))
	for _, r := range rows {
		if r.err != nil {
			continue
		}
		scored = append(scored, r)
		sum += abs(r.got - r.ref)
		if r.got == r.ref {
			exact++
		}
	}
	fmt.Printf("\nscored            %d paper(s)%s\n", len(scored), plural(broken))
	fmt.Printf("Σ|page deviation| %d\n", sum)
	fmt.Printf("exact pagination  %d\n\n", exact)
	sort.Slice(scored, func(i, j int) bool {
		return abs(scored[i].got-scored[i].ref) > abs(scored[j].got-scored[j].ref)
	})
	for i, r := range scored {
		if i >= top {
			break
		}
		fmt.Printf("  %-14s %3d  (reference %3d)  %+d\n", r.id, r.got, r.ref, r.got-r.ref)
	}
	return nil
}

func plural(broken int) string {
	if broken == 0 {
		return ""
	}
	return fmt.Sprintf(", %d produced no readable PDF and are NOT scored", broken)
}

// renderNote puts the measurement's CONFIGURATION in its own report.
//
// ⛔ PDF figures are opt-in (GOTEX_PDFRENDER=1), and 83 of the 154 corpus papers
// have one — more than half the population is set with an empty placeholder box
// unless it is on. 2312.05895 is the clearest case: it ends with twelve
// \includepdf calls for a 12-page supplement and measures 15 pages against the
// reference's 27 with the renderer off, and 27 against 27 with it on. A number
// produced in one configuration and read in the other is not a small error; it
// sent me chasing a defect that was a flag.
func renderNote() string {
	if os.Getenv("GOTEX_PDFRENDER") == "1" {
		return "  (GOTEX_PDFRENDER=1: PDF figures rasterised)"
	}
	return "  (GOTEX_PDFRENDER unset: PDF figures are empty boxes — 83 of 154 corpus papers have one)"
}

// contains reports whether commit anc is an ancestor of (or equal to) desc.
func contains(repo, anc, desc string) bool {
	if anc == desc {
		return true
	}
	cmd := exec.Command("git", "merge-base", "--is-ancestor", anc, desc)
	cmd.Dir = repo
	return cmd.Run() == nil
}

// compileAndCount runs one side on one paper and parses the PDF it wrote.
func compileAndCount(bin, dir, texmf, out string, timeout time.Duration) (int, error) {
	_ = os.Remove(out) // never read the previous paper's PDF
	abs, err := filepath.Abs(out)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command(bin, "-lenient", "-o", abs, "main.tex")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	if texmf != "" {
		cmd.Env = append(cmd.Env, "GOTEX_TEXMF="+texmf)
	}
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return 0, fmt.Errorf("timed out after %s", timeout)
	}
	return pageCount(abs)
}

// report prints the comparison, and REFUSES to score it when a paper failed on
// one side: a population that shrank silently is how a regression hides.
func report(rs []result, top int, keep bool) error {
	var errs int
	for _, r := range rs {
		if r.baseErr != nil || r.headErr != nil {
			errs++
			which, e := "base", r.baseErr
			if e == nil {
				which, e = "head", r.headErr
			}
			fmt.Printf("  ERROR %-14s %s: %v\n", r.paper.ID, which, e)
		}
	}
	if errs > 0 {
		fmt.Printf("\n%d paper(s) produced no readable PDF on one side.\n", errs)
		if !keep {
			return fmt.Errorf("refusing to score a population that lost %d paper(s); pass -keep-going to score the rest anyway", errs)
		}
	}

	var sumBase, sumHead, exactBase, exactHead, toward, away int
	type mover struct {
		id        string
		b, h, ref int
	}
	var movers []mover
	scored := 0
	for _, r := range rs {
		if r.baseErr != nil || r.headErr != nil {
			continue
		}
		scored++
		db, dh := abs(r.base-r.paper.RefPage), abs(r.head-r.paper.RefPage)
		sumBase += db
		sumHead += dh
		if db == 0 {
			exactBase++
		}
		if dh == 0 {
			exactHead++
		}
		if r.base != r.head {
			movers = append(movers, mover{r.paper.ID, r.base, r.head, r.paper.RefPage})
			switch {
			case dh < db:
				toward++
			case dh > db:
				away++
			}
		}
	}
	fmt.Printf("\nscored            %d paper(s)\n", scored)
	fmt.Printf("Σ|page deviation| %d -> %d   (%+d)\n", sumBase, sumHead, sumHead-sumBase)
	fmt.Printf("exact pagination  %d -> %d   (%+d)\n", exactBase, exactHead, exactHead-exactBase)
	fmt.Printf("papers moved      %d   (%d toward the reference, %d away, %d sideways)\n",
		len(movers), toward, away, len(movers)-toward-away)
	sort.Slice(movers, func(i, j int) bool {
		return abs(movers[i].h-movers[i].b) > abs(movers[j].h-movers[j].b)
	})
	for i, m := range movers {
		if i >= top {
			fmt.Printf("  … and %d more\n", len(movers)-top)
			break
		}
		fmt.Printf("  %-14s %3d -> %3d   (reference %3d)\n", m.id, m.b, m.h, m.ref)
	}
	return nil
}

// progress writes a counter only when stderr is a TERMINAL. Redirected to a file
// — which is how a long run is kept — a carriage-return counter writes 154
// fragments into the record and buries the report under them.
func progress(i, n int) {
	if !stderrIsTerminal() {
		return
	}
	fmt.Fprintf(os.Stderr, "\r%d/%d", i, n)
}

func progressDone() {
	if stderrIsTerminal() {
		fmt.Fprintln(os.Stderr)
	}
}

func stderrIsTerminal() bool {
	st, err := os.Stderr.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
