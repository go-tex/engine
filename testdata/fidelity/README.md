# Fidelity comparison vs a real LaTeX engine

`../../scripts/fidelity.sh` compiles each `*.tex` here with **gotex** and with
**tectonic** (a self-contained real LaTeX engine), extracts the text of both PDFs
with `pdftotext`, and reports how much of the reference text gotex reproduces. It
measures *content* fidelity — is the same text typeset — not pixel parity (the two
engines use different fonts and line-breaking, so a pixel diff is meaningless).

It is a developer gauge, not a CI gate (it needs network for tectonic's package
bundle and `pdftotext`). Run from the repo root:

```
GOWORK=off ./scripts/fidelity.sh
```

## Latest result (2026-08-11, gotex v0.85.0 vs tectonic 0.17.0)

| doc   | ref words | gotex | common | note |
|-------|-----------|-------|--------|------|
| basic | 24 | 24 | **24** | full parity |
| geom  | 24 | 24 | **24** | full parity (geometry margins) |
| color | 14 | 13 | 13 | only the page number "1" differs |
| table | 10 |  9 |  9 | only the page number "1" differs |
| math  | 17 | 10 | 10 | equation glyphs are vector, not text |

**gotex reproduces 100 % of the extractable prose text of real LaTeX.** The only
differences are two documented, deliberate ones:

1. **Page number.** gotex defaults to `\pagestyle{empty}`; LaTeX's article class
   defaults to `plain` (a bottom-centred number). So the reference has a stray "1"
   that gotex omits until `\pagestyle{plain}` is requested.
2. **Math is vector.** gotex renders math through go-tex/math to vector paths, so
   equation glyphs are *drawn*, not selectable PDF text. The **SVG** output carries
   math as a live, inspectable `<svg>` — the engine's differentiator — but the PDF
   text layer does not include the equation characters. (Prose is real, selectable
   PDF text.)

## Ligature PDF text (fixed)

This harness first surfaced that ligated words were not ASCII-searchable in the PDF:
"first" was drawn with the ﬁ ligature glyph and mapped to U+FB01 in `/ToUnicode`, so
a copy or search recovered "ﬁrst" (Unicode) but not "first" (ASCII). The PDF driver
now draws f-ligatures through `TextShaped(components, "liga")`, so the font's liga
feature still produces the ligature glyph while the text layer stays ASCII — the
ligature renders identically AND "first" is searchable. (The harness still folds
ligatures before comparing, which is harmless now that both sides are ASCII.)
