// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// refDir lays the three fixtures of cmd/internal/refaudit out as a reference
// directory: one corrupt by a leak, one by a junk first page, one clean.
func refDir(t *testing.T) (string, []Paper) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("..", "internal", "refaudit", "testdata")
	names := map[string]string{"leaky": "leaky.pdf", "junk": "junkfirst.pdf", "good": "clean.pdf"}
	var papers []Paper
	for _, id := range []string{"leaky", "junk", "good"} {
		b, err := os.ReadFile(filepath.Join(src, names[id]))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".pdf"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		papers = append(papers, Paper{ID: id, RefPage: 3})
	}
	return dir, papers
}

func TestAuditLeavesCorruptReferencesOut(t *testing.T) {
	dir, papers := refDir(t)
	usable, bad, err := auditRefs(papers, auditConfig{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(usable) != 1 || usable[0].ID != "good" {
		t.Errorf("usable = %v, want only the clean reference", usable)
	}
	if len(bad) != 2 || bad[0].paper.ID != "leaky" || bad[1].paper.ID != "junk" {
		t.Fatalf("unusable = %v, want leaky then junk", bad)
	}
	if !strings.Contains(bad[0].reason, "leak") || !strings.Contains(bad[1].reason, "junk first page") {
		t.Errorf("reasons: %q / %q", bad[0].reason, bad[1].reason)
	}
}

func TestAuditIncludeScoresThemAnyway(t *testing.T) {
	dir, papers := refDir(t)
	usable, bad, err := auditRefs(papers, auditConfig{Dir: dir, Include: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(usable) != 3 || len(bad) != 2 {
		t.Errorf("usable %d, unusable %d; want 3 scored and the 2 still named", len(usable), len(bad))
	}
}

func TestAuditNotRunWithoutADirectory(t *testing.T) {
	_, papers := refDir(t)
	usable, bad, err := auditRefs(papers, auditConfig{})
	if err != nil || len(usable) != 3 || bad != nil {
		t.Fatalf("no -refpdf must change nothing: %v %v %v", usable, bad, err)
	}
	var out bytes.Buffer
	reportUnusable(&out, nil, auditConfig{})
	if !strings.Contains(out.String(), "NOT RUN") {
		t.Errorf("a skipped audit must say so: %q", out.String())
	}
}

// An unread reference is not a clean one.
func TestAuditRefusesAnUnreadReference(t *testing.T) {
	dir, papers := refDir(t)
	papers = append(papers, Paper{ID: "absent"})
	if _, _, err := auditRefs(papers, auditConfig{Dir: dir}); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("a missing reference PDF was not refused: %v", err)
	}
}

func TestAuditRefusesWhenNothingIsUsable(t *testing.T) {
	dir, papers := refDir(t)
	if _, _, err := auditRefs(papers[:2], auditConfig{Dir: dir}); err == nil {
		t.Fatal("a corpus of only corrupt references was scored")
	}
}

func TestReportUnusableBlock(t *testing.T) {
	dir, papers := refDir(t)
	_, bad, _ := auditRefs(papers, auditConfig{Dir: dir})
	var out bytes.Buffer
	reportUnusable(&out, bad, auditConfig{Dir: dir})
	s := out.String()
	for _, want := range []string{"reference unusable: 2 paper(s)", "leaky", "junk", "left out of Σ|page deviation|"} {
		if !strings.Contains(s, want) {
			t.Errorf("block lacks %q:\n%s", want, s)
		}
	}
	out.Reset()
	reportUnusable(&out, bad, auditConfig{Dir: dir, Include: true})
	if !strings.Contains(out.String(), "ARE scored") {
		t.Errorf("-include-unusable must say they are scored:\n%s", out.String())
	}
	out.Reset()
	reportUnusable(&out, nil, auditConfig{Dir: dir})
	if !strings.Contains(out.String(), "no unusable reference") {
		t.Errorf("clean corpus not said: %q", out.String())
	}
}

func TestLoadAuditedWritesTheBlockAndDropsThem(t *testing.T) {
	dir, papers := refDir(t)
	root := t.TempDir()
	var list, refs strings.Builder
	for _, p := range papers {
		pd := filepath.Join(root, p.ID)
		if err := os.Mkdir(pd, 0o755); err != nil {
			t.Fatal(err)
		}
		list.WriteString(pd + "/main.tex\n")
		refs.WriteString("3\n")
	}
	lp, rp := filepath.Join(root, "list.txt"), filepath.Join(root, "refs.txt")
	if err := os.WriteFile(lp, []byte(list.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rp, []byte(refs.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	got, err := loadAudited(&out, lp, rp, auditConfig{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "good" || !strings.Contains(out.String(), "reference unusable: 2") {
		t.Errorf("papers %v, output %q", got, out.String())
	}
	// Each refusal must name ITS OWN cause, not the generic "nothing to score".
	if _, err := loadAudited(&out, filepath.Join(root, "absent.txt"), rp, auditConfig{Dir: dir}); err == nil || !strings.Contains(err.Error(), "absent.txt") {
		t.Errorf("a missing corpus list: %v", err)
	}
	missing := t.TempDir()
	if _, err := loadAudited(&out, lp, rp, auditConfig{Dir: missing}); err == nil || !strings.Contains(err.Error(), "could not be audited") {
		t.Errorf("an empty reference directory: %v", err)
	}
}
