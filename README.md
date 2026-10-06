# engine — go-tex

[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27%2B-00ADD8)](https://go.dev/dl/)
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
  `\catcode`, read via `\the`/`\count`; TeX's own number syntaxes in any
  numeric slot (`"FF`, `'17`, `` `A ``) and the glyph metrics
  `\fontcharht`/`\fontchardp`/`\fontcharwd`/`\fontcharic`, which are internal
  dimensions wherever one is read.
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
report what was dropped. `Diagnostics` also records the paths the document read from
**outside its own directory** — an absolute path, or one that walked out with `../`,
excluding the search directories the host itself configured. Reading a named file is
what `\input` is for and it is not an error, but a service compiling a document it did
not write is the one who can judge it, and nothing else in this package will tell it. On a **1000-source arXiv sweep** (measured 2026-10-04),
strict mode compiles almost none end-to-end — each hits a package command in the
preamble — while lenient mode produces a multi-page PDF for all but **one** of
the 71 sources the sweep flags as truncated; that one still stops at a single page
(tracked in #517, which also records its diagnosis). The same sweep counted twelve
such papers when the series began. It is a preview aid, not a fidelity claim — the roadmap
below is how the gaps close for real.

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
with no TeXLive and no server.** `amsart` now joins them: `\documentclass{amsart}`
loads the real embedded `amsart.cls` and typesets its own title, sections and
theorem heads (its `\maketitle` drove real token-register support — `\toks`/
`\newtoks` — into the engine, and its `\newtheorem…[section]` machinery, which
used to loop, runs through the class-kernel substrate's counter hooks).
`GOTEX_AMSART=0` forces the old emulation back, for comparing the two paths.

### Supported classes & packages, at a glance

The full, honest matrix — every supported class, package, primitive, output
format and image type, with its status, plus every remaining gap — lives at
[go-tex.github.io/docs](https://go-tex.github.io/docs/). In short:

- **Classes (real embedded):** `article`, `report`, `book`, `amsart`. `beamer`
  runs its real class when resolvable. `revtex4-x`, `acmart`, `IEEEtran`,
  `elsarticle` and the `aastex6x` family fall back to content-preserving
  emulation. Any other resolvable `.cls` is loaded and run as real TeX.
- **Packages with native handling:** `amsmath` (equation/align/gather/multline/…),
  `amssymb`, `amsthm`, `thmtools` (`\declaretheorem`, including its comma list of
  names and both of its counter keys), `graphicx`, `xcolor`, `hyperref`,
  `geometry`, `fancyhdr`,
  `setspace`, `enumitem`, `multicols`, `booktabs`/`multirow`/`tabularx`,
  `subcaption`, `algorithm`/`algorithmic`, `listings` (`caption=`, `label=` and
  `captionpos=`, so a listing is numbered, captioned and referable; and
  `basicstyle=` through `\lstset`, `\lstdefinestyle` and `style=`, so code set
  at `\footnotesize` or `\scriptsize` takes the room it takes in the reference),
  `minted`, `siunitx`,
  `cleveref` (`\cref`/`\Cref` with cleveref's own default naming, its
  `capitalise` and `noabbrev` options, `\crefname`/`\Crefname` and
  `\crefformat`), `caption`'s `\captionsetup{name=}`, `longtable`, `url`,
  paragraph columns (`p{}`, with `m`/`b` treated as `p`),
  `numprint`, `makeidx`, `verbatim`, BibTeX. Any other resolvable `.sty` runs as
  real TeX macros through the full LaTeX2e option mechanism.
- **Opt-in and working:** PDF-figure rasterization. `GOTEX_PDFRENDER=1` wires
  `go-tex/pdfrender` into the CLI and a vector `.pdf` figure typesets as a real
  raster instead of a framed placeholder — measured on one corpus paper, 21
  figures reported as empty boxes become 0. It is off by default so the engine
  core, and the `js/wasm` build with it, carries no PDF renderer.
- **Partly:** `biblatex`. Biber's data `.bbl` is parsed and its entries are
  typeset — on the two corpus papers that ship one, 30 of 30 and 15 of 17 titles
  are present and no raw field data reaches the page. Its citation commands are
  **not** there: `\textcite`, `\parencite`, `\autocite`, `\footcite` and
  `\smartcite` are undefined and print the bare key, and the citation and
  bibliography *styles* are not modelled.
- **Not yet:** TikZ/pgf drawing (gated behind `GOTEX_PGF`, in bring-up), full
  float pagination (`GOTEX_FLOATS`), two-column reprint layouts
  (`GOTEX_TWOCOLUMN`), EPS graphics (an `.eps` is *measured* from its
  `%%BoundingBox`, so its placeholder has the right shape, but it is not drawn),
  `rotating`'s sideways floats (`sidewaystable`/`sidewaysfigure` are undefined,
  so the body sets inline and its caption carries no number), and the
  XeTeX/LuaTeX Unicode engines / `fontspec`.

## Compiling a document you did not write

A `.tex` is a program and this package is its interpreter. Lenient mode is
offered above as a preview for "a real third-party paper", the fidelity work runs
the engine over arXiv sources in bulk, and the playground runs it on whatever a
visitor pastes. Four properties are therefore held on purpose rather than by
accident, each with a test that fails if it stops being true:

- **A document cannot run a process.** No `os/exec`, no `syscall.Exec` — TeX's
  `\write18` shell escape has no implementation here. Asserted by a scan of the
  package's own source, since there is no API that could express the property.
- **A document cannot write a file.** No `os.Create`, `os.WriteFile`,
  `os.Remove`, `os.Rename`… in the engine package. (`cmd/` is deliberately out of
  scope: the CLI writes the PDF it was asked for.)
- **A document cannot read a file outside the search roots.** `\input`,
  `\include`, `\usepackage`, `\documentclass` and `\bibliography` resolve only
  under the working directory and the `TEXINPUTS`/`GOTEX_TEXMF` entries — the
  same search path the engine already looked in. This is TeX Live's
  `openin_any=p` policy. `GOTEX_READ_ANY=1` is a named opt-out for a macro tree
  deliberately kept outside the document.
- **A document cannot exhaust memory or run forever.** A macro that expands
  exponentially — ten macros, each ten copies of the next — asks for 10¹⁰
  characters while taking *few* expansion steps and keeping a *shallow* input
  stack, so the step and depth ceilings could not see it: it reached **15.96 GB**
  and was still growing. One paragraph is now bounded (200 000 nodes, against a
  largest-real-paragraph of 8 881 measured over the fidelity corpus) and the run
  **ends** rather than recovering, as TeX's capacity error does. The rest was the
  hyphenator: a document with no space in it is one enormous word and Liang's
  algorithm is quadratic in a word's length, so TeX's own limit applies — no word
  longer than 63 letters is hyphenated (`tex.web` §891). The same file now fails
  in **0.06 s at ~90 MB**.

What that does **not** cover, stated rather than glossed over:

- `\font` and `\includegraphics` read by absolute path, because each *decodes*
  its file and the engine loads system fonts from `/System/Library/Fonts`. A file
  that is neither a font nor an image yields an error and no content, so nothing
  is spliced into the page — but the *shape* of that error still tells a document
  whether a path exists.
- The first two properties are read off the source, not enforced at runtime: they
  say the package contains no such call, which is what makes a commit that adds
  one fail the test and have to argue the case.
- The **CLI** may use the network. `gotex` fetches the TeXMF bundles a document
  asks for (`\usepackage{pgf}` and friends) unless `-offline` is given, so the
  document influences what is downloaded — but only names a closed registry knows
  become bundles, so it cannot aim the fetch somewhere of its own choosing. To
  decide which bundles are needed the CLI also reads the document's **class file**,
  one level deep and by NAME only: a `\documentclass` argument carrying a path
  separator is not read, because that read happens a layer above the engine's own
  policy. The engine package itself opens no connection.

`govulncheck` reports no called vulnerability under the pinned toolchain
(`go 1.27.1`); a 1.26.4 build of the same code called six, among them an
`encoding/xml` recursion bomb reachable from an SVG figure.

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
   text, and clickable `/Link` annotations for `\href` URIs and `\hyperlink`
   in-document jumps) and self-contained **SVG** pages; the SVG carries a source
   map for click-to-line.
6. ✅ **Real classes** — `\documentclass{article|report|book|amsart}` loads and
   runs the genuine embedded LaTeX class (see above), reproducing the reference
   engine's prose on the fidelity gate — in native builds **and** in `js/wasm`.

Next: real TikZ/pgf (behind `GOTEX_PGF` today), float pagination and two-column
reprint layouts out of their env flags, PDF-figure rasterization and clickable
PDF links, a broader real-document conformance corpus (PDF-diff vs pdftex/xetex),
and the TRIP test — see the [capability reference](https://go-tex.github.io/docs/)
for the complete list of gaps.
the meaningful gate is the conformance ratchet plus the fidelity check against a
real LaTeX engine, not a fixed coverage figure — CI enforces an 80% floor and
measures 91.0% today. Pure Go, CGO=0, `go vet` clean, green on the host plus
four cross arches under qemu — **s390x** (big-endian), `ppc64le`, `riscv64` and
`loong64` — and on `js/wasm` and `wasip1/wasm`.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-tex/engine authors.
