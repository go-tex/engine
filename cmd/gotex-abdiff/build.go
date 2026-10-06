// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// buildSide compiles the gotex CLI from one git revision and returns the binary's
// path and the commit it was built from.
//
// ⛔ IT BUILDS IN A WORKTREE OF ITS OWN, and that is not tidiness. Building a
// "base" by switching the working tree to another revision CARRIES UNCOMMITTED
// CHANGES ACROSS: `git switch` keeps modified files, so a base built that way is
// the other revision PLUS whatever was being tested, both arms hold the
// treatment, and the comparison comes back clean. That is how a real defect was
// nearly dismissed as absent — the table said "0 -> 0" and it was true of two
// identical binaries.
//
// `git worktree add --detach` cannot pick up the working tree, so the only thing
// in the build is the revision named. The resolved commit goes in the report, so
// a reader can see what was actually compared instead of trusting the labels.
func buildSide(repo, rev, outBin string) (commit string, err error) {
	commit, err = gitOut(repo, "rev-parse", rev+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("revision %q: %w", rev, err)
	}
	wt, err := os.MkdirTemp("", "abdiff-"+shortCommit(commit)+"-")
	if err != nil {
		return "", err
	}
	defer func() {
		_, _ = gitOut(repo, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
	}()
	if _, err := gitOut(repo, "worktree", "add", "--quiet", "--detach", wt, commit); err != nil {
		return "", fmt.Errorf("worktree for %s: %w", rev, err)
	}
	abs, err := filepath.Abs(outBin)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("go", "build", "-o", abs, "./cmd/gotex")
	cmd.Dir = wt
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build %s: %w\n%s", rev, err, out)
	}
	return commit, nil
}

func gitOut(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func shortCommit(c string) string {
	if len(c) > 8 {
		return c[:8]
	}
	return c
}
