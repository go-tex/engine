# License note for the embedded LaTeX base files

The files in this directory are part of / derived from the **LaTeX base system**
and the **AMS document classes**, and are distributed under the

> **LaTeX Project Public License (LPPL), version 1.3c or later**
> <https://www.latex-project.org/lppl.txt>

They must be kept **verbatim** when embedded. Each file carries its own
`\ProvidesClass`/`\ProvidesFile` line with the upstream version and date, and the
full LPPL preamble at the top of the file (do not strip it).

| File | ProvidesClass/File | Version | Upstream source |
|------|--------------------|---------|-----------------|
| `article.cls` | `\ProvidesClass{article}` | `2026-06-04 v1.4n` | generated from `classes.dtx` (latex3/latex2e) with option `article` |
| `size10.clo`  | `\ProvidesFile{size10.clo}` | `2026-06-04 v1.4n` | generated from `classes.dtx` with option `10pt` |
| `size11.clo`  | `\ProvidesFile{size11.clo}` | `2026-06-04 v1.4n` | generated from `classes.dtx` with option `11pt` |
| `size12.clo`  | `\ProvidesFile{size12.clo}` | `2026-06-04 v1.4n` | generated from `classes.dtx` with option `12pt` |
| `amsart.cls`  | `\ProvidesClass{amsart}` | `2020/05/29 v2.20.6` | generated from `amsclass.dtx` (CTAN amscls) with options `amsart,classes` |
| `isomath.sty` | `\ProvidesPackage{isomath}` | `2012/09/04 v0.6.1` | CTAN `isomath`, © 2008, 2012 Günter Milde |

## Provenance / how these were produced

CTAN and the LaTeX base ship **only the sources** (`classes.dtx` + `classes.ins`,
`amsclass.dtx` + `amsclass.ins`); the `.cls`/`.clo` are normally produced at
install time by running the `.ins` through TeX (docstrip). No `.cls`/`.clo` are
served pre-generated from CTAN, and `tug.org`'s SVN checkout endpoint refused the
TLS handshake from this host.

They were therefore regenerated **from the authentic LPPL sources** with a small
faithful docstrip emulator (`../docstrip.py`) using the exact module option lists
from the upstream `.ins` files:

- `classes.ins`  → `\file{article.cls}{\from{classes.dtx}{article}}`, `{10pt}`, `{11pt}`, `{12pt}`
- `amsclass.ins` → `\file{amsart.cls}{\from{amsclass.dtx}{amsart,classes}}`

Sources fetched (verbatim, kept alongside in `..`):

- `classes.dtx`, `classes.ins` — `https://raw.githubusercontent.com/latex3/latex2e/develop/base/`
- `amsclass.dtx`, `amsclass.ins` — `https://mirrors.ctan.org/macros/latex/required/amscls/`
  (served via mirror `ctan.mines-albi.fr`)

The macro **bodies are byte-for-byte the upstream code** (docstrip only strips
`%`-documentation lines and evaluates the module guards); only the standard
docstrip file header/preamble is reproduced by the emulator.

## Support packages beamer loads (added 2026-08-19)

`beamer.cls` requires these, and its own TDS distribution does **not** contain
them. Measured over a 500-document sample of real talks, with `beamer.cls` on the
search path and nothing else:

| files present | pages typeset |
|---|---|
| beamer alone | 1 983 |
| + `etoolbox` + the `iftex` family | 3 313 |
| + `keyval` | **4 286** |

Every document still *compiled* in all three cases — the loss is silent. Without
these the engine ships a beamer that renders **less than half** the content, so
they are embedded rather than left to a download: they are small, they are
LPPL like everything else in this directory, and they must be there before the
first page is typeset.

| File | ProvidesPackage | Version | Upstream source |
|------|-----------------|---------|-----------------|
| `etoolbox.sty` | `etoolbox` | `2025/10/02 v2.5m` | CTAN `install/macros/latex/contrib/etoolbox.tds.zip`, `tex/latex/etoolbox/` |
| `keyval.sty` | `keyval` | `2026-05-17 v1.15` | TeX Live tlnet `archive/graphics.tar.xz`, `tex/latex/graphics/` |
| `iftex.sty` | `iftex` | `2024/12/12 v1.0g` | CTAN `install/macros/generic/iftex.tds.zip`, `tex/generic/iftex/` |
| `ifetex.sty` | `ifetex` | `2019/10/25 v1.3` | idem |
| `ifluatex.sty` | `ifluatex` | `2019/10/25 v1.5` | idem |
| `ifpdf.sty` | `ifpdf` | `2019/10/25 v3.4` | idem |
| `ifvtex.sty` | `ifvtex` | `2019/10/25 v1.7` | idem |
| `ifxetex.sty` | `ifxetex` | `2019/10/25 v0.7` | idem |

All eight are **verbatim** upstream files, LPPL 1.3c or later, with their own
preamble intact — do not strip it.

`keyval.sty` is generated from `keyval.dtx` by docstrip, so CTAN serves no
pre-built copy; it is taken from the TeX Live tlnet archive, which ships the
generated runtime file. `ifetex`/`ifvtex` are not loaded by any document in the
corpus, but the `iftex` package is embedded whole rather than cherry-picked.

## xkeyval, the option machinery (added 2026-09-26)

`acmart.cls:46` is `\RequirePackage{xkeyval}`, and so are the preambles of several
other classes in the corpus. Without it every `\DeclareOptionX`, `\define@boolkey`,
`\define@choicekey`, `\ExecuteOptionsX` and `\ProcessOptionsX` is undefined, and the
whole `\if@ACM@*` cascade downstream of them with it — 45 skipped commands on one
paper.

Measured over the 154-paper corpus with these four files embedded, against
`go-tex/engine` at `d5c1729`:

| | |
|---|---|
| Σ&#124;page error&#124; | 328 → 335 (**+7**) |
| glyphs | **+19336** |
| papers moved | 10 |

Σ gets WORSE while the documents get closer to their reference, and the word counts
say why. On `2311.07602` (reference 31 pages, 15463 words):

| | pages | words |
|---|---|---|
| reference | 31 | 15463 |
| without xkeyval | **31** | 12683 (−2780) |
| with xkeyval | 35 | 14306 (−1157) |

The engine was **exactly right on the page count while missing 2780 words** — exact
by cancellation. Embedding xkeyval recovers 1623 of them and the length error moves
from hidden to visible.

One paper over-produces and is not yet explained: `2304.01951` goes from 10356 words
(−574) to 11805 (**+875**) against a 10930-word reference, and from 11 pages to 17.
See go-tex/engine#306.

| File | Provides | Version | Upstream source |
|------|----------|---------|-----------------|
| `xkeyval.sty` | `\ProvidesPackage{xkeyval}` | `2020/11/20 v2.8` | the tectonic bundle this engine is measured against, `sha256:6ffe055852f8faf66c0acbe1a7fb27f87b869a90bad1204f3bf4d9683f597c7c` (a TeX Live-derived bundle) |
| `xkeyval.tex` | `\ProvidesFile{xkeyval.tex}` | `2014/12/03 v2.7a` | idem — `xkeyval.sty` loads it with `\input xkeyval` |
| `xkvutils.tex` | — | generated from `xkeyval.dtx`, shipped with v2.8 | idem — `xkeyval.tex:49` inputs it unconditionally |
| `keyval.tex` | — | generated from `keyval.dtx` | idem — `xkvutils.tex:67-70` inputs it when `\ver@keyval.sty` is undefined |

All four are **verbatim**, LPPL 1.3c or later, with their LPPL preamble intact — do
not strip it. `xkvutils.tex` and `keyval.tex` carry no version line of their own;
they are docstrip products of their parent `.dtx`.

`xkvtxhdr.tex` is deliberately NOT embedded: `xkeyval.tex:59-64` inputs it only under
`\ifx\ProvidesFile\@undefined`, which is the **plain TeX** branch. Under LaTeX the
`\else` runs and the file is never wanted.

Embedding these needs `//go:embed texmf/*.tex` as well as `*.sty` (see `texmf.go`),
and the kernel's `\@filelist` / `\@addtofilelist` / `\filename@parse`, which
`xkeyval.sty` walks at load time to find the document class.

## `isomath.sty`

Not part of the LaTeX base system: a third-party CTAN package, © 2008, 2012 Günter
Milde, under the **LPPL 1.3 or later** (its own header states the licence; the
preamble is intact and the file is verbatim — same SHA-256 as the distribution copy).

It is embedded because its absence costs **56 dropped equations** on one paper of the
arXiv reference corpus, measured with the dropped-equation census: `\matrixsym` 30,
`\vectorsym` 19, `\tensorsym` 7, all of them content the reader never sees and none
of it visible in the page count, which stays at 27 either way.

It requires `fixmath` and `kvoptions` (`isomath.sty:68`, `:76`) and **neither is
embedded**. That is fine and is asserted by a test: the loader skips a
`\RequirePackage` it cannot resolve and the `\providecommand` definitions below it
still run. If a missing `\RequirePackage` ever became fatal, this package would stop
working and those 56 equations would come back — silently, because a dropped equation
does not fail a build.

## `natbib.sty`, the author's name (added 2026-11-16)

A third-party CTAN package, © 1993-2010 Patrick W Daly, **LPPL** (its own notice
states version 1 of the licence "or any later version"); verbatim, preamble intact.

It is embedded because without it `\citet{k}` prints `[1]` where the reference
prints the author's **name**. Measured from the page text of the test beside this
change:

| | `\citet{k}` | `\citep{k}` |
|---|---|---|
| without the file | `[1]` | `[1]` |
| with it | `Nordbotten et al. (2021)` | `(Nordbotten et al., 2021)` |

Those are `tectonic`'s own forms. **20 of the 154** measured papers
`\usepackage{natbib}` and **none of them ships it**
([go-tex/engine#703](https://github.com/go-tex/engine/issues/703) counts 68 of 154
affected by the numeric fallback), and no document could reach a copy by any path:
measured before this change, `natbib` appeared nowhere in this directory and
nowhere in the bundle catalogue either (zero occurrences in
`github.com/go-tex/texmf@v0.7.0`). It is not in `neverLoadReal`, so a resolvable
copy is loaded and run as real TeX — which is the whole fix, since
[#712](https://github.com/go-tex/engine/pull/712) already measured that the
genuine file produces the reference's output word for word.

| File | Provides | Version | Upstream source |
|------|----------|---------|-----------------|
| `natbib.sty` | `\ProvidesPackage{natbib}` | `2010/09/13 8.31b (PWD, AO)` | TeX Live tlnet `archive/natbib.tar.xz`, `tex/latex/natbib/natbib.sty`, sha256 `d9b0f6ed58f5c260d2df9435b97da23ca326e3596db8ef0681bbff7a14343603`, 45456 bytes |

CTAN serves **no** pre-built copy (`macros/latex/contrib/natbib/natbib.sty` is a
404) because the `.sty` is a docstrip product — the same reason `keyval.sty` above
is taken from tlnet rather than CTAN. The CTAN source `natbib.dtx` (sha256
`6120e8b4574221bb22243ee6a720204841ecccfb0210bed82833e1db12e8734c`) declares the
same `2010/09/13 8.31b`, so the chain is consistent; docstrip was **not** re-run to
prove byte-equality and no such claim is made here.

⚠ **One condition new to this directory, reviewed and decided on 2026-11-16.**
`natbib.sty`'s notice adds: *"It may not be distributed without the original source
file `natbib.dtx`."* None of the fifteen files above carries that sentence. The two
readings and the decision:

- **literal** — ship `natbib.dtx` (249436 bytes) beside the `.sty`;
- **as practised** — ship the generated `.sty` alone, which is what TeX Live, the
  canonical distributor, does: its runtime package carries exactly this file with
  no `.dtx` beside it. This directory already embeds `keyval.sty` on that basis,
  for the same reason — the `.sty` is a docstrip product CTAN does not serve.

**TeX Live's practice is followed.** The condition is written down here rather than
resolved away, so a later reader can reverse the call without rediscovering it: the
`.dtx` can be added beside the `.sty` at any time, and its sha256 is recorded two
paragraphs above.

⛔ And three traps the test beside it records, because the first version of that
test **passed while both citations rendered `?`**: the observable is "et al.", not
the author's surname (which is also in the bibliography entry, needing no citation
machinery); it must read the **two-pass** `compile()`, since `\bibcite` reaches the
`.aux` only on the second pass; and the document needs a `\documentclass`, because
natbib's `\citet` calls the kernel's `\@citex`, which only a loaded class defines.
