// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.RGBA{1, 2, 3, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// graphicx searches for a figure written without its extension, pdf first. An
// \includegraphics{fig} must resolve to fig.pdf / fig.png / … on disk instead of
// failing to a placeholder — the dominant reason real papers' figures did not load
// even with a rasteriser wired.
func TestResolveGraphicsPath(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "fig.png"))

	if got, ok := resolveGraphicsPath(filepath.Join(dir, "fig")); !ok || filepath.Base(got) != "fig.png" {
		t.Fatalf("extension-less {fig} resolved to (%q,%v), want fig.png,true", got, ok)
	}
	if got, ok := resolveGraphicsPath(filepath.Join(dir, "fig.png")); !ok || filepath.Base(got) != "fig.png" {
		t.Fatalf("exact fig.png resolved to (%q,%v)", got, ok)
	}

	// pdf wins over png when both exist (pdftex \Gin@extensions order).
	writeTestPNG(t, filepath.Join(dir, "both.png"))
	writeTestPNG(t, filepath.Join(dir, "both.pdf")) // content irrelevant to resolution
	if got, ok := resolveGraphicsPath(filepath.Join(dir, "both")); !ok || filepath.Ext(got) != ".pdf" {
		t.Fatalf("{both} resolved to %q, want the .pdf first", got)
	}

	// An explicit extension that does not exist is honoured, not searched past.
	if _, ok := resolveGraphicsPath(filepath.Join(dir, "fig.eps")); ok {
		t.Fatal("explicit fig.eps must not fall back to fig.png")
	}
	// Nothing matches.
	if _, ok := resolveGraphicsPath(filepath.Join(dir, "nope")); ok {
		t.Fatal("a missing {nope} must not resolve")
	}
	// A directory must not count as a file.
	if err := os.Mkdir(filepath.Join(dir, "adir.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, ok := resolveGraphicsPath(filepath.Join(dir, "adir")); ok {
		t.Fatal("a directory named adir.png must not resolve {adir}")
	}
}

// End to end: an extension-less reference loads the PNG through loadImage.
func TestIncludeGraphicsExtensionSearch(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "fig.png"))

	data, format, iw, ih, _, _, err := loadImage(filepath.Join(dir, "fig"), false)
	if err != nil {
		t.Fatalf("extension-less loadImage: %v", err)
	}
	if format != imgPNG || len(data) == 0 || iw != 4 || ih != 3 {
		t.Fatalf("loaded (format=%v bytes=%d %dx%d), want PNG 4x3", format, len(data), iw, ih)
	}
}
