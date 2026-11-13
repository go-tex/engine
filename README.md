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
  dimensions wherever one is read. **`\showthe` prints what `\the` would give**, on a
  diagnostic stream of its own (`Diagnostics.Shows`, and `gotex` writes it
  unconditionally): TeX sends `\message` to the log and the terminal but `\showthe` to
  the terminal, and folding the two together makes a
  document's own diagnostics indistinguishable from its output. The shape is
  `print_nl("> "); token_show(…)` (`tex.web` 23721-23725) followed by `error`, and the
  trailing full stop is `error`'s own first act (`print_char(".")`, 1883-1884) — not
  part of the value.
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
  ⛔ "likely" is the catch, and it is why a command that *takes* an argument is worth
  defining even when its effect is not implemented: `\marginpar` was undefined, and in
  lenient mode its argument reached the page as ordinary text —
  `AVANTUN\marginpar{NOTEUN}APRESUN` set `AVANTUNNOTEUNAPRESUN`, and the optional form
  set the first argument *with its brackets*, where `tectonic` puts `AVANTUNAPRESUN` and
  the note out in the margin. It is now defined and reads both arguments
  (`ltfloat.dtx` 951-967, 973-985), so the body text matches the reference exactly;
  placing the note in the margin needs `\vadjust` (#665) and is still missing;

- an **unloadable figure** becomes a framed placeholder of the requested size
  (this includes a raster figure above 40 megapixels, see below);
- a **math macro** go-tex/math doesn't know drops that one equation;
- a **`\setlength` on an unmodelled length**, and a missing `\input`/`\bibliography`/
  `\font` **file**, are ignored.

**Raster figures above 40 megapixels.** `\includegraphics` decodes a PNG, JPEG,
GIF, WEBP, TIFF, BMP, ICNS or ICO figure through `go-gfx/gfx`, which since v0.37.0
refuses an image of more than `codec.DefaultMaxPixels` (40 million pixels, width
times height) from its header, before allocating anything: the size is read from
the file, and a compressed format describes a huge image in a few bytes. Such a
figure fails with an error wrapping `codec.ErrTooLarge`: in strict mode the
compile stops (`includegraphics <name>: …`), in lenient mode it becomes the
framed placeholder above and is recorded as a dropped figure. There is no option
to raise the ceiling; downscale the figure.

Every skipped construct is tallied (`(*Engine).SkippedCommands`) so a caller can
report what was dropped. `Diagnostics` also records the paths the document read from
**outside its own directory** — an absolute path, or one that walked out with `../`,
excluding the search directories the host itself configured. Reading a named file is
what `\input` is for and it is not an error, but a service compiling a document it did
not write is the one who can judge it, and nothing else in this package will tell it.

On a **1000-source arXiv sweep** (`measure trunc`, re-measured **2026-10-19**), strict
mode compiles almost none end-to-end — each hits a package command in the preamble —
while lenient mode flags **64** sources, and the engine's two alarms do not say the
same thing:

| | sources |
|---|---:|
| runaway guard tripped, alone | 2 |
| groups still open at end of document, alone | 58 |
| both | 4 |
| **either** | **64** |
| rendered as a SINGLE page | **1** |

An aborted loop has lost content; groups left open may be only a leak — seven papers
measured the same day carry that alarm and render 14 to 42 pages, and one leaves 20
groups open over 17 pages. The number that is not a suspicion is the last row, and it
is **one source in a thousand** (`2607.05497`, `aas.cls`, tracked in #625). The same
sweep counted **twelve** when the series began, and #517 records each one's diagnosis.

It is a preview aid, not a fidelity claim — the roadmap below is how the gaps close
for real.

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
table, `\addcontentsline` bridged to it as well — `\chapter` and `\part` are the
two entries no `\@startsection` records, which is why a book's contents list
used to have no chapters in it — and — the keystone — **stable source lines** (loading a 644-line class no
longer shifts the line numbers the editor maps glyphs back to). A real
`\documentclass{article}` document typesets a numbered title, a
`\tableofcontents` shaped by **the loaded class's own `\l@…` macros** — the
indent and number-box widths are read out of its `\@dottedtocline` calls, so
`article`'s leaderless bold `\l@section` and `book`'s dotted one at the very
same level come out differently, and the leader tiles `\@dotsep` rather than
`\dotfill`'s narrower box —
numbered sections, and numbered figure/table captions, and it
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
  `geometry`, `fancyhdr` (and the class page styles beside it: a
  `\thispagestyle{plain}` page — every chapter opening in a book or report, and
  the first page of its contents list — keeps its folio centred at the foot even
  though the class declared a running head for the whole document; and the
  running head itself now carries **its own page's** mark, because `\mark`
  travels with page breaking and `\topmark`/`\firstmark`/`\botmark` are
  maintained per page as `tex.web` §1012 maintains them, so a head that tracks
  sections names the section the reader is in rather than the document's last
  one),
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
- **The front matter stays on the sheet.** A vertical block that should fill a
  title page — an author/affiliation list, an abstract — used to be emitted as one
  horizontal list that was never line-broken, so everything past the right margin
  was *drawn off the paper*: measured at up to **x=6619pt on a 612pt sheet**, with
  author names, affiliations and e-mails outside it. No other channel can see that
  — the page count cannot (the clearest witness is 15 pages against 15), the ink
  count cannot (every glyph is emitted), `-report-skipped` cannot (nothing is
  skipped) and the text layer carries the words. Seven distinct causes, each with
  its own witness: a macro standing for several `tabular` rows (`article.cls` sets
  the author block as `\begin{tabular}[t]{c}\@author\end{tabular}` and `authblk`
  builds `\@author` with `\\`), a bare `\endX` closing what `\X` opened
  (`sagej.cls` writes `\def\endabstract{\endminipage\endlrbox}`), `calc`
  expressions in a length argument (`{\textwidth-57.625mm}`), `\cite`'s optional
  note, **`\unvbox` inside a paragraph** — a vbox holds finished *lines*, and
  handing them to the line breaker glues them end to end and re-breaks the lot as
  one paragraph (acmart writes `\noindent\unvbox\mktitle@bx`, so its title and
  every author block came out as a single line) — **`\maketitle` never running the
  class's own `\@maketitle`**, which left `\@author` in a `\centerline` no line
  breaker touches, and a package whose key-list commands were undefined and left
  their keys behind (`mdframed`, `caption`).

  Over the 154-paper corpus **21 papers** still draw text past their own page edge
  in their first two pages, the widest line at **1281pt**, against a floor of **0**
  on the reference PDFs themselves. ⚠ That floor is 0 and not 2 because the
  instrument changed: it lives in the measure harness now (`measure offpage`) and
  filters text the PDF reader surfaces but that was never *typeset* — a `/Title`
  string's octal escapes, hyperref's PostScript specials, pdfmark attributes,
  `burl`'s PostScript definitions. Unfiltered the reference itself reports 12, one
  of them at x=5135. The earlier series — 49 at v0.260.0, 23 at v0.261.0 — was taken
  with a filter that left 2 there, so it is **not** directly comparable to these.
  ⚠ Not closed: 21 remain, and the top of that list is a different population again
  (see [#629](https://github.com/go-tex/engine/issues/629)).
- **Float placement** takes its seven parameters from the **class**, not from
  article's: `\topnumber`, `\bottomnumber` and `\totalnumber` are counters the
  class kernel declares so a `\setcounter` of them takes (amsart's
  `\setcounter{topnumber}{4}` used to be a no-op on an undeclared counter), and
  `\topfraction`, `\bottomfraction`, `\textfraction` and `\floatpagefraction` are
  read from their macros. Measured against the reference on forty figures in
  amsart prose: four floats to a page on both sides, 11 pages each, where
  article's hardcoded values gave three and 14.
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
  in **0.06 s at ~90 MB**. The same expansion routed into a *macro* rather than
  onto a page took a second ceiling: `\edef\boom{…}` never builds a paragraph, so
  it passed the first one untouched at **6.3 GB** while the typeset form stopped
  at 97 MB. One replacement text is now bounded too (1 000 000 tokens, against a
  largest-real-replacement-text of 2 999 measured over the same corpus), and that
  file returns in **0.38 s at 99 MB** — against tectonic's 236 MB, where TeX's
  `main_memory` answers the same shape.

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
   `\halign`. The page builder keeps TeX's **two** page measurements apart:
   `page_total`, which is what the break is decided on, and `page_depth`, which is
   not charged against the goal and is capped at `\maxdepth` (`tex.web` 19519-19529,
   19594-19599). Summing the two, as this did, refused a page that filled to exactly
   `\textheight` over the descender of its own last line and cost it a line
   (#622, #648). The page also carries its **glue**: `page_so_far` is eight numbers in
   TeX (`tex.web` 19226-19232) — the goal, the total, the depth, three orders of
   infinite stretch, the finite one and the shrink — and the badness that decides a
   break is a function of the shortfall *over the glue available to absorb it*
   (19593-19599), not of two heights. A page ending in `\vfill` is badness 0 to TeX and
   was ~10000 here. That badness is now TeX's own **integer** function (§108, with its
   three overflow branches and the octal rounding constants) rather than a float
   `100·(t/s)³`: the two agree on ordinary values and diverge exactly where TeX's
   rounding decides a close break. Since a breakpoint's cost is `badness + penalty`,
   every penalty was until then being compared against numbers on the wrong scale. `\hspace` and `\vspace` read a **glue**, as the reference does by
   putting their argument through a skip register (`ltspace.dtx` 1412-1413, 450):
   reading a rigid dimen kept the natural width and dropped the stretch, so
   `\hspace{\fill}` became a 0pt space and the words on either side were set
   adjoining — 188.50 where `tectonic` puts the second one at the right margin,
   477.48. `\fill`, `\enskip` and `\samepage` are defined, and `\stretch` is the
   reference's **second** order of infinity (`0pt plus n fill`, l.1454) rather than
   the first, which decides who wins the leftover space on a list that also holds a
   `\hfil`. `\vfil`/`\vfill`/`\vss` contribute their glue at the reference's orders
   (`tex.web` §20557-20559, §20600-20601, §3318-3320), and `\vfill` is the **second**
   order where a macro in the format used to shadow the primitive with the first (#674).
   **A footnote's mark is whatever `\thefootnote` expands to**, not digits built in
   Go: the counter is the TeX counter `\c@footnote` (`latex.ltx:13164-13165`), so
   `\renewcommand\thefootnote{\fnsymbol{footnote}}` works, and `\maketitle` scopes it
   to `\@fnsymbol` as `article.cls:205` does — a title block's `\thanks` comes out
   `∗`, not `1`, as it does in the reference. The foot label carries no full stop
   (`\@makefnmark` has none); this engine wrote `1.` and a space.
   **And a note's layout is `\@makefntext`, called**, not a row built in Go: the three
   base classes right-align the mark in a `1.8em` box
   (`\hb@xt@1.8em{\hss\@makefnmark}#1`, `article.cls:609-612`, `report.cls:708`,
   `book.cls:714`), so every note's text used to begin one box-width to the left of
   the reference's — measured as no box of `14.400pt` anywhere on the page before and
   one now, with `72 of the 154` corpus papers rendering differently and Σ\|deviation\|
   `277 → 276`. The mark reaches it through `\@thefnmark` and `\@makefnmark`
   (`latex.ltx:13168`, `:13179`), both replaceable by a document. `\@textsuperscript`
   is still the identity here, so the mark is set at text size on the baseline where
   the reference raises it — a separate divergence, unchanged.
   **`\footnoterule` is a macro the page builder executes**, not three nodes built in
   Go: `60 of the 154` measured papers redefine it, and what this engine drew was
   `article.cls:605-608`'s `.4\columnwidth` — right for article and its relatives by
   coincidence, where the kernel's own rule is a fixed `2in`
   (`latex.ltx:13162-13163`). `\let\footnoterule\relax` now draws **nothing**, which is
   what `article.cls:172` asks for on a title page.
   **A citation's brackets are `\@biblabel` and `\@cite`, called**, not brackets built in
   Go: the reference makes both a macro (`latex.ltx:13324`, `:13322`) and
   `article.cls:568` builds the bibliography list's label by calling `\@biblabel`. This
   engine runs real `.cls` files, so a class's redefinition was already in the eqtb and
   was dropped at the point of use — a document writing `\def\@biblabel#1{<<#1>>}`
   rendered **byte-identically** to one that did not. `22 of the 154` measured papers
   carry a class that redefines one of the two (`19` for `\@biblabel`, `12` for
   `\@cite`), and only 50 of the 154 ship a class at all, so that is **44 %** of the
   papers that bring their own; `18 of 154` render differently. `\if@tempswa` is set
   rather than inherited, as `latex.ltx:13283-13284` sets it, so a redefined `\@cite`
   can decide whether to print the note. `\@citex` — the iteration natbib replaces — is
   **not** executed; one paper in 154 ships `natbib.sty`
   ([#681](https://github.com/go-tex/engine/issues/681)).
   **A package can talk to itself across passes, so the real `natbib` runs.**
   `\@auxout` was `\def\@auxout{-1}`, a number no `\openout` ever opens, so
   everything a package wrote to the `.aux` went nowhere; it is a real stream
   now, carried to the next pass beside the label tables. Given the genuine
   `natbib.sty`, `\citep`/`\citet` in `authoryear` mode produce word for word
   what `tectonic` produces, with no undefined commands skipped
   ([#712](https://github.com/go-tex/engine/pull/712)). What is left is
   *reaching* the file rather than running it: `20 of the 154` measured papers
   `\usepackage{natbib}` and **none of those twenty ships it**, so all twenty
   fall back to the built-in emulation, where `\citet` loses the author name
   outright — "obtained by [27]" where LaTeX writes "obtained by Nordbotten
   et al. [27]" ([#703](https://github.com/go-tex/engine/issues/703)).
   **A display chooses between the long and the short skips.** `tex.web:22578-22585`
   takes `\abovedisplayshortskip`/`\belowdisplayshortskip` when the previous line ends
   to the *left* of where the display begins — they do not collide, so little space is
   needed — and this engine always took the long pair. Measured baseline to baseline
   across one display between two text lines: `11.96`/`17.93` in the reference against
   our `21.92`/`21.92`, now `11.96`/`17.93` **exactly**. A document of 40 displays went
   from 6 pages to the reference's 5. On the corpus the signed page bias falls from
   `+50` to **`+35`** and the median absolute deviation reaches **0** — but Σ\|deviation\|
   moves only 276 → 275: every mover got shorter, which rights the papers we set too
   long and worsens six we already set too short, two of them from exact. That surplus
   glue was cancelling a second, opposite error, now measurable on its own
   ([#683](https://github.com/go-tex/engine/issues/683)). `\predisplaysize` follows
   `tex.web:21781-21813`, two ems and all, but is not exposed as a register.
   **Every page is packed `exactly` to `\vsize`, whatever its page style**
   (`tex.web` §19321 `page_goal:=vsize`, §19564 `best_size:=page_goal`, §19813
   `vpackage(…,best_size,exactly,…)`): TeX has no page-style branch in the packer,
   because styles are the output routine's business. `\pagestyle{empty}` — which this
   engine takes as its **default** — was packed at its **natural** height, so the page
   was exactly as tall as its material and every vertical glue on it stretched by
   nothing: `\vspace{\fill}` under that style put its word at **143.91** where
   `tectonic` puts it at **664.72**. §19811-19814 sets `vbadness:=inf_bad` and
   `vfuzz:=max_dimen` around that call *"to inhibit error messages"*, so a page that
   cannot fill itself is packed short in **silence** — it is not a warning.
   **A running head was four mechanisms, and three correct fixes left it empty.**
   The band and the folio were right ([#726](https://github.com/go-tex/engine/pull/726)),
   the page-style dispatch was right, the mark pair moved to `\@themark` — the name
   the kernel and every class use ([#728](https://github.com/go-tex/engine/pull/728)) —
   and the head stayed blank, because `amssubstrate.go` carried `\def\mark#1{}`: a
   stub that **discarded its argument**, one layer below all three. A stub like that
   is the one shape no channel sees — the document asks, the engine says nothing, the
   suite stays green, and the loss exists only as ink that is not on the page. It
   cost 27 of the 128 corpus papers with a reference their head. What found it was a
   test asking not *is this layer right* but **does the node arrive**
   ([#729](https://github.com/go-tex/engine/pull/729)). Marks now travel as nodes on
   the vertical list, their argument expanded at mark time (`scan_toks(false,true)`,
   so amsart's `\mark{\the\@temptokena}` stores the pair and not the three tokens
   that would fetch it), and `\topmark`/`\firstmark`/`\botmark` follow §1012's
   arithmetic (`:19718-19729`, `:19782-19789`) — including the two consequences a
   plainer reading misses and a head depends on: `\botmark` is **not** cleared on a
   page with no marks, so a head set once at `\maketitle` stays; and `\firstmark`
   falls back to `\topmark`, so a page broken mid-section reports the mark in force
   at its own top. Measured: 6 corpus papers gain ink at **identical page counts**,
   invisible to Σ and visible only to `hashdiff`, and Σ/bias/exact do not move.
   A `\mark` in a class **preamble** is read and dropped: `ws-fnl.cls` carries one,
   and contributing it to a page that does not exist yet turned 9 pages into 10 with
   the ink identical. `\\` is a **Go primitive** that scans its own
   star and optional length: as a macro built on `\@ifstar` it put `\@ifstar` into the
   maths source and every formula holding a matrix row separator was refused, because
   the maths capture writes a control sequence **verbatim** and never consults its
   meaning — measured four ways, including a primitive that calls `fail` and is never
   run during a capture. `\\[20pt]` used to typeset the five characters `[20pt]` and
   `\\*` a `*`, on 38 and 9 of the 154 measured papers; the length now rides on the
   forced penalty and the line loop emits it after the line, giving the reference's
   31.88pt step where a bare `\\` gives 11.96 (#662). **`\vadjust` is implemented**
   (`tex.web` §2994-3001, §12900-12903, §17413-17418, §21195, §21199-21231): vertical
   material written inside a paragraph rides along in an adjust node that contributes
   no width, and comes out into the enclosing vertical list **after** the line it was
   written on — measured against `tectonic` as a 41.84pt baseline step where a normal
   one is 11.96, with the control paragraph unchanged. It is an **error** in vertical
   mode, as §21195 requires. Three commands were waiting on it and now work:
   `\pagebreak`/`\nopagebreak` in horizontal mode (the reference makes two pages on a
   short first paragraph and so do we, where we made one), `\\*`'s page-break
   suppression (the `\nobreak` goes *inside* the `\vadjust`, so it reaches the vertical
   list where 10000 refuses a page rather than a line), and `\\[len]`'s space, which
   now uses the same mechanism instead of the stopgap field it shipped with (#665). `\pagebreak`, `\nopagebreak`, `\linebreak`
   and `\nolinebreak` are **penalties** rather than no-ops (`ltspace.dtx` 294-306,
   344-375, 527-531): the optional degree goes through `\@getpen` (0, 51, 151, 301,
   10000) and the sign is what separates forcing from forbidding. All four did nothing
   at all before, on 18 of the 154 measured corpus papers; `\pagebreak` between two
   paragraphs made one page where `tectonic` makes two. Two gaps are left and named:
   in **horizontal** mode `\pagebreak` needs `\vadjust`, which is not implemented
   (#665), and a line ended by a forced break is **not justified** here — `\parfillskip`
   is appended to every forced-break fragment instead of once per paragraph, so the
   slack it should give the interword glue is absorbed (#666).
   **Hyphenation is Liang's algorithm on the embedded US-English patterns**
   (`go-typeset/hyphenation`), with `\lefthyphenmin`/`\righthyphenmin`,
   `\hyphenchar` — a face that files it negative is never broken, which is how
   `\texttt` keeps code whole (`tex.web` §896) — and the document's own
   `\hyphenation{…}` exception list. Two defects there were invisible to every
   test:
   **an accented word broke at an offset**, because the library built its value
   array over the word by BYTE index and read it back by RUNE index — the two
   agree for exactly as long as every letter is one byte, which is every
   English word and no French one. `élémentaires` broke as `éléme-ntaires`,
   between the m and the n of a syllable, where no pattern can put a break;
   with the fix it is whole and `présentation` breaks `présenta-tion`, which is
   where tectonic breaks it
   ([go-typeset/hyphenation#8](https://github.com/go-typeset/hyphenation/pull/8),
   [#774](https://github.com/go-tex/engine/pull/774)).
   **And `\hyphenation{…}` was a stub that discarded its argument** — a document
   writes it for the words the patterns get *wrong*, so the silence fell exactly
   on the words it was asked about. An entry now replaces the patterns for its
   word (`tex.web` §937), including an entry with no hyphen at all, which says
   the word may never be broken
   ([#775](https://github.com/go-tex/engine/pull/775)).
   ⚠ **Only English patterns are embedded.** A paper in another language is set
   unhyphenated where a real TeX hyphenates it; that gap is tracked in
   [#754](https://github.com/go-tex/engine/issues/754).
3. ✅ **Math** — `$…$` and the display environments delegated to `go-tex/math`
   (vector output).
4. ✅ **Fonts** — OpenType via `go-opentype`; a built-in font so it runs with no
   assets, with kerning and ligatures. A **family** when one is given: roman,
   bold, italic, slanted (NFSS's own substitution onto the italic), typewriter
   and sans are each bound to a file, and a glyph is drawn with the face it was
   measured with — `\textbf`, `\texttt` and `\verb` reach the page in their own
   face, not only in their own widths.
   **The size ladder is the class's table, to the point.** `\tiny` through
   `\Huge` are ten sizes a `size1x.clo` states, and three of them are not whole
   — `\Large` is `14.4`, `\LARGE` `17.28`, `\huge` `20.74`. A face held its size
   as an integer, so `\Large` landed on 14: 2.8 % narrow on every section
   heading in every document. Reading each line's `Tf` out of the PDF, all ten
   now agree with `tectonic` (`\Large` 13.95 → **14.35**, `\huge` 21 →
   **20.66**, `\Huge` 25 → **24.79**), and an `[11pt]` class sets its body at
   the `10.95` its `\@xipt` states rather than at 11
   ([#718](https://github.com/go-tex/engine/pull/718),
   [#719](https://github.com/go-tex/engine/pull/719)).
   **Kerning comes from every lookup, not the first.** OpenType applies GPOS
   lookups in LookupList order and their adjustments add; stopping at the first
   one that *covers* a pair returned a deliberate zero and never reached the
   real value. Libertinus Serif files `A`/`V` with `0` in one kern lookup and
   `−112` in a later one, so `\hbox{AV}` at 10pt measured `13.47pt` against
   XeTeX's `12.35pt`, and now measures `12.35pt`
   ([go-opentype/opentype#63](https://github.com/go-opentype/opentype/pull/63)).
   Neither of these shows in the page-count barometer below: a kern, or a
   heading a third of a point wider, costs a page only when it tips one. The
   witness is the oracle comparison.
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

## The instruments

Fidelity work here is measured, not asserted, and each of these answers one
question the others cannot. They are ordinary `go run ./cmd/…` programs.

| | |
|---|---|
| `gotex-abdiff` | compiles a corpus of real papers at two revisions and reports Σ\|page deviation\| against the reference counts, naming every paper that moved. It builds each side in its own `git worktree`, checks that the head really descends from the base, and prints whether `GOTEX_PDFRENDER` was set — three measurements had been read the wrong way before it did. |
| `gotex-refdiff` | compiles a sample of papers with `tectonic` and with the engine and scores each one: word recall (`what` text came out) or, with `-layout`, word displacement (`where` it landed). `-gotex` names the binary to measure, so two revisions can be put side by side over the **same** sample; `-list` draws that sample from a named corpus list instead of walking a raw arXiv directory, where the papers the REFERENCE cannot compile score "ref-unavailable" — a 30-paper run came back 30 of 30 unavailable on both sides of a change, which reads exactly like "no difference". A compile that merely exhausts `-timeout` is `timed-out` and leaves the score on **either** side, with a banner on stderr: it used to be scored 0 on ours and merely dropped on the reference's, so a slow paper — or a loaded machine — moved the mean one way only. |
| `gotex-linediff` | puts one page of two PDFs side by side **line by line** — where each line starts and ends, its baseline, face and size, and, when it holds a dot leader, how many tiles it draws and how far apart. This is the rung below a per-paper score: the sentence a defect is fixed from reads "our entry starts 1.58pt right of the reference's and its dots are twice as dense". It reads the PDFs with `go-pdfkit/reader` — no `qpdf`, no poppler, no MuPDF. |
| `gotex-worddiff` | which **words** one PDF holds that the other does not, as a multiset, for two builds of this engine and no reference. It answers the question a moved score leaves open — `gotex-refdiff` says a paper fell 0.1 points, this says which words that was. It reads both files through `internal/pdftext`, the extractor the score itself uses: a word-diff on a different extractor reports the two extractors' disagreement in the engine's units, which is how a fused `CenterforTheoretical` got published twice as an engine defect before the line dump showed the spaces were in both files. `-layout` keeps **columns** apart — without it a two-column page is read as one stream and the end of one column's line fuses onto the start of another's (`disFigure`, `participaentire`), and which boundaries fuse moves when the lines do. On #772's two papers, scoring with `-layout` extraction made one of two apparent regressions vanish outright and reduced the other to a single word. |
| `gotex-coverage` | the coverage floor CI enforces. |

A dot leader is the case that makes the last one worth having: `Title. . . . 7`
reads the same whether the tiles are 4.8pt apart or 8.5, so no text-level
comparison can see the difference. The drawn positions can.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-tex/engine authors.
