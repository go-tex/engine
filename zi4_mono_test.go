// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// zi4 binds \tt to Inconsolata when the tree has the file, and only \tt: the roman
// and sans faces are not touched by a package that changes the monospace face alone.
func TestZi4BindsTheMonospaceFace(t *testing.T) {
	for _, pkg := range []string{"zi4", "inconsolata"} {
		t.Run(pkg, func(t *testing.T) {
			e, err := buildEngine(Options{Resolve: func(name string) ([]byte, bool) {
				if name == "Inconsolatazi4-Regular.otf" {
					return lmSansRegular, true
				}
				return nil, false
			}}, true)
			if err != nil {
				t.Fatal(err)
			}
			rmBefore := faceKey(e.eq["rm"].font)
			ttBefore := faceKey(e.eq["tt"].font)
			if _, err := e.Run(`\usepackage{` + pkg + `}`); err != nil {
				t.Fatal(err)
			}
			tt := e.eq["tt"]
			if tt == nil || tt.kind != mFont || tt.font == nil {
				t.Fatalf("\\tt not bound to a face after \\usepackage{%s}: %+v", pkg, tt)
			}
			if faceKey(tt.font) == ttBefore {
				t.Error("\\tt is still the built-in monospace face")
			}
			if faceKey(e.eq["rm"].font) != rmBefore {
				t.Error("a monospace package changed the roman face")
			}
			if _, subst := e.fontSubst[pkg]; subst {
				t.Errorf("%s was reported as substituted although its face was found", pkg)
			}
		})
	}
}

// With no such file in the tree the substitution stays, and is reported.
func TestZi4WithoutTheFileIsReported(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	ttBefore := faceKey(e.eq["tt"].font)
	if _, err := e.Run(`\usepackage{zi4}`); err != nil {
		t.Fatal(err)
	}
	if faceKey(e.eq["tt"].font) != ttBefore {
		t.Error("\\tt changed although the tree has no Inconsolata")
	}
	if e.fontSubst["zi4"] != "Inconsolata" {
		t.Errorf("fontSubst[zi4] = %q, want it reported", e.fontSubst["zi4"])
	}
}

// A file that is not a font is a missing face, not a panic and not a binding.
func TestZi4WithUnparsableFile(t *testing.T) {
	e, err := buildEngine(Options{Resolve: func(name string) ([]byte, bool) {
		return []byte("not a font"), name == "Inconsolatazi4-Regular.otf"
	}}, true)
	if err != nil {
		t.Fatal(err)
	}
	ttBefore := faceKey(e.eq["tt"].font)
	if _, err := e.Run(`\usepackage{zi4}`); err != nil {
		t.Fatal(err)
	}
	if faceKey(e.eq["tt"].font) != ttBefore {
		t.Error("\\tt was bound to an unparsable file")
	}
}
