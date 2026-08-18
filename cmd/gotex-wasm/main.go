// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

//go:build js && wasm

// Command gotex-wasm compiles the engine to GOOS=js/wasm and exposes LaTeX
// compilation to JavaScript, so an editor like loom can render LaTeX to SVG
// *directly in the browser* — no server round-trip, no microVM, no TeX Live.
// This is the capability TeX Live can never have: client-side live preview.
//
// It registers two globals:
//
//	gotexToSVG(source [, {size, margin}])
//	    → { pages: [svgString, …] }
//	    | { error, errorLine?, errorCol?, errorMessage? }
//	gotexVersion()                         → a short identifier string
//
// On failure `error` is always a human string; when the failure has a source
// location it also carries errorLine (1-based), errorCol (0-based) and
// errorMessage so the editor can point at the exact spot. Each rendered page's
// SVG groups glyphs under <g data-l="N"> (N = source line) for click-to-source
// and jump-to-line navigation.
//
// The built-in font is embedded, so it runs with no assets. \input and \font
// from disk are unavailable in the browser sandbox (no filesystem); a document's
// text is compiled with the built-in font unless it selects one another way.
package main

import (
	"errors"
	"syscall/js"

	engine "github.com/go-tex/engine"
)

func optionsFrom(v js.Value) engine.Options {
	opt := engine.Options{}
	if v.Type() != js.TypeObject {
		return opt
	}
	if s := v.Get("size"); s.Type() == js.TypeNumber {
		opt.Size = s.Int()
	}
	if m := v.Get("margin"); m.Type() == js.TypeNumber {
		opt.Margin = m.Float()
	}
	return opt
}

// toSVG compiles source to per-page SVG strings, returning {pages:[…]} or {error}.
func toSVG(_ js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		return map[string]any{"error": "gotexToSVG: a source string is required"}
	}
	var opt engine.Options
	if len(args) > 1 {
		opt = optionsFrom(args[1])
	}
	pages, diag, err := engine.CompileToSVGPagesDiag([]byte(args[0].String()), opt)
	if err != nil {
		// Keep `error` a string for existing callers; add the structured source
		// location (1-based line, 0-based column) when the failure carries one so
		// the editor can point at the exact spot.
		res := map[string]any{"error": err.Error()}
		var se engine.SourceError
		if errors.As(err, &se) && se.Line > 0 {
			res["errorLine"] = se.Line
			res["errorCol"] = se.Col
			res["errorMessage"] = se.Msg
		}
		return res
	}
	out := make([]any, len(pages))
	for i, p := range pages {
		out[i] = p
	}
	return map[string]any{"pages": out, "diagnostics": diagnosticsJS(diag)}
}

// diagnosticsJS turns the compile Diagnostics into a plain JS object for the
// playground's log panel: the undefined commands it skipped, the math equations the
// math layer dropped (invisible loss inside a formula), and the silent-swallow flags
// (a runaway that tripped, groups left open, a page-count explosion).
func diagnosticsJS(d engine.Diagnostics) map[string]any {
	skipped := make(map[string]any, len(d.Skipped))
	for name, count := range d.Skipped {
		skipped[name] = count
	}
	mathDropped := make(map[string]any, len(d.MathDropped))
	for name, count := range d.MathDropped {
		mathDropped[name] = count
	}
	return map[string]any{
		"skipped":     skipped,
		"mathDropped": mathDropped,
		"runaway":     d.Runaway,
		"openGroups":  d.OpenGroups,
		"pageCapHit":  d.PageCapHit,
	}
}

func version(js.Value, []js.Value) any { return "gotex-wasm (github.com/go-tex/engine)" }

func main() {
	js.Global().Set("gotexToSVG", js.FuncOf(toSVG))
	js.Global().Set("gotexVersion", js.FuncOf(version))
	select {} // keep the Go runtime alive so the exported functions stay callable
}
