// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
)

// ── Year helpers ──────────────────────────────────────────────────────────────

// convertTitleYearStr returns the display year string for a title.
// Returns "unknown" for 0000 or missing dates, matching Python convertTitleYear().
func convertTitleYearStr(t *Title) string {
	if !t.TitleCopyright.Valid || t.TitleCopyright.String == "" {
		return "unknown"
	}
	if len(t.TitleCopyright.String) < 4 {
		return "unknown"
	}
	return ISFDBconvertYear(t.TitleCopyright.String[:4])
}

// ── Core title line ───────────────────────────────────────────────────────────

// buildCoreTitleLine builds the core title link + optional language tag + year
// for a bibliography entry.  Matches Python's buildCoreTitleLine().
func buildCoreTitleLine(t *Title, authorLangID int) string {
	link := fmt.Sprintf(` <a href="/title.cgi?%d" class="italic">%s</a> `,
		t.TitleID, ISFDBText(t.TitleTitle.String))

	// Language label only when title language differs from the author's language
	langStr := ""
	if t.TitleLanguage.Valid && authorLangID != 0 &&
		int(t.TitleLanguage.Int32) != authorLangID {
		if langName, ok := Languages[int(t.TitleLanguage.Int32)]; ok {
			langStr = fmt.Sprintf("[%s] ", ISFDBText(langName))
		}
	}

	yearStr := fmt.Sprintf("(<b>%s</b>)", convertTitleYearStr(t))
	return link + langStr + yearStr
}

// ── Author display helpers ─────────────────────────────────────────────────────

// bibAuthorLinks returns a slice of HTML author links.
func bibAuthorLinks(authors []AuthorRef) []string {
	links := make([]string, len(authors))
	for i, a := range authors {
		links[i] = BuildAuthorLink(a.AuthorID, a.Canonical)
	}
	return links
}

// bibDisplayAuthorList writes a list of author links joined by " <b>and</b> ".
func bibDisplayAuthorList(w io.Writer, authors []AuthorRef) {
	fmt.Fprint(w, strings.Join(bibAuthorLinks(authors), " <b>and</b> "))
}

// bibDisplayVariantAuthors writes "[also/only as by Name]" inline.
// qualifier is "", "also " or "only ".
func bibDisplayVariantAuthors(w io.Writer, authors []AuthorRef, qualifier string) {
	if len(authors) == 0 {
		return
	}
	fmt.Fprintf(w, " [<b>%sas by</b> %s]",
		qualifier,
		strings.Join(bibAuthorLinks(authors), " <b>and</b> "))
}

// ── REVIEW author display ─────────────────────────────────────────────────────

// bibDisplayAuthorsForReview renders: "<b>by</b> <reviewed_authors> (co-reviewed with X)"
// Matches Python's displayAuthorsforReview().
func bibDisplayAuthorsForReview(w io.Writer, coAuthors []AuthorRef, t *Title, authorID int) {
	reviewed, err := SQLReviewedAuthors(DB, t.TitleID)
	if err != nil {
		log.Println(err)
	}
	fmt.Fprint(w, " <b>by</b> ")
	for i, a := range reviewed {
		if i > 0 {
			fmt.Fprint(w, " <b>and</b> ")
		}
		fmt.Fprint(w, BuildAuthorLink(a.AuthorID, a.Canonical))
	}

	if len(coAuthors) == 0 {
		fmt.Fprintln(w)
		return
	}

	var output strings.Builder
	for i, a := range coAuthors {
		if i == 0 {
			if authorID != 0 {
				output.WriteString("(co-reviewed with ")
			} else {
				output.WriteString("(reviewed by ")
			}
		} else {
			output.WriteString(" <b>and</b> ")
		}
		output.WriteString(BuildAuthorLink(a.AuthorID, a.Canonical))
	}
	output.WriteString(")")
	fmt.Fprintln(w, output.String())
}

// ── INTERVIEW author display ──────────────────────────────────────────────────

// bibDisplayAuthorsForInterview renders: "<b>with</b> <interviewee_authors> (co-interviewer X)"
// Matches Python's displayAuthorsforInterview().
func bibDisplayAuthorsForInterview(w io.Writer, coAuthors []AuthorRef, t *Title, authorID int) {
	interviewees, err := SQLIntervieweeAuthors(DB, t.TitleID, authorID)
	if err != nil {
		log.Println(err)
	}
	fmt.Fprint(w, " <b>with</b> ")
	for i, a := range interviewees {
		if i > 0 {
			fmt.Fprint(w, " <b>and</b> ")
		}
		fmt.Fprint(w, BuildAuthorLink(a.AuthorID, a.Canonical))
	}

	if len(coAuthors) == 0 {
		fmt.Fprintln(w)
		return
	}

	var output strings.Builder
	for i, a := range coAuthors {
		if i == 0 {
			if authorID != 0 {
				if len(coAuthors) == 1 {
					output.WriteString("(co-interviewer ")
				} else {
					output.WriteString("(co-interviewers ")
				}
			} else {
				output.WriteString("(interviewed by ")
			}
		} else {
			output.WriteString(" <b>and</b> ")
		}
		output.WriteString(BuildAuthorLink(a.AuthorID, a.Canonical))
	}
	output.WriteString(")")
	fmt.Fprintln(w, output.String())
}

// ── Main title display ────────────────────────────────────────────────────────

// displayMainTitle renders one canonical title's line: title link, type tags,
// [non-genre], [graphic format], and co-author attribution.
// Matches Python's displayMainTitle().
func displayMainTitle(w io.Writer, t *Title, authorID int, coAuthors []AuthorRef,
	seriesType int, authorLangID int, nongenre bool) {

	output := buildCoreTitleLine(t, authorLangID)
	ttype := t.TitleTType.String

	switch ttype {
	case "NOVEL":
		// Novel never gets a type tag

	case "OMNIBUS":
		if t.TitleContent.Valid && t.TitleContent.String != "" {
			output += fmt.Sprintf(" <b>[O/%s]</b> ", t.TitleContent.String)
		} else {
			output += " <b>[O]</b> "
		}

	case "COLLECTION":
		if seriesType != SeriesTypeOther {
			output += " <b>[C]</b> "
		}

	default:
		if seriesType != SeriesTypeOther {
			switch ttype {
			case "ANTHOLOGY":
				if seriesType != SeriesTypeAnth {
					output += " <b>[A]</b> "
				}
			case "SHORTFICTION":
				if seriesType != SeriesTypeSF {
					output += " <b>[SF]</b> "
				}
			case "ESSAY":
				if seriesType != SeriesTypeEssay {
					output += " <b>[ES]</b> "
				}
			case "EDITOR":
				if seriesType != SeriesTypeEdit {
					output += " <b>[ED]</b> "
				}
			case "NONFICTION":
				if seriesType != SeriesTypeNonfic {
					output += " <b>[NF]</b> "
				}
			case "POEM":
				if seriesType != SeriesTypePoem {
					output += " <b>[POEM]</b> "
				}
			case "COVERART":
				if seriesType != SeriesTypeCoverArt {
					output += " <b>[COVERART]</b> "
				}
			case "INTERIORART":
				if seriesType != SeriesTypeInteriorArt {
					output += " <b>[INTERIORART]</b> "
				}
			case "REVIEW":
				if seriesType != SeriesTypeReview {
					output += " <b>[REVIEW]</b> "
				}
			case "INTERVIEW":
				if seriesType != SeriesTypeInterview {
					output += " <b>[INTERVIEW]</b> "
				}
			default:
				output += fmt.Sprintf(" <b>[%s]</b> ", ttype)
			}
		}
	}

	// Non-genre marker: shown when we're displaying the genre section
	if !nongenre && t.TitleNonGenre.String == "Yes" {
		output += " <b>[non-genre]</b>"
	}
	if t.TitleGraphic.String == "Yes" {
		output += " <b>[graphic format]</b>"
	}

	fmt.Fprintln(w, output)

	// Author attribution depends on title type
	switch ttype {
	case "REVIEW":
		bibDisplayAuthorsForReview(w, coAuthors, t, authorID)
	case "INTERVIEW":
		bibDisplayAuthorsForInterview(w, coAuthors, t, authorID)
	default:
		if len(coAuthors) > 0 {
			if authorID != 0 {
				fmt.Fprint(w, " <b>with</b> ")
			} else {
				fmt.Fprint(w, " <b>by</b>")
			}
			bibDisplayAuthorList(w, coAuthors)
			fmt.Fprintln(w)
		}
	}
}

// ── Variant display ───────────────────────────────────────────────────────────

// bibIntMapsEqual returns true when two map[int]bool contain the same keys.
func bibIntMapsEqual(a, b map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// displayVariantTitle renders a single variant or serial title line.
// Matches Python's displayVariantTitle().
func displayVariantTitle(w io.Writer, vt *Title, parent *Title, vtType string,
	bd *BibliographyData, authorID int, authorLangID int) {

	// Determine label
	isTranslation := vt.TitleLanguage.Valid && parent.TitleLanguage.Valid &&
		vt.TitleLanguage.Int32 != parent.TitleLanguage.Int32

	label := ""
	if isTranslation {
		label = "Translation"
	} else if vtType == "serial" {
		label = ""
	} else {
		label = "Variant"
		if vt.TitleTType.String == "INTERIORART" && parent.TitleTType.String == "COVERART" {
			label = "Interior Art"
		} else if parent.TitleTType.String == "INTERIORART" && vt.TitleTType.String == "COVERART" {
			label = "Cover Art"
		}
	}

	labelStr := ""
	if label != "" {
		labelStr = fmt.Sprintf(" <b>%s:</b>", label)
	}

	output := fmt.Sprintf(`%s <a href="/title.cgi?%d" class="italic">%s</a> `,
		labelStr, vt.TitleID, ISFDBText(vt.TitleTitle.String))

	if isTranslation {
		if langName, ok := Languages[int(vt.TitleLanguage.Int32)]; ok {
			output += fmt.Sprintf("[%s] ", ISFDBText(langName))
		}
	}

	output += fmt.Sprintf("(<b>%s</b>)", convertTitleYearStr(vt))

	if vt.TitleTType.String == "OMNIBUS" {
		if vt.TitleContent.Valid && vt.TitleContent.String != "" {
			output += fmt.Sprintf(" <b>[O/%s]</b> ", vt.TitleContent.String)
		} else {
			output += " <b>[O]</b> "
		}
	}

	fmt.Fprintln(w, output)

	// Show "as by" if variant authors differ from canonical authors
	canonicalIDs := make(map[int]bool)
	for _, a := range bd.ParentAuthors[parent.TitleID] {
		canonicalIDs[a.AuthorID] = true
	}
	if authorID != 0 {
		canonicalIDs[authorID] = true
	}

	variantAuthors := bd.VariantAuthors[vt.TitleID]
	variantIDs := make(map[int]bool)
	for _, a := range variantAuthors {
		variantIDs[a.AuthorID] = true
	}

	if !bibIntMapsEqual(canonicalIDs, variantIDs) {
		bibDisplayVariantAuthors(w, variantAuthors, "")
		fmt.Fprintln(w)
	}
}

// displayVariants renders the "also/only appeared as" section under a parent title.
// Matches Python's displayVariants().
func displayVariants(w io.Writer, t *Title, bd *BibliographyData, authorID int, authorLangID int) {
	variantList := bd.VariantTitles[t.TitleID]
	serialList := bd.SerialTitles[t.TitleID]

	if len(variantList) == 0 && len(serialList) == 0 {
		return
	}

	parentHasPubs := bd.ParentTitlesWithPubs[t.TitleID]

	// Decide whether variant info goes on the same line or as a sub-list.
	// "Same line" only when there is exactly 1 variant whose title, type, and
	// language all match the parent.
	titleVariation := true
	if len(variantList) == 1 {
		v := variantList[0]
		if v.TitleTitle.String == t.TitleTitle.String &&
			v.TitleTType.String == t.TitleTType.String {
			tHasLang := t.TitleLanguage.Valid
			vHasLang := v.TitleLanguage.Valid
			if !tHasLang || !vHasLang ||
				(tHasLang && vHasLang && t.TitleLanguage.Int32 == v.TitleLanguage.Int32) {
				titleVariation = false
			}
		}
	}

	if titleVariation {
		// Multi-variant or differing title: render as indented list
		if parentHasPubs {
			fmt.Fprint(w, " also appeared as:")
		} else {
			fmt.Fprint(w, " only appeared as:")
		}
		if len(variantList) > 0 {
			fmt.Fprintln(w, "<ul>")
			for _, v := range variantList {
				fmt.Fprintln(w, "<li>")
				displayVariantTitle(w, v, t, "variant", bd, authorID, authorLangID)
			}
			fmt.Fprintln(w, "</ul>")
		}
	} else if len(variantList) > 0 {
		// Single same-title variant: show "also/only as by" inline
		qualifier := "only "
		if parentHasPubs {
			qualifier = "also "
		}
		bibDisplayVariantAuthors(w, bd.VariantAuthors[variantList[0].TitleID], qualifier)
		fmt.Fprintln(w)
	}

	// Serializations
	if len(serialList) > 0 {
		fmt.Fprintln(w, "<ul>")
		fmt.Fprintln(w, "<li><b>Serializations:</b>")
		for _, s := range serialList {
			fmt.Fprintln(w, "<li>")
			displayVariantTitle(w, s, t, "serial", bd, authorID, authorLangID)
		}
		fmt.Fprintln(w, "</ul>")
	}
}

// ── Section renderers ─────────────────────────────────────────────────────────

// printOneSeries renders a single series and its sub-series recursively.
// Matches Python's printSeries().
func printOneSeries(w io.Writer, s *Series, bd *BibliographyData, seriesType int,
	nongenre bool, authorID int, authorLangID int) {

	pos := ""
	if s.ParentPosition.Valid {
		pos = fmt.Sprintf("%d ", s.ParentPosition.Int32)
	}
	fmt.Fprintf(w, "<li> %s<a href=\"/pe.cgi?%d\">%s</a>\n",
		pos, s.SeriesID, ISFDBText(s.SeriesTitle))
	fmt.Fprintln(w, "<ul>")

	for _, t := range bd.CanonicalTitles {
		if !t.SeriesID.Valid || int(t.SeriesID.Int32) != s.SeriesID {
			continue
		}

		numStr := ""
		if t.TitleSeriesNum.Valid {
			numStr = fmt.Sprintf(" %d", t.TitleSeriesNum.Int32)
			if t.TitleSeriesNum2.Valid && t.TitleSeriesNum2.String != "" {
				numStr += "." + t.TitleSeriesNum2.String
			}
		}
		fmt.Fprintf(w, "<li>%s\n", numStr)

		coAuthors := bd.ParentAuthors[t.TitleID]
		displayMainTitle(w, t, authorID, coAuthors, seriesType, authorLangID, nongenre)
		displayVariants(w, t, bd, authorID, authorLangID)
	}

	// Recurse into sub-series (already sorted by (ParentPosition, SeriesTitle) in biblio.go)
	for _, childID := range bd.SeriesParent[s.SeriesID] {
		if child, ok := bd.SeriesTree[childID]; ok {
			printOneSeries(w, child, bd, seriesType, nongenre, authorID, authorLangID)
		}
	}

	fmt.Fprintln(w, "</ul>")
}

// printSeriesType renders all top-level series of a given type bucket that
// match the current genre pass.  Matches Python's printSeriesType().
func printSeriesType(w io.Writer, bd *BibliographyData, seriesType int, nongenre bool,
	authorID int, authorLangID int) {

	// Gather top-level series of this type, filtered by genre
	var seriesList []*Series
	for topID, priority := range bd.SeriesPriority {
		if priority != seriesType {
			continue
		}
		isGenre := bd.SeriesGenre[topID]
		if !nongenre && !isGenre {
			continue // genre pass: skip non-genre series
		}
		if nongenre && isGenre {
			continue // non-genre pass: skip genre series
		}
		if s, ok := bd.SeriesTree[topID]; ok {
			seriesList = append(seriesList, s)
		}
	}
	if len(seriesList) == 0 {
		return
	}

	sort.Slice(seriesList, func(i, j int) bool {
		return seriesList[i].SeriesTitle < seriesList[j].SeriesTitle
	})

	label := seriesTypeLabel[seriesType]
	fmt.Fprintf(w, "<b>%s Series</b>\n", label)
	fmt.Fprintln(w, "<ul>")
	for i, s := range seriesList {
		printOneSeries(w, s, bd, seriesType, nongenre, authorID, authorLangID)
		if i < len(seriesList)-1 {
			fmt.Fprintln(w, "<br>")
		}
	}
	fmt.Fprintln(w, "</ul>")
}

// printWorks renders a flat (non-series) title-type section.
// Titles that belong to a series and whose type appears in seriesTypePriority
// are skipped (they were already rendered in a series section).
// Matches Python's displayWorks() for Summary pages.
func printWorks(w io.Writer, bd *BibliographyData, titleType string, nongenre bool,
	authorID int, authorLangID int) {

	_, typeInSeriesPriority := seriesTypePriority[titleType]

	first := true
	for _, t := range bd.CanonicalTitles {
		if t.TitleTType.String != titleType {
			continue
		}
		// Genre filter
		if !nongenre && t.TitleNonGenre.String == "Yes" {
			continue
		}
		if nongenre && t.TitleNonGenre.String != "Yes" {
			continue
		}
		// Skip titles that are in a series (they belong to a series section)
		if typeInSeriesPriority && t.SeriesID.Valid && t.SeriesID.Int32 != 0 {
			continue
		}

		if first {
			label := titleTypeLabel[titleType]
			if label == "" {
				label = titleType
			}
			fmt.Fprintf(w, "<b>%s</b>\n", label)
			fmt.Fprintln(w, "<ul>")
			first = false
		}
		fmt.Fprintln(w, "<li>")
		coAuthors := bd.ParentAuthors[t.TitleID]
		displayMainTitle(w, t, authorID, coAuthors, SeriesTypeOther, authorLangID, nongenre)
		displayVariants(w, t, bd, authorID, authorLangID)
	}
	if !first {
		fmt.Fprintln(w, "</ul>")
	}
}

// ── Main entry point ──────────────────────────────────────────────────────────

// PrintBibliography renders the complete bibliography ContentBox for an author.
// It performs two passes: genre works first, then non-genre works (matching
// Python's displaySummary() called twice with nongenre=0 and nongenre=1).
func PrintBibliography(w io.Writer, bd *BibliographyData, authorID int, authorLangID int) {
	if len(bd.CanonicalTitles) == 0 {
		return
	}

	fmt.Fprintln(w, `<div class="ContentBox">`)

	printBiblioViewLinks(w, authorID, "Summary")

	// Two passes: genre (pass=0) then non-genre (pass=1)
	for pass := 0; pass < 2; pass++ {
		nongenre := pass == 1

		if nongenre {
			// Only print the separator if there are non-genre titles
			hasNonGenre := false
			for _, t := range bd.CanonicalTitles {
				if t.TitleNonGenre.String == "Yes" {
					hasNonGenre = true
					break
				}
			}
			if !hasNonGenre {
				break
			}
			fmt.Fprintln(w, "<hr>")
			fmt.Fprintln(w, `<div class="nongenre"><b>Non-Genre Titles</b></div><br>`)
		}

		for _, section := range orderedSections {
			switch v := section.(type) {
			case int:
				// Series-type bucket
				printSeriesType(w, bd, v, nongenre, authorID, authorLangID)
			case string:
				// Flat title-type section
				printWorks(w, bd, v, nongenre, authorID, authorLangID)
			}
		}
	}

	fmt.Fprintln(w, `</div>`)
}
