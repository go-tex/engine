// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "embed"

// This file embeds a small set of real LaTeX base files (the article class and its
// size option files) so \documentclass/\usepackage can resolve them with no TeX
// distribution present. The embedded files are LPPL-licensed and kept verbatim; see
// texmf/LICENSE-NOTE.md for provenance. A file in the document's own directory or
// on the TEXINPUTS/GOTEX_TEXMF path takes precedence (see findTeXFile).

//go:embed texmf/*.cls texmf/*.clo texmf/*.def
var embeddedTeXMF embed.FS

// embeddedTeXFile returns a base class/package file shipped in the binary, if the
// embedded set has one by that exact name.
func embeddedTeXFile(name string) ([]byte, bool) {
	data, err := embeddedTeXMF.ReadFile("texmf/" + name)
	if err != nil {
		return nil, false
	}
	return data, true
}
