#!/usr/bin/env bash
# Copyright (c) the go-tex/engine authors.
# SPDX-License-Identifier: BSD-3-Clause
#
# fidelity.sh — compare gotex's output against a real LaTeX engine on the same
# documents, as a content-fidelity gauge. For each testdata/fidelity/*.tex it
# compiles a reference PDF with `tectonic` (a self-contained LaTeX engine) and a
# gotex PDF, extracts the text of both with `pdftotext`, and reports how much of the
# reference text gotex reproduces. Text extraction is robust to the font difference
# between the two engines, so it measures whether the SAME content is typeset — not
# pixel parity (the engines use different fonts and line-breaking).
#
# It is a developer tool, not a CI gate: it needs network (tectonic downloads its
# package bundle) and poppler. Run from the repo root:
#     GONOSUMDB=off ./scripts/fidelity.sh
# Requirements (all available via pkgx): tectonic, pdftotext (poppler), go.
#
# Known, expected differences (NOT regressions):
#   * gotex defaults to \pagestyle{empty}; LaTeX to plain — the bottom page number
#     "1" appears in the reference only.
#   * math is rendered by gotex as vector paths, so equation glyphs are not
#     extractable text in the PDF (the SVG output is the selectable/interactive form).
#   * ligatures (fi/fl/…): gotex's PDF font subset does not yet map the ligature
#     glyph back to its component letters in ToUnicode, so a word like "first" is
#     extracted as "rst". This is a real, tracked limitation of the PDF text layer.
set -u

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

tectonic() { pkgx tectonic "$@"; }
pdftotext() { command -v pdftotext >/dev/null && command pdftotext "$@" || pkgx pdftotext "$@"; }

echo "building gotex…"
( cd "$root" && GOWORK=off go build -o "$work/gotex" ./cmd/gotex ) || exit 1

# norm extracts a document's words, lower-cased and one per line. Ligatures are
# folded to their component letters first (gotex keeps them as the Unicode ligature
# in /ToUnicode, real LaTeX decomposes them to ASCII — folding measures the content
# fairly; the ASCII-search limitation is noted above), so "ﬁrst" counts as "first".
norm() {
	pdftotext "$1" - 2>/dev/null |
		sed 's/ﬀ/ff/g; s/ﬁ/fi/g; s/ﬂ/fl/g; s/ﬃ/ffi/g; s/ﬄ/ffl/g' |
		tr 'A-Z' 'a-z' | tr -cs 'a-z0-9' '\n' | grep -v '^$' | sort -u
}

printf '%-8s %8s %8s %8s   %s\n' doc ref_words gotex common notes
for tex in "$root"/testdata/fidelity/*.tex; do
	d="$(basename "$tex" .tex)"
	tectonic -o "$work" "$tex" >/dev/null 2>&1
	"$work/gotex" -o "$work/$d.got.pdf" "$tex" >/dev/null 2>&1
	norm "$work/$d.pdf" > "$work/$d.ref.txt"
	norm "$work/$d.got.pdf" > "$work/$d.got.txt"
	rw=$(wc -l < "$work/$d.ref.txt" | tr -d ' ')
	gw=$(wc -l < "$work/$d.got.txt" | tr -d ' ')
	common=$(comm -12 "$work/$d.ref.txt" "$work/$d.got.txt" | wc -l | tr -d ' ')
	missing=$(comm -23 "$work/$d.ref.txt" "$work/$d.got.txt" | tr '\n' ' ')
	printf '%-8s %8s %8s %8s   %s\n' "$d" "$rw" "$gw" "$common" "${missing:+missing: $missing}"
done
