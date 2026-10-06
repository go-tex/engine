// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Paper is one corpus entry: where it lives, and how many pages the reference
// engine sets it in.
type Paper struct {
	ID      string // the directory's base name, which is how a report names it
	Dir     string // the directory to compile in
	RefPage int    // the reference's page count
}

// loadCorpus reads the two files the corpus is kept in and pairs them.
//
// ⛔ THE PAIRING IS POSITIONAL, and that is the whole reason this function
// exists. The reference file holds page counts, one per line, with NO identifier
// — line n belongs to line n of the list. Read as a map keyed by paper id it
// yields nothing at all, and a measurement then reports "0 exact, Σ 0", which
// reads exactly like a perfect score. That happened; the control below is why it
// cannot happen again:
//
//   - the two files must have the SAME number of entries, and the error names
//     both counts rather than silently truncating to the shorter;
//   - every path must resolve to a directory that exists, so a stale list cannot
//     quietly shrink the population.
func loadCorpus(listPath, refsPath string) ([]Paper, error) {
	list, err := nonEmptyLines(listPath)
	if err != nil {
		return nil, err
	}
	refs, err := nonEmptyLines(refsPath)
	if err != nil {
		return nil, err
	}
	if len(list) != len(refs) {
		return nil, fmt.Errorf("corpus list has %d entries and the reference file %d: they are paired BY POSITION, so they must match", len(list), len(refs))
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("corpus list %s is empty", listPath)
	}
	out := make([]Paper, 0, len(list))
	for i, l := range list {
		dir := strings.TrimSuffix(strings.TrimSuffix(l, "main.tex"), "/")
		for strings.HasSuffix(dir, "/") {
			dir = strings.TrimSuffix(dir, "/")
		}
		n, err := strconv.Atoi(refs[i])
		if err != nil {
			return nil, fmt.Errorf("%s line %d: reference page count %q is not a number", refsPath, i+1, refs[i])
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("%s line %d: %s is not a directory", listPath, i+1, dir)
		}
		out = append(out, Paper{ID: filepath.Base(dir), Dir: dir, RefPage: n})
	}
	return out, nil
}

func nonEmptyLines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}
