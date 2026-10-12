package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ \Collect@Body (environ.sty) hands an environment's WHOLE body to a macro.
// Undefined, it did not merely lose the body — an unknown command is skipped and
// skipping RELEASES its argument, so \@saveabstract ran next and took as its single
// argument the very next TOKEN.
//
// acmart.cls:1482 is \renewenvironment{abstract}{\Collect@Body\@saveabstract}{}, and
// 2304.01951's abstract is one \input: \@saveabstract swallowed \input and
// "{sections/00-abstract}" was left in the stream as ordinary text. Our page 1 carried
// two words — a file name and a folio — against the reference's 853.
func TestCollectBodyHandsTheWholeBodyToTheMacro(t *testing.T) {
	out, _ := runLaTeX(t, `\makeatletter`+
		`\long\def\keep#1{\long\gdef\kept{#1}}`+
		`\newenvironment{grab}{\Collect@Body\keep}{}`+
		`\begin{grab}ALPHA BETA GAMMA\end{grab}`+
		`\message{KEPT:\kept:}\makeatother`)
	if !strings.Contains(out, "KEPT:ALPHA BETA GAMMA:") {
		t.Errorf("le corps entier doit arriver à la macro, reçu: %q", out)
	}
}

// ⛔ The first token of the body must survive. With \Collect@Body skipped, a body
// beginning with a letter lost exactly that letter — "AVANT …" came out "VANT …" — and
// the rest was typeset outside the box the class meant for it. A test that only checked
// the body was "present" would have passed on that.
func TestCollectBodyDoesNotEatTheFirstToken(t *testing.T) {
	out, _ := runLaTeX(t, `\makeatletter`+
		`\long\def\keep#1{\long\gdef\kept{#1}}`+
		`\newenvironment{grab}{\Collect@Body\keep}{}`+
		`\begin{grab}AVANT\end{grab}`+
		`\message{KEPT:\kept:}\makeatother`)
	if strings.Contains(out, "KEPT:VANT:") {
		t.Error("le premier token du corps a été mangé")
	}
	if !strings.Contains(out, "KEPT:AVANT:") {
		t.Errorf("reçu: %q", out)
	}
}

// ⛔⛔ The macro is pushed as TOKENS, never re-spliced as source. Written back as text,
// \@saveabstract is re-read where @ is catcode 12, so it tokenises as \@ plus the
// letters "saveabstract" — and the page came out reading "saveabstractProcessing-in-
// memory promises to…". The class read that name with @ a letter; the token is already
// a control sequence and must stay one.
//
// The witness is an @-name, because that is the case that failed: a name of plain
// letters would survive the round trip and prove nothing.
func TestCollectBodyKeepsAnAtNameAsAControlSequence(t *testing.T) {
	out, _ := runLaTeX(t, `\makeatletter`+
		`\long\def\@keepit#1{\long\gdef\kept{#1}\message{MACRO-RAN}}`+
		`\newenvironment{grab}{\Collect@Body\@keepit}{}`+
		`\makeatother`+
		`\begin{grab}CORPS\end{grab}`+
		`\makeatletter\message{KEPT:\kept:}\makeatother`)
	if !strings.Contains(out, "MACRO-RAN") {
		t.Errorf("la macro @ n'a pas été exécutée (nom recomposé en texte ?): %q", out)
	}
	if !strings.Contains(out, "KEPT:CORPS:") {
		t.Errorf("reçu: %q", out)
	}
}

// A body holding a macro must reach the collector as that macro, not as its result:
// the whole point of collecting is that the class decides WHEN it is typeset.
func TestCollectBodyKeepsAMacroInTheBody(t *testing.T) {
	out, _ := runLaTeX(t, `\makeatletter`+
		`\long\def\keep#1{\long\gdef\kept{#1}}`+
		`\def\inner{INNER-RAN}`+
		`\newenvironment{grab}{\Collect@Body\keep}{}`+
		`\begin{grab}X\inner Y\end{grab}`+
		`\message{KEPT:\kept:}\makeatother`)
	if !strings.Contains(out, "KEPT:XINNER-RANY:") && !strings.Contains(out, "KEPT:X INNER-RAN Y:") {
		t.Errorf("le corps doit contenir la macro et se développer à l'usage: %q", out)
	}
}

// ⛔ Called OUTSIDE any \begin{…} there is no \end to read up to, and scanning for one
// would swallow the rest of the document. It must decline — currentEnvir's second
// result exists for exactly this, since currentEnvName would guess "figure".
func TestCollectBodyOutsideAnEnvironmentDoesNotSwallowTheDocument(t *testing.T) {
	// ⛔ The witness is a \message AFTER the call, not typeset text: runLaTeX returns
	// the diagnostic stream, so a test looking for ordinary words in it fails
	// whatever the engine does. The first version of this test did exactly that and
	// reported a swallowed document that was not swallowed.
	out, _ := runLaTeX(t, `\makeatletter\long\def\keep#1{}\Collect@Body\keep\makeatother `+
		`\message{REACHED-THE-END}`)
	if !strings.Contains(out, "REACHED-THE-END") {
		t.Errorf("ce qui suit l'appel a été avalé: %q", out)
	}
}

// ⛔⛔ And the case the guard above is actually FOR: inside \begin{document},
// \@currenvir names "document", so a scan for the matching \end runs to the end of
// the document and eats it. The previous test does not reach that branch — it is
// refused earlier because \@currenvir is unset entirely — and mutate confirmed as
// much by surviving the removal of `env == "document"`. A guard no test can reach is
// an assertion, not a guard.
func TestCollectBodyRefusesToCaptureTheWholeDocument(t *testing.T) {
	out, _ := runLaTeX(t, `\makeatletter\long\def\keep#1{\message{CAPTURED}}\makeatother`+
		`\begin{document}`+
		`\makeatletter\Collect@Body\keep\makeatother `+
		`\message{REACHED-THE-END}`+
		`\end{document}`)
	if !strings.Contains(out, "REACHED-THE-END") {
		t.Errorf("le document entier a été capturé: %q", out)
	}
	if strings.Contains(out, "CAPTURED") {
		t.Errorf("la macro a reçu le document comme corps: %q", out)
	}
}

// ⛔⛔ The case that distinguishes a TOKEN-level collector from a source-level one,
// and the reason this is implemented the way it is. acmart does exactly this:
//
//	\newenvironment{translatedabstract}[1]{\Collect@Body
//	  \@savetranslatedabstract\@mktranslatedabstract{#1}}{}
//
// Two control sequences and a braced argument follow the call, so when
// \Collect@Body runs there are TOKENS STILL PENDING in the input lists ahead of
// the body.
//
// ⛔ What environ does with them is NOT what I assumed. The first version of this
// test demanded that the pending tokens RUN and stay out of the body, and it
// failed. Asked of the reference, with environ.sty itself:
//
//	\newenvironment{grab}[1]{\Collect@Body\keep\after{#1}}{}
//	\begin{grab}{ARG}CORPS\end{grab}
//	→ KEPT:\long macro:->\after {ARG}CORPS:
//
// The pending tokens are collected INTO the body, at its head, and \after never
// runs. That is deliberate, and acmart relies on it: \@mktranslatedabstract goes
// into the STORED abstract so the language travels with it, where a collector that
// executed it instead would run a "mk" macro before \maketitle. The code was
// right and the test was wrong — asking the reference settled it in one run.
//
// So the body is the pending tokens FOLLOWED by the source body, which is what
// reading through getNext gives: it reads the pending lists before the character
// buffer. A collector that scans the character buffer skips them entirely. The
// 154-paper corpus does not exercise this — both implementations measure
// identically there, down to the ink — so this test is what justifies the
// token-level version rather than the simpler one.
func TestCollectBodyCollectsTokensPendingAfterTheCall(t *testing.T) {
	out, _ := runLaTeX(t, `\makeatletter`+
		`\long\def\keep#1{\long\gdef\kept{#1}\message{KEEP-RAN}}`+
		`\def\after#1{\message{AFTER-RAN:#1:}}`+
		`\newenvironment{grab}[1]{\Collect@Body\keep\after{#1}}{}`+
		`\begin{grab}{ARG}CORPS\end{grab}`+
		`\message{KEPT:\meaning\kept:}\makeatother`)
	if !strings.Contains(out, `\after {ARG}CORPS`) && !strings.Contains(out, `\after{ARG}CORPS`) {
		t.Errorf("le corps doit être les tokens pendants PUIS la source, comme environ: %q", out)
	}
	if strings.Contains(out, "AFTER-RAN") {
		t.Errorf("\\after ne doit PAS s'exécuter: il est capturé dans le corps: %q", out)
	}
	if !strings.Contains(out, "KEEP-RAN") {
		t.Errorf("le collecteur doit avoir appelé la macro: %q", out)
	}
}
