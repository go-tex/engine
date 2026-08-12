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
