package engine

// \Collect@Body (environ.sty, and amsmath's \collect@body before it) gathers an
// environment's whole body and hands it to a macro as ONE argument, instead of
// letting it be typeset where it stands. A class uses it to capture front matter
// it will typeset later:
//
//	\renewenvironment{abstract}{\Collect@Body\@saveabstract}{}      (acmart.cls:1482)
//	\long\def\@saveabstract#1{… \long\gdef\@abstract{#1}}           (:1483-1486)
//	\def\@mkabstract{… \ignorespaces\@abstract\par}                 (:2453-2461)
//
// ⛔⛔ UNDEFINED, THIS DOES NOT LOSE THE ABSTRACT — IT TYPESETS PART OF IT IN THE
// WRONG PLACE AND EATS ONE TOKEN OF IT. An unknown command is skipped, and
// skipping RELEASES its argument (see \aftermaketitle@chk in kernelhelpers.go for
// the same mechanism costing a whole paper). So \Collect@Body vanished,
// \@saveabstract ran next and took as its single argument the very next token —
// not the body. Measured on 2304.01951, whose abstract is one \input:
//
//	\begin{abstract}\input{sections/00-abstract}\end{abstract}
//
// \@saveabstract swallowed \input, and "{sections/00-abstract}" was left in the
// stream as ORDINARY TEXT. Our page 1 carried two words — the file's name and a
// folio — against the reference's 853, and the abstract never appeared. With a
// body that starts with a letter the loss is subtler and worse: "AVANT …" came out
// "VANT …", the A gone and the rest set outside the box the class meant for it.
func (e *Engine) doCollectBody() {
	// The macro to call, kept as TOKENS and never as text.
	//
	// ⛔ Re-splicing it as source does not work, and it fails in a way that still
	// typesets: \@saveabstract written back into the document is re-read where @ is
	// catcode 12, so it tokenises as \@ followed by the letters "saveabstract", and
	// the page came out reading "saveabstractProcessing-in-memory promises to…".
	// The class read that name with @ a letter, inside \renewenvironment; the token
	// is already a control sequence and must stay one.
	mac := e.grabUndelimited()
	if len(mac) == 0 {
		return
	}
	// currentEnvir and not currentEnvName: the latter falls back to "figure" when
	// \@currenvir is unset, which for this caller would be a wrong guess and not a
	// neutral one. "document" is refused with an unset name, and that is not
	// belt-and-braces: called outside any environment, \@currenvir still names the
	// enclosing one, so the scan ran to \end{document} and ate the whole document.
	env, ok := e.currentEnvir()
	if !ok || env == "" || env == "document" {
		e.skipUndefined("Collect@Body")
		return
	}
	body, found := e.collectEnvBodyToks(env)
	if !found {
		// No \end{env} anywhere ahead: the body read so far is the rest of the
		// input, and handing THAT to the macro would delete the document. Put it
		// back and decline, so a malformed source degrades to the old behaviour
		// rather than to an empty file.
		e.push(body)
		e.skipUndefined("Collect@Body")
		return
	}
	// <macro>{body}, as tokens, read before anything else. environ then puts the
	// \end back; here collectEnvBodyToks has consumed it and left \begin's group to
	// be closed by endEnvGroup, which is the same end state.
	out := append([]tok{}, mac...)
	out = append(out, chTok('{', catBegin))
	out = append(out, body...)
	out = append(out, chTok('}', catEnd))
	e.push(out)
	e.endEnvGroup()
}

// collectEnvBodyToks reads tokens up to the \end{env} that closes this
// environment, and returns them with whether that \end was actually found.
//
// ⛔⛔ IT MUST READ TOKENS, NOT SOURCE TEXT, and the first version of this read
// source. readRawEnvBody scans the character buffer, which is right for a verbatim
// environment — whose body certainly follows in the buffer — and wrong here,
// because \Collect@Body can be reached with tokens still PENDING in the input
// lists. acmart itself does that:
//
//	\newenvironment{translatedabstract}[1]{\Collect@Body
//	  \@savetranslatedabstract\@mktranslatedabstract{#1}}{}
//
// Two control sequences and a braced argument follow the call. Reading the body
// from the buffer steps over those pending tokens and re-splicing into the buffer
// puts the collected body behind them, so the order of execution inverts.
// Measured: 2405.20319 came out 10 pages against a reference of 14 and lost 1875
// words — including the paper's TITLE, three figure captions, a table caption and
// the tail of its bibliography, which stopped mid-list at entry [36] — with no
// diagnostic at all. getNext reads the pending lists first and the buffer after,
// so collecting through it keeps the order the document has.
//
// Nesting is counted, so \begin{env}…\end{env} inside the body does not end it
// early. A \begin or \end whose name is not a plain brace group (\begin\foo) is
// passed through as ordinary tokens rather than guessed at.
func (e *Engine) collectEnvBodyToks(env string) ([]tok, bool) {
	var body []tok
	depth := 0
	for {
		t, ok := e.getNext()
		if !ok {
			return body, false
		}
		if !t.cs_ || (t.cs != "end" && t.cs != "begin") {
			body = append(body, t)
			continue
		}
		name, nameToks, ok := e.peekBraceName()
		if !ok {
			// Not \end{…}: keep both the control sequence and whatever was read
			// looking for a name, so nothing is silently dropped.
			body = append(body, t)
			body = append(body, nameToks...)
			continue
		}
		if t.cs == "begin" {
			if name == env {
				depth++
			}
			body = append(body, t)
			body = append(body, nameToks...)
			continue
		}
		if name != env {
			body = append(body, t)
			body = append(body, nameToks...)
			continue
		}
		if depth > 0 {
			depth--
			body = append(body, t)
			body = append(body, nameToks...)
			continue
		}
		return body, true
	}
}

// peekBraceName reads a {name} group at the token level and returns the name, the
// exact tokens it consumed (so a caller that decides it did not want them can put
// them back verbatim), and whether a brace group was there at all.
//
// The tokens are returned rather than reconstructed: rebuilding "{" + name + "}"
// would normalise catcodes and spaces, and this is used on a path where the body
// must come out exactly as it went in.
func (e *Engine) peekBraceName() (string, []tok, bool) {
	var seen []tok
	t, ok := e.getNext()
	if !ok {
		return "", seen, false
	}
	seen = append(seen, t)
	if t.cs_ || t.cat != catBegin {
		e.back(t)
		return "", nil, false
	}
	var name []rune
	for {
		u, ok := e.getNext()
		if !ok {
			return "", seen, false
		}
		seen = append(seen, u)
		if !u.cs_ && u.cat == catEnd {
			return string(name), seen, true
		}
		if u.cs_ {
			// \begin{\foo} is not an environment name this can resolve; hand the
			// tokens back to the caller rather than inventing one.
			return "", seen, false
		}
		name = append(name, u.ch)
	}
}
