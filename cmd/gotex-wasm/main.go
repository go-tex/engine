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
//	gotexToSVG(source [, {size, margin}])  → { pages: [svgString, …] }  | { error }
//	gotexVersion()                         → a short identifier string
//
// The built-in font is embedded, so it runs with no assets. \input and \font
// from disk are unavailable in the browser sandbox (no filesystem); a document's
// text is compiled with the built-in font unless it selects one another way.
package main

import (
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
	pages, err := engine.CompileToSVGPages([]byte(args[0].String()), opt)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	out := make([]any, len(pages))
	for i, p := range pages {
		out[i] = p
	}
	return map[string]any{"pages": out}
}

func version(js.Value, []js.Value) any { return "gotex-wasm (github.com/go-tex/engine)" }

func main() {
	js.Global().Set("gotexToSVG", js.FuncOf(toSVG))
	js.Global().Set("gotexVersion", js.FuncOf(version))
	select {} // keep the Go runtime alive so the exported functions stay callable
}
