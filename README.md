# engine — go-tex

[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![status](https://img.shields.io/badge/status-engine%20core%20(in%20progress)-orange)](#status)

**A pure-Go (no cgo) TeX engine, under construction — aimed at functional parity
with a TeX distribution, not a subset.** This module is the engine's **mouth and
gullet**: a faithful re-implementation of TeX's category-code tokenizer, the
equivalents table (`eqtb`) with grouping/scoping, macro definition with
**delimited parameters**, and the full **expansion** machinery.

It is developed the way parity is actually reachable — **reimplement the engine
faithfully, gated by TeX's own conformance oracle** — and later run the *real*
LaTeX kernel and packages on it (they are TeX macros), diffing output against
`pdftex`/`xetex`. The primary gate here is the **conformance ratchet**
(`TestConformance`): TeX snippets checked byte-for-byte against real-TeX output.

## Working today (verified, faithful)

- **Category-code tokenizer** — full catcode table, comments, control words/symbols.
- **Macros** — `\def` (undelimited, **delimited**, and grouped parameters, with
  backtracking on partial delimiter matches), `\edef`/`\gdef`/`\xdef`, `\let`
  (to macros, primitives, undefined, and character tokens), `\global`.
- **Expansion** — `\expandafter`, `\csname`/`\endcsname`, `\noexpand`, `\string`,
  `\the`, `\number`, `\romannumeral`, `\meaning`, `\uppercase`/`\lowercase`.
- **Conditionals** — `\if`, `\ifnum`, `\ifx`, `\ifcat`, `\ifodd`, `\ifcase`,
  `\iftrue`/`\iffalse`, with `\else`/`\or`/`\fi` and nesting.
- **Registers & arithmetic** — `\count`, `\advance`, `\multiply`, `\chardef`,
  `\catcode`, read via `\the`/`\count`.
- **Grouping** — `{…}`, `\begingroup`/`\endgroup`, save/restore of meanings,
  registers, and catcodes; `\global` escapes the current group.

Faithfulness is checked on subtleties only real TeX gets right (a control word
absorbing its following space; significant spaces in conditional branches).

```go
out, _ := engine.New().Run(`\def\twice#1{#1#1}\message{\twice{\twice A}}`)
// out == "AAAA"
```

## Real-world documents (lenient mode)

A real third-party paper pulls in classes, packages, fonts, figures and `.bib`
files that a from-scratch engine does not carry. Strict mode aborts on the first
such gap (as TeX does). **Lenient mode** (`gotex -lenient`, or `Options{Lenient:
true}`) turns those gaps into best-effort no-ops so an editor preview shows the
typesettable content instead of one hard error:

- an **undefined command** is skipped, along with its likely `[opt]{arg}` block;
- an **unloadable figure** becomes a framed placeholder of the requested size;
- a **math macro** go-tex/math doesn't know drops that one equation;
- a **`\setlength` on an unmodelled length**, and a missing `\input`/`\bibliography`/
  `\font` **file**, are ignored.

Every skipped construct is tallied (`(*Engine).SkippedCommands`) so a caller can
report what was dropped. On a sample of **54 real arXiv sources**, strict mode
compiled 0 end-to-end (each hit a package command in the preamble); lenient mode
produces a multi-page PDF for **all 54**, with real, selectable prose text. It is
a preview aid, not a fidelity claim — the roadmap below is how the gaps close for
real.

### Loading real classes and packages

`\documentclass` and `\usepackage` (and `\RequirePackage`, `\LoadClass`,
`\LoadClassWithOptions`) do more than emulate: they **resolve and load the real
`.cls`/`.sty`** — from the document's own directory (an arXiv paper's bundled
class/package), a `TEXINPUTS`/`GOTEX_TEXMF` search path, or an embedded base set —
making `@` a letter and running the file's own `\newcommand`/`\def`/… on the
engine. The LaTeX2e option mechanism runs too: `\DeclareOption`,
`\DeclareOption*`, `\ProcessOptions`, `\ExecuteOptions`, `\CurrentOption`,
`\PassOptionsToPackage`/`\PassOptionsToClass`, plus `\IfFileExists`/
`\InputIfFileExists`. A file loads *tolerantly* — a command the engine lacks is
skipped, so a real class contributes what it can — and a **runaway-expansion
guard** bounds macro expansion so a pathological or partially-supported file can
never hang (it stops with partial output in lenient mode, an error in strict).
Distribution-heavy packages the engine emulates natively or better as stubs
(`geometry`, `tikz`, `hyperref`, `graphicx`, encodings, …) are not loaded from
disk. Base classes (`article`, `amsart`) are the next embed — the kernel gap to
run them is mapped.

## Status & roadmap to parity

This is **stage 1** (mouth + gullet). The remaining stages, each gated by an
objective oracle:

1. ✅ **Mouth + gullet** — tokenizer, `eqtb`, macros, expansion (this module).
2. ⬜ **Stomach** — box/glue/penalty model, h/v lists, **Knuth–Plass** line
   breaking, page builder, `\halign` — gated by the **TRIP test**.
3. ⬜ **Math** (Appendix G) — `go-tex/math` is the starting point.
4. ⬜ **Fonts** — TFM + OpenType (via `go-opentype`).
5. ⬜ **Output** — DVI → **PDF** (via `go-pdfkit`), then run the real
   `latex.ltx` + packages, gated by **PDF-diff vs pdftex/xetex**.

Because it is under construction, line coverage (currently ~84%) rises as
subsystems land; the meaningful gate is the growing conformance ratchet, not a
fixed coverage figure. Pure Go, CGO=0, `go vet` clean, green across six 64-bit
arches plus `js/wasm` and `wasip1/wasm`.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-tex/engine authors.
