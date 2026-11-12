// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// Where every embedded font came from, as DATA rather than as prose.
//
// ⛔ THE REPOSITORY CARRIED THE LICENCE AND NOT THE PROVENANCE. The files
// arrived in #744 and #745 described as "as tectonic's", which in practice
// meant copied out of a bundle cache on somebody's machine. What was recorded
// was a sentence in a Go comment — "the unmodified OpenType files of the Latin
// Modern 2.004 distribution" — and nothing that could be checked: no source,
// no checksum, no version as a value, and not the MANIFEST that the GUST Font
// License itself names as the document defining the Work.
//
// A font is not like other vendored bytes. It is redistributed with the
// program, so the claim "unmodified" is the whole licence argument, and a claim
// nobody can verify is not an argument.
//
// ⭐ VERIFIED, NOT ASSERTED. Every file below was fetched from CTAN and compared
// byte for byte with what this repository carries; all five matched. The
// checksums are what makes that verification repeatable, and
// TestEmbeddedFontsMatchTheirRecordedProvenance re-runs it on every build.
//
// # Two distributions, which the old comment ran together
//
// The four text faces come from CTAN `fonts/lm` (Latin Modern 2.004). The maths
// face does NOT: it is CTAN `fonts/lm-math` (Latin Modern Math 1.959), a
// separate package with its own manifest, its own README and its own authors.
// Saying "the Latin Modern 2.004 distribution" of all five was wrong about one
// of them.
//
// # The licence, and one thing that looks like a problem and is not
//
// GUST Font License — "actually an instance of the LaTeX Project Public
// License", in the upstream README's own words. It permits distribution and
// modification; its one extra clause is "requested, but not legally required":
// rename derived works. These are embedded unchanged, so it does not arise.
//
// ⚠ `lm-math` ships, in its doc directory, a PRELIMINARY (2006-09-30) text of
// that licence, where `lm` ships version 1.0 of 22 June 2009. The copy in
// fonts/lm is the 2009 one and is byte-identical to `lm`'s. That is correct for
// all five: every font's own copyright field points at the canonical URL
// http://tug.org/fonts/licenses/GUST-FONT-LICENSE.txt rather than at the text
// beside it, so the stale copy in lm-math's doc directory is their packaging
// and not a different licence that would apply to us.
type fontProvenance struct {
	file    string // the path under fonts/
	pkg     string // the CTAN package it comes from
	version string // the version the FONT ITSELF declares in its name table
	sha256  string // of the file as embedded
}

// ctanMirror is where the two packages were fetched from on 2026-11-12. It is a
// redirector, so the bytes behind it move between mirrors; the per-file
// checksums below are what pins the content, not this URL.
const ctanMirror = "https://mirrors.ctan.org/"

// embeddedFontProvenance records every font file this binary carries.
//
// A file added here without its checksum, or with the wrong one, fails the test
// in fontprovenance_test.go — which is the point: the next person to vendor a
// font has to say where it came from, and the claim is checked rather than
// believed.
var embeddedFontProvenance = []fontProvenance{
	{"lm/lmroman10-regular.otf", "fonts/lm", "2.004",
		"1aa18cfefa58132c52ce5de70db1fd1154201c19cd2b2cdaffba4906a33e6852"},
	{"lm/lmroman10-bold.otf", "fonts/lm", "2.004",
		"102fe06c430a8b681b2bf6876b7cd967ae4d47b4b6b41d915eb7913b726d9fb1"},
	{"lm/lmroman10-italic.otf", "fonts/lm", "2.004",
		"c1fce25075567bb8dbf2151658c3b442690041db17a2d49fc9e55905ea5b7169"},
	{"lm/lmmono10-regular.otf", "fonts/lm", "2.004",
		"22deb6d3be3ffcb40b33f0e010afb256c677d97c2a2bdee1f5740cdc97751558"},
	{"lm/latinmodern-math.otf", "fonts/lm-math", "1.959",
		"6075562b771f8b82f0c179e363389684f2dd09de30038269e2628e504bd7be0f"},

	// ── the optical sizes ────────────────────────────────────────────────────
	//
	// Latin Modern is an optically-sized family: lmroman9 is not lmroman10 made
	// smaller, it is a face drawn for 9pt, with relatively wider letters and a
	// wider space. tulmr.fd's \DeclareFontShape tables are the reference's own
	// size→file map, and the sizes below are exactly the files those tables name
	// (see lmoptical.go). Each version string is read out of the font's own name
	// table rather than off its filename.
	{"lm/lmroman5-regular.otf", "fonts/lm", "2.004",
		"954bd9c376597c7043e7b616430163e0158155edf7434fd70082cf8460c08533"},
	{"lm/lmroman6-regular.otf", "fonts/lm", "2.004",
		"501396c437f76c260b8724265b8495f318a640ee5a4674852ecbb20daf256696"},
	{"lm/lmroman7-regular.otf", "fonts/lm", "2.004",
		"48c4b46ab255eec96588fb992fba1ed6102e53d051da480738a61cc6cdb328c3"},
	{"lm/lmroman8-regular.otf", "fonts/lm", "2.004",
		"f73015c3827596586faa380c398501caf9b5bcbb6f40b46af6a51277ca97c9fc"},
	{"lm/lmroman9-regular.otf", "fonts/lm", "2.004",
		"8ab7b017fc67ea0e30cbf9e7ebd940010442ea67b40cf65f87d73f3ca11b0e6f"},
	{"lm/lmroman12-regular.otf", "fonts/lm", "2.004",
		"e6be218ae83e61aa8a29990d3cdc401c678c1962188cb9a4a8b6359e4f5e5870"},
	{"lm/lmroman17-regular.otf", "fonts/lm", "2.004",
		"ed048c80fb4b67663b80899b89fe5fba17d7442e1286d61fd08fe5689c6b7c3c"},

	{"lm/lmroman5-bold.otf", "fonts/lm", "2.004",
		"6326fb955d07bedcc8f73684e6b6bfa76706d38f3a85bd89c0659504ace366a1"},
	{"lm/lmroman6-bold.otf", "fonts/lm", "2.004",
		"f1ebd52916182bfc2e3f9d4b5ba847ad6acc6c7761354c0dc052fb01ec14f73c"},
	{"lm/lmroman7-bold.otf", "fonts/lm", "2.004",
		"063ea9e378b4fca4fd59771cf522faa21eb7c8516c89464745c79725b384fb9a"},
	{"lm/lmroman8-bold.otf", "fonts/lm", "2.004",
		"04d4f633c3da31a169f4c54a7fe8c7e63925b39d030381f8d3eab955f16be7b2"},
	{"lm/lmroman9-bold.otf", "fonts/lm", "2.004",
		"f72feac9ea12eb858d8093d6c0ee710a6c25f59b61278c88ec41c0a680838415"},
	{"lm/lmroman12-bold.otf", "fonts/lm", "2.004",
		"28c8782ac2b6486958b5dc7610ada7800c53546ff7f36bc65909a876e1cd338e"},

	{"lm/lmroman7-italic.otf", "fonts/lm", "2.004",
		"19d20f2bb00fbffe5d3d66e9b6a54f0108289068029e9d88649d981d53613ddf"},
	{"lm/lmroman8-italic.otf", "fonts/lm", "2.004",
		"fb0536b5c3eee5b5622cd827ceb410e6514e36512ebdc51d2cf98a96c06d44e8"},
	{"lm/lmroman9-italic.otf", "fonts/lm", "2.004",
		"2106cffcdc9f9cb411d7c3441e34d77a168b998a58b40e727ab5b815437d1975"},
	{"lm/lmroman12-italic.otf", "fonts/lm", "2.004",
		"ada6b3f451238784e5a5ea75d43efa1eb4329cd75ce65bf047ed2f31b7b5e2f3"},

	{"lm/lmmono8-regular.otf", "fonts/lm", "2.004",
		"17319490d20f24e192bd7e7999484b95111a4654ac2620e7aa215591142ae997"},
	{"lm/lmmono9-regular.otf", "fonts/lm", "2.004",
		"cdd53c7dc01a1d25f21eaf7cae44bfb8a9665e4cbd60e5f8da14287aca819bb9"},
	{"lm/lmmono12-regular.otf", "fonts/lm", "2.004",
		"b826e490fa671f254840abe88afea0975b606f57fb4a8b93f53787c26d8ac319"},
}
