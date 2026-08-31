// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the overpic package's overpic environment:
//
//	\begin{overpic}[options]{imagefile} … \put(x,y){overlay} … \end{overpic}
//
// overpic sets an \includegraphics as the background of a picture and lets the
// document \put annotations (labels, arrows) on top of it in the image's own
// coordinate frame. Without a handler the name is an undefined environment, and
// in lenient mode its head and body TYPESET AS GARBAGE — the option list, the
// file name, and every \put coordinate and label leak into the running text
// (`[width=5cm]img.png(10,10)LABEL`), which is worse than dropping them. 164 of
// the corpus papers use it.
//
// overpic is a PICTURE environment the engine has no drawing layer for — exactly
// like tikzpicture/pgfpicture/tikzcd — so it takes the same treatment: gobble the
// whole environment (its options, the image name, and every \put overlay) and
// reserve a modest framed placeholder where the figure sat (emitPicturePlaceholder
// via gobbleEnvBody's placeholder path). This keeps the surrounding text flowing
// and pagination stable while removing the garbage.
//
// It deliberately does NOT reconstruct the background \includegraphics: its images
// are near-universally width=\textwidth PDFs, and with no PDF rasteriser wired each
// would frame a full-\textwidth SQUARE placeholder — a near-full page per figure,
// 12 of them on one corpus paper — which over-paginates far worse than the fixed
// picture placeholder. Treating overpic as the picture environment it is stays
// consistent with the rest of the engine and avoids that.
func (e *Engine) doOverpic(name string) {
	// overpic is only ever an environment; the guard mirrors doSubfigure so a stray
	// command spelling never sends the gobble scan hunting for an \end that is not
	// there (which would swallow the rest of the document).
	if !e.inEnvironment(name) {
		return
	}
	e.gobbleEnvBody(name, true) // consume opts+image+overlays, frame a picture placeholder
}
