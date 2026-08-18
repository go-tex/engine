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

// hostTeXFile resolves one exact file name against the two sources that do not
// need a filesystem: the host's Options.Resolve first, then the base set embedded
// in the binary. Keeping the order in one place is what stops the two call sites
// (\usepackage/\documentclass in packages.go, \input in io.go) from drifting
// apart — a package found through the host whose own \input'ed parts were not
// would load only its first file.
func (e *Engine) hostTeXFile(name string) ([]byte, string, bool) {
	if e.resolve != nil {
		if data, ok := e.resolve(name); ok {
			return data, "<host>/" + name, true
		}
	}
	if data, ok := embeddedTeXFile(name); ok {
		return data, "<embedded>/" + name, true
	}
	return nil, "", false
}

// embeddedTeXFile returns a base class/package file shipped in the binary, if the
// embedded set has one by that exact name.
func embeddedTeXFile(name string) ([]byte, bool) {
	data, err := embeddedTeXMF.ReadFile("texmf/" + name)
	if err != nil {
		return nil, false
	}
	return data, true
}
