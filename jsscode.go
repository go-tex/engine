// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the Code environments of jss.cls (Journal of Statistical
// Software) — the class behind the Sweave/knitr R papers on arXiv. jss defines
// them as fancyvrb verbatim environments:
//
//	\DefineVerbatimEnvironment{Code}{Verbatim}{}
//	\DefineVerbatimEnvironment{CodeInput}{Verbatim}{fontshape=sl}
//	\DefineVerbatimEnvironment{CodeOutput}{Verbatim}{}
//	\newenvironment{CodeChunk}{}{}
//
// so Code/CodeInput/CodeOutput are verbatim code blocks and CodeChunk is a
// transparent wrapper grouping an input/output pair. Without handlers they were
// undefined environments: the code still reached the page but with category codes
// active and line breaks gone, so a multi-line R session collapsed into a single
// run of prose ("R> x <- c(1, 2, 3) R> mean(x) [1] 2"). They now reuse the listings
// verbatim machinery, setting each line verbatim in the tt font (the fontshape=sl
// slant on CodeInput is not reproduced — the content and its line structure are
// what matter). 29 of the corpus papers use them.

// doNamedVerbatimEnv typesets \begin{name} … \end{name} as a verbatim code block,
// where name is a fancyvrb-style environment (Code/CodeInput/CodeOutput). An
// optional [options] head is accepted and ignored; the body is read raw to
// \end{name} and set line-by-line in the tt font.
func (e *Engine) doNamedVerbatimEnv(name string) {
	e.scanRawOptBracket() // fancyvrb [options], accepted and ignored
	content, line := e.readRawEnvBody(`\end{` + name + `}`)
	e.renderVerbatimBlock(content, line, lstOptions{})
}
