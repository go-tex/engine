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
