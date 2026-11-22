// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/go-tex/engine/cmd/internal/refaudit"
)

// auditConfig says where the reference PDFs are and whether an unusable one is
// still scored.
type auditConfig struct {
	Dir     string // directory holding <paper id>.pdf; empty means the audit is NOT run
	Include bool   // score the unusable references anyway
}

// unusableRef is a reference PDF that cannot stand as a reference.
type unusableRef struct {
	paper  Paper
	reason string
}

// auditRefs probes the reference PDF of every paper and splits the corpus.
//
// ⛔ A reference that cannot be READ stops the run: an unread reference is not a
// clean one, and scoring against it would be scoring against nothing. An empty
// Dir runs no audit and the caller says so; it never reads as "all clean".
func auditRefs(papers []Paper, cfg auditConfig) (usable []Paper, bad []unusableRef, err error) {
	if cfg.Dir == "" {
		return papers, nil, nil
	}
	for _, p := range papers {
		v, err := refaudit.Probe(filepath.Join(cfg.Dir, p.ID+".pdf"))
		if err != nil {
			return nil, nil, fmt.Errorf("reference of %s could not be audited: %w", p.ID, err)
		}
		if why := v.Unusable(); why != "" {
			bad = append(bad, unusableRef{p, why})
			if cfg.Include {
				usable = append(usable, p)
			}
			continue
		}
		usable = append(usable, p)
	}
	if len(usable) == 0 {
		return nil, nil, fmt.Errorf("every reference in %s is unusable or none was found: nothing to score", cfg.Dir)
	}
	return usable, bad, nil
}

// loadAudited is what both modes start from: the corpus, audited, with the
// "reference unusable" block already written to w.
func loadAudited(w io.Writer, list, refs string, cfg auditConfig) ([]Paper, error) {
	papers, err := loadCorpus(list, refs)
	if err != nil {
		return nil, err
	}
	papers, bad, err := auditRefs(papers, cfg)
	if err != nil {
		return nil, err
	}
	reportUnusable(w, bad, cfg)
	return papers, nil
}

// reportUnusable prints the "reference unusable" block. It always says what was
// done with them, because a total that silently lost papers reads as a better
// corpus.
func reportUnusable(w io.Writer, bad []unusableRef, cfg auditConfig) {
	if cfg.Dir == "" {
		fmt.Fprintln(w, "⚠ reference audit NOT RUN (-refpdf not given): corrupt references, if any, are scored")
		return
	}
	if len(bad) == 0 {
		fmt.Fprintln(w, "reference audit: no unusable reference")
		return
	}
	fmt.Fprintf(w, "reference unusable: %d paper(s)\n", len(bad))
	for _, b := range bad {
		fmt.Fprintf(w, "  %-14s %s\n", b.paper.ID, b.reason)
	}
	if cfg.Include {
		fmt.Fprintln(w, "  ⚠ -include-unusable: these ARE scored below, and the totals carry their defect")
	} else {
		fmt.Fprintln(w, "  left out of Σ|page deviation| and exact pagination (-include-unusable scores them)")
	}
	fmt.Fprintln(w)
}
