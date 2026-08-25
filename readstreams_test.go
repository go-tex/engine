// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// withDataFile puts a file on the input path and returns an engine that can read
// it, so a test can exercise \openin against something real.
func withDataFile(t *testing.T, name, content string) *Engine {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTEX_TEXMF", dir)
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// The reason every package needs \openin and \ifeof is not reading data — it is
// asking whether a file EXISTS, and there is no other way to put that question
// to TeX:
//
//	\openin\r=fancyhdr.sty \ifeof\r <absent>\else <present>\fi \closein\r
//
// Measured against a real LaTeX (tectonic): a file that opens reads as not-ended,
// one that is missing reads as ended, and so does a stream after \closein.
func TestOpeninAnswersWhetherAFileExists(t *testing.T) {
	e := withDataFile(t, "donnees.txt", "ligne un\nligne deux\n")
	out, err := e.Run(`\documentclass{article}\newread\zr` +
		`\openin\zr=donnees.txt \message{[present=\ifeof\zr NON\else OUI\fi]}` +
		`\closein\zr \message{[apres-closein=\ifeof\zr FIN\else OUVERT\fi]}` +
		`\newread\zs\openin\zs=nexistepas.txt \message{[absent=\ifeof\zs OUI\else NON\fi]}` +
		`\message{[jamais-ouvert=\ifeof15 OUI\else NON\fi]}`)
	if err != nil {
		t.Fatal(err)
	}
	const want = `[present=OUI] [apres-closein=FIN] [absent=OUI] [jamais-ouvert=OUI]`
	if got := trimNL(out); got != want {
		t.Errorf("obtenu  %s\nattendu %s", got, want)
	}
}

// \read takes one line at a time and the stream ends when the lines run out.
func TestReadTakesOneLineAtATime(t *testing.T) {
	e := withDataFile(t, "donnees.txt", "premiere\nseconde\n")
	out, err := e.Run(`\documentclass{article}\newread\zr\openin\zr=donnees.txt ` +
		`\read\zr to \zl \message{[1=\zl]}` +
		`\read\zr to \zl \message{[2=\zl]}` +
		`\message{[reste=\ifeof\zr FIN\else ENCORE\fi]}` +
		`\closein\zr`)
	if err != nil {
		t.Fatal(err)
	}
	// The file ends with a newline, so an empty third line is still to come.
	const want = `[1=premiere] [2=seconde] [reste=ENCORE]`
	if got := trimNL(out); got != want {
		t.Errorf("obtenu  %s\nattendu %s", got, want)
	}
}

// TeX allows a space between "to" and the name, and a bare stream number in
// place of an allocated handle.
func TestReadAcceptsTheFormsTeXDoes(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"espace avant le nom", `\read\zr to \zl `},
		{"sans espace", `\read\zr to\zl `},
		{"numéro de flux nu", `\read0 to \zl `},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := withDataFile(t, "donnees.txt", "contenu\n")
			out, err := e.Run(`\documentclass{article}\newread\zr\openin\zr=donnees.txt ` +
				c.src + `\message{[\meaning\zl]}`)
			if err != nil {
				t.Fatal(err)
			}
			if got := trimNL(out); got != `[macro:->contenu]` {
				t.Errorf("%s : obtenu %s", c.src, got)
			}
		})
	}
}

// Reading past the end, or from a stream that was never opened, gives an empty
// macro rather than failing.
func TestReadPastTheEndIsEmpty(t *testing.T) {
	e := withDataFile(t, "donnees.txt", "seule\n")
	out, err := e.Run(`\documentclass{article}\newread\zr\openin\zr=donnees.txt ` +
		`\read\zr to \za \read\zr to \zb \read\zr to \zc ` +
		`\read9 to \zd \message{[a=\meaning\za][b=\meaning\zb][c=\meaning\zc][d=\meaning\zd]}`)
	if err != nil {
		t.Fatal(err)
	}
	const want = `[a=macro:->seule][b=macro:->][c=macro:->][d=macro:->]`
	if got := trimNL(out); got != want {
		t.Errorf("obtenu  %s\nattendu %s", got, want)
	}
}

// \newread allocates with \chardef, so the handle IS the stream number. As a
// count register it read as the register's VALUE — zero until something wrote
// it — and every stream a document opened collided on number 0.
func TestNewreadHandleIsTheStreamNumber(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\documentclass{article}\newread\za\newread\zb\newread\zc` +
		`\message{[\number\za][\number\zb][\number\zc][\meaning\za]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != `[0][1][2][\char"0]` {
		t.Errorf("obtenu %s, attendu [0][1][2][\\char\"0]", got)
	}
}

// Two streams open at once stay apart — the case the \chardef fixes.
func TestTwoStreamsAreIndependent(t *testing.T) {
	dir := t.TempDir()
	for n, c := range map[string]string{"un.txt": "AAA\n", "deux.txt": "BBB\n"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOTEX_TEXMF", dir)
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\documentclass{article}\newread\za\newread\zb` +
		`\openin\za=un.txt \openin\zb=deux.txt ` +
		`\read\za to \la \read\zb to \lb \message{[a=\la][b=\lb]}` +
		`\closein\za\closein\zb`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != `[a=AAA][b=BBB]` {
		t.Errorf("obtenu %s, attendu [a=AAA][b=BBB]", got)
	}
}

// The edges: a stream number outside the sixteen TeX keeps names no stream, so
// it reads as ended and yields nothing; \openin with no file name opens nothing;
// and \read with no name to read into consumes the line and stops there.
func TestReadStreamEdges(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"numéro hors des seize", `\message{[\ifeof99 FIN\else OUVERT\fi]}`, "[FIN]"},
		{"négatif", `\message{[\ifeof-1 FIN\else OUVERT\fi]}`, "[FIN]"},
		{"\\openin hors bornes", `\openin99=donnees.txt \message{[\ifeof99 FIN\else OUVERT\fi]}`, "[FIN]"},
		{"\\openin sans nom", `\newread\zr\openin\zr= \message{[\ifeof\zr FIN\else OUVERT\fi]}`, "[FIN]"},
		{"\\read sans nom de macro", `\newread\zr\openin\zr=donnees.txt \read\zr to {}\message{[ok]}`, "[ok]"},
		{"les espaces d'une ligne sont gardés", `\newread\zr\openin\zr=espaces.txt \read\zr to \zl \message{[\meaning\zl]}`, `[macro:->a b c]`},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for n, body := range map[string]string{"donnees.txt": "x\n", "espaces.txt": "a b c\n"} {
				if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("GOTEX_TEXMF", dir)
			e, err := buildEngine(Options{Lenient: true}, true)
			if err != nil {
				t.Fatal(err)
			}
			out, err := e.Run(`\documentclass{article}` + c.src)
			if err != nil {
				t.Fatal(err)
			}
			if got := trimNL(out); got != c.want {
				t.Errorf("%s : obtenu %s, attendu %s", c.src, got, c.want)
			}
		})
	}
}
