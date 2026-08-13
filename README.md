# engine — go-tex

[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![status](https://img.shields.io/badge/status-engine%20core%20(in%20progress)-orange)](#status)

**A pure-Go (no cgo) TeX engine aimed at functional parity with a TeX
distribution, not a subset.** It is a faithful re-implementation of TeX — the
category-code tokenizer, the equivalents table (`eqtb`) with grouping/scoping,
macro definition with **delimited parameters** and the full **expansion**
machinery (the mouth and gullet); a scaled-point box/glue/penalty **stomach**
with Knuth–Plass line breaking and a cost-based page builder; math via
`go-tex/math`; OpenType fonts; and **PDF + SVG** output — and on top of it, it
**loads and runs the genuine LaTeX classes**: `\documentclass{article}`,
`{report}` and `{book}` execute the real, embedded `.cls` files, in native builds
and in the browser (`js/wasm`), with no TeXLive.

It is developed the way parity is actually reachable — **reimplement the engine
faithfully, gated by objective oracles** — then run the *real* LaTeX classes and
packages on it (they are TeX macros). Two gates hold the line: the **conformance
ratchet** (`TestConformance`, TeX snippets checked byte-for-byte against real-TeX
output) and a **fidelity check** that compares whole-document prose against a real
LaTeX engine (`tectonic`).

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
disk.

**The standard base classes run for real.** `\documentclass{article}`,
`{report}` and `{book}` load and execute the **genuine, embedded LaTeX classes**
(`article.cls`/`report.cls`/`book.cls` + their size option files, LPPL, verbatim)
— not an emulation. Everything they need is in place: the LaTeX2e kernel helpers,
a class-kernel substrate (constants, registers, `\if@` flags, NFSS font-switch
aliases), `\newcommand*`/`\DeclareOldFontCommand`, the rubber-glue and
`<factor><internal-dimen>` length scanner, numbered `\@startsection` with
`\@tocentry`, `\secdef` via `\@dblarg` (so `\chapter` works), `\@float`
figure/table captions, `\@starttoc` bridged to the engine's two-pass contents
table, and — the keystone — **stable source lines** (loading a 644-line class no
longer shifts the line numbers the editor maps glyphs back to). A real
`\documentclass{article}` document typesets a numbered title, a dotted
`\tableofcontents`, numbered sections, and numbered figure/table captions, and it
reproduces the reference engine's prose on the fidelity gate. Because the class
files are `go:embed`ed and the resolver needs no filesystem, **the real classes
also run in the `js/wasm` build — genuine LaTeX class rendering in the browser,
with no TeXLive and no server.** (`amsart` still uses the emulation; it
additionally needs amsmath.)

## Status & roadmap to parity

Each stage is gated by an objective oracle:

1. ✅ **Mouth + gullet** — tokenizer, `eqtb`, macros, expansion.
2. ✅ **Stomach** — box/glue/penalty model in scaled points, h/v lists,
   Knuth–Plass line breaking with an emergency pass, cost-based page builder,
   `\halign`.
3. ✅ **Math** — `$…$` and the display environments delegated to `go-tex/math`
   (vector output).
4. ✅ **Fonts** — OpenType via `go-opentype`; a built-in font so it runs with no
   assets, with kerning and ligatures.
5. ✅ **Output** — **PDF** (via `go-pdfkit`, embedded subset fonts, selectable
   text) and self-contained **SVG** pages; the SVG carries a source map for
   click-to-line.
6. ✅ **Real classes** — `\documentclass{article|report|book}` loads and runs the
   genuine embedded LaTeX class (see above), reproducing the reference engine's
   prose on the fidelity gate — in native builds **and** in `js/wasm`.

Next: more real classes/packages (`amsart` + `amsmath`), a broader real-document
conformance corpus (PDF-diff vs pdftex/xetex), and the TRIP test. Coverage ~91%;
the meaningful gate is the conformance ratchet plus the fidelity check against a
real LaTeX engine, not a fixed coverage figure. Pure Go, CGO=0, `go vet` clean,
green across three 64-bit arches under qemu plus `js/wasm` and `wasip1/wasm`.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-tex/engine authors.
