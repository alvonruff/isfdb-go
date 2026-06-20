// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// FormatNote formats a raw note string, substituting templates, normalizing
// whitespace and HTML, and wrapping the result in a div.
//
// Parameters:
//   note        - raw note text from the database
//   noteType    - label shown before the note, e.g. "Note" (empty = no label)
//   displayMode - "short" truncates at {{BREAK}}, "full" removes it, "edit" leaves it
//   recordID    - ID of the parent record, used in the BREAK link
//   recordType  - type string used in the BREAK link, e.g. "title"
//   div         - if true, wrap output in <div class="notes">
func FormatNote(note, noteType, displayMode string, recordID int, recordType string, div bool) string {
	if note == "" {
		return ""
	}

	note = ISFDBHostCorrection(note, "all")

	// Handle {{BREAK}} marker
	const breakMarker = "{{BREAK}}"
	if displayMode == "short" && strings.Contains(note, breakMarker) {
		note = note[:strings.Index(note, breakMarker)]
		note += fmt.Sprintf(` ... <big><a class="inverted" href="%s://%s/note.cgi?%s+%d">view full %s</a></big>`,
			PROTOCOL, HTMLHOST, recordType, recordID, noteType)
	} else if displayMode == "full" && strings.Contains(note, breakMarker) {
		idx := strings.Index(note, breakMarker)
		note1 := strings.TrimRight(note[:idx], " ")
		note2 := strings.TrimLeft(note[idx+len(breakMarker):], " ")
		note = note1 + " " + note2
	}

	// Substitute templates
	for name, entry := range Templates {
		templateLink := entry.URL
		templateName := entry.Display
		templateDesc := entry.Mouseover

		if !strings.Contains(templateLink, "%s") {
			// Non-record-based template: {{TEMPLATENAME}}
			pattern := regexp.MustCompile(`(?i)\{\{` + regexp.QuoteMeta(name) + `\}\}`)
			var substituted string
			if templateLink == "" {
				substituted = templateName
			} else {
				substituted = fmt.Sprintf(`<a href="%s">%s</a>`, templateLink, templateName)
			}
			if templateDesc != "" {
				substituted = fmt.Sprintf(`<abbr class="template" title="%s">%s</abbr>`, templateDesc, substituted)
			}
			note = pattern.ReplaceAllString(note, substituted)
		} else {
			// Record-based template: {{TEMPLATENAME|value}}
			pattern := regexp.MustCompile(`(?i)\{\{` + regexp.QuoteMeta(name) + `\|`)
			fragments := pattern.Split(note, -1)
			if len(fragments) == 1 {
				continue
			}
			var result strings.Builder
			for count, fragment := range fragments {
				if count == 0 {
					result.WriteString(fragment)
					continue
				}
				// Split on closing }}
				parts := strings.SplitN(fragment, "}}", 2)
				linkingValue := parts[0]
				rest := ""
				if len(parts) > 1 {
					rest = parts[1]
				}
				if linkingValue != "" {
					actualLink := fmt.Sprintf(templateLink, url.QueryEscape(linkingValue))
					var displayValue string
					if templateName == "" {
						displayValue = linkingValue
					} else {
						displayValue = fmt.Sprintf("%s %s", templateName, linkingValue)
					}
					var fullValue string
					if strings.HasPrefix(templateLink, "http") {
						fullValue = fmt.Sprintf(`<a href="%s">%s</a>`, actualLink, displayValue)
					} else {
						fullValue = displayValue
					}
					if templateDesc != "" {
						fullValue = fmt.Sprintf(`<abbr class="template" title="%s">%s</abbr>`, templateDesc, fullValue)
					}
					result.WriteString(fullValue)
				}
				result.WriteString(rest)
			}
			note = result.String()
		}
	}

	// Convert MediaWiki-style level-2 section headings to bold HTML headers.
	// Handles both "== Title ==" (with closing ==) and "== Title" (without).
	// The substitution runs before whitespace normalisation so the surrounding
	// newlines are still present for pre-wrap display.
	sectionRe := regexp.MustCompile(`(?m)^==\s*(.+?)\s*(?:==)?\s*$`)
	note = sectionRe.ReplaceAllStringFunc(note, func(match string) string {
		sub := sectionRe.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		return "<b>" + sub[1] + "</b>"
	})

	retval := note

	// Remove legacy isfdb-specific comment strings
	retval = strings.ReplaceAll(retval, "<!--isfdb specific-->", "")

	// List of <br> variants to normalize
	brVariants := []string{"br", "br/", "br /", "Br", "Br/", "Br /", "BR", "BR/", "BR /"}

	// Replace double <br>s with <p>
	for _, br := range brVariants {
		double := "<" + br + "><" + br + ">"
		for strings.Contains(retval, double) {
			retval = strings.ReplaceAll(retval, double, "<p>")
		}
	}

	// Replace single <br>s with newlines
	for _, br := range brVariants {
		retval = strings.ReplaceAll(retval, "<"+br+">", "\n")
	}

	// Collapse double newlines into single
	for strings.Contains(retval, "\n\n") {
		retval = strings.ReplaceAll(retval, "\n\n", "\n")
	}

	// Clean up whitespace around block elements
	for _, elem := range []string{"p", "/p", "ul", "li", "/li"} {
		enclosed := "<" + elem + ">"
		enclosedUpper := strings.ToUpper(enclosed)
		for strings.Contains(retval, " "+enclosed) {
			retval = strings.ReplaceAll(retval, " "+enclosed, enclosed)
		}
		for strings.Contains(retval, enclosed+" ") {
			retval = strings.ReplaceAll(retval, enclosed+" ", enclosed)
		}
		retval = strings.ReplaceAll(retval, "\n"+enclosed, enclosed)
		retval = strings.ReplaceAll(retval, enclosed+"\n", enclosed)
		retval = strings.ReplaceAll(retval, "\n"+enclosedUpper, enclosedUpper)
		retval = strings.ReplaceAll(retval, enclosedUpper+"\n", enclosedUpper)
	}

	// Replace <p> with two newlines
	retval = strings.ReplaceAll(retval, "<p>", "\n\n")
	retval = strings.ReplaceAll(retval, "<P>", "\n\n")

	// Strip leading/trailing spaces (but not newlines)
	retval = strings.Trim(retval, " ")

	if div {
		if noteType != "" {
			retval = fmt.Sprintf(`<div class="notes"><b>%s:</b> %s</div>`, noteType, retval)
		} else {
			retval = fmt.Sprintf(`<div class="notes">%s</div>`, retval)
		}
	}

	return retval
}
