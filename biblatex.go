// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"strings"
)

// biblatex writes a .bbl of a different shape from BibTeX's. BibTeX emits a ready
// \begin{thebibliography} block that \bibliography simply \inputs; biber emits a
// DATA file — \entry{key}{type}{} … \endentry with \name, \field and \list — and
// leaves the formatting to the package's styles at run time.
//
// So \printbibliography cannot be served by splicing, and undefined it took the
// whole reference list with it. Seven of the 154 corpus papers use it, each losing
// every entry: 2408.08832 renders 90.4% of its reference's words, and the missing
// tenth is its bibliography.
//
// What this reads is the subset the entries actually carry — the names, the title,
// the container (journal/booktitle), the year, the volume, the pages — mapped onto
// the SAME bibEntry the .bib parser produces, so the existing formatter and the
// existing \bibitem machinery do the rest. Styles (author-year labels, sorting
// templates, \textcite) are NOT emulated: this is the numbered list every other
// bibliography in the engine is, which is what the plain styles give anyway.

var (
	blEntryRE = regexp.MustCompile(`\\entry\{([^}]*)\}\{([^}]*)\}`)
	blFieldRE = regexp.MustCompile(`\\(?:field|strng|verb)\{([a-zA-Z]+)\}\{`)
	blNamePRE = regexp.MustCompile(`\b(family|given)=\{`)
	blListRE  = regexp.MustCompile(`\\list\{([a-zA-Z]+)\}\{[0-9]+\}\{`)
)

// isBiblatexBBL reports whether a .bbl is biber's data format rather than
// BibTeX's ready-made thebibliography block. biber writes the marker itself.
func isBiblatexBBL(src string) bool {
	return strings.Contains(src, "biblatex auxiliary file")
}

// braced returns the text of the group that STARTS at src[i] == '{', and the index
// just past its closing brace. Nesting is tracked, since a field value carries
// braces of its own ({\"a}, {Sp{\"a}th}).
func braced(src string, i int) (string, int) {
	if i >= len(src) || src[i] != '{' {
		return "", i
	}
	depth := 0
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i+1 : j], j + 1
			}
		}
	}
	return src[i+1:], len(src)
}

// blClean removes the name-delimiter macros biber writes INSIDE its data —
// \bibinitperiod after an initial, \bibinitdelim and \bibnamedelima/b between
// parts of a compound name. They are biber's own spacing instructions, not
// document commands, and left in they reached the page as 50 undefined control
// sequences on one paper.
var blDelimRE = regexp.MustCompile(`\\bib(initperiod|initdelim|namedelim[a-z]|rangedash|rangessep|lstinitsep|initsep)\s*`)

func blClean(s string) string {
	s = blDelimRE.ReplaceAllStringFunc(s, func(m string) string {
		switch {
		case strings.HasPrefix(m, `\bibinitperiod`):
			return ". "
		case strings.HasPrefix(m, `\bibrangedash`), strings.HasPrefix(m, `\bibrangessep`):
			return "--" // a page or date range: biber's own dash
		}
		return " "
	})
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// blField maps a biblatex field name onto the BibTeX name formatEntry reads.
func blField(name string) string {
	switch name {
	case "journaltitle", "journal":
		return "journal"
	case "booktitle":
		return "booktitle"
	case "title":
		return "title"
	case "volume", "number", "pages", "publisher", "institution", "school", "howpublished":
		return name
	case "year":
		return "year"
	}
	return ""
}

// blYear pulls the year out of biblatex's date field, which is ISO-ish
// ("2018", "2018-05", "2018-05-02/2018-05-04").
var blYearRE = regexp.MustCompile(`\d{4}`)

// parseBiblatexBBL turns biber's data .bbl into the entries the formatter wants.
func parseBiblatexBBL(src string) []bibEntry {
	var out []bibEntry
	for _, loc := range blEntryRE.FindAllStringSubmatchIndex(src, -1) {
		key := src[loc[2]:loc[3]]
		typ := strings.ToLower(src[loc[4]:loc[5]])
		body := src[loc[1]:]
		if end := strings.Index(body, `\endentry`); end >= 0 {
			body = body[:end]
		}
		en := bibEntry{typ: typ, key: key, fields: map[string]string{}}

		// Names: biber writes family={…} and given={…} per author, in order. Only
		// the FIRST \name block is read — biber lists author, then editor, then
		// translator, and the formatter wants the authors.
		if nb := strings.Index(body, `\name{`); nb >= 0 {
			seg := body[nb:]
			if ne := strings.Index(seg, "\n    }"); ne > 0 {
				seg = seg[:ne]
			}
			var names []string
			var fam string
			for _, m := range blNamePRE.FindAllStringSubmatchIndex(seg, -1) {
				val, _ := braced(seg, m[1]-1)
				switch seg[m[2]:m[3]] {
				case "family":
					if fam != "" {
						names = append(names, fam)
					}
					fam = blClean(val)
				case "given":
					if fam != "" {
						names = append(names, fam+", "+blClean(val))
						fam = ""
					}
				}
			}
			if fam != "" {
				names = append(names, fam)
			}
			if len(names) > 0 {
				en.fields["author"] = strings.Join(names, " and ")
			}
		}

		// \list{publisher}{1}{% {Addison-Wesley}% }
		for _, m := range blListRE.FindAllStringSubmatchIndex(body, -1) {
			name := blField(body[m[2]:m[3]])
			if name == "" {
				continue
			}
			val, _ := braced(body, m[1]-1)
			val = strings.TrimSpace(strings.Trim(strings.TrimSpace(val), "%"))
			if inner, _ := braced(val, 0); inner != "" {
				val = inner
			}
			if _, seen := en.fields[name]; !seen {
				en.fields[name] = val
			}
		}

		for _, m := range blFieldRE.FindAllStringSubmatchIndex(body, -1) {
			raw := body[m[2]:m[3]]
			val, _ := braced(body, m[1]-1)
			if raw == "date" || raw == "year" {
				if y := blYearRE.FindString(val); y != "" {
					if _, seen := en.fields["year"]; !seen {
						en.fields["year"] = y
					}
				}
				continue
			}
			name := blField(raw)
			if name == "" {
				continue
			}
			if _, seen := en.fields[name]; !seen {
				en.fields[name] = blClean(val)
			}
		}
		out = append(out, en)
	}
	return out
}
