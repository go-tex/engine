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
