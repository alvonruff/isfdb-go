// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"log"
	"strings"
)

const bullet = "•"
const enspace = "&ensp;"

// titleCase capitalises only the first letter of an ASCII string.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// formatAuthors formats a slice of AuthorRef as linked author names joined by " <b>and</b> ".
func formatAuthors(authors []AuthorRef) string {
	parts := make([]string, len(authors))
	for i, a := range authors {
		parts[i] = BuildAuthorLink(a.AuthorID, a.Canonical)
	}
	return strings.Join(parts, " <b>and</b> ")
}

// formatAuthorsWrap is like formatAuthors but wraps the result with prefix and suffix
// when at least one author exists.
func formatAuthorsWrap(authors []AuthorRef, prefix, suffix string) string {
	if len(authors) == 0 {
		return ""
	}
	return prefix + formatAuthors(authors) + suffix
}

// PrintContents renders the publication contents ContentBox (the bottom section).
func PrintContents(w io.Writer, pubTitles []*Title, p *Pub) {
	fmt.Fprintln(w, `<div class="ContentBox">`)

	// Build a fast lookup map: titleID -> *Title
	titleMap := make(map[int]*Title, len(pubTitles))
	for _, t := range pubTitles {
		titleMap[t.TitleID] = t
	}

	// ── Find the container/referral title ────────────────────────────────
	var referenceTitle *Title
	for _, t := range pubTitles {
		ttype := t.TitleTType.String
		if ttype == p.PubCType.String {
			referenceTitle = t
			break
		}
		if (p.PubCType.String == "MAGAZINE" || p.PubCType.String == "FANZINE") && ttype == "EDITOR" {
			referenceTitle = t
			break
		}
	}

	referenceLangID := 0
	if referenceTitle != nil && referenceTitle.TitleLanguage.Valid {
		referenceLangID = int(referenceTitle.TitleLanguage.Int32)
	}

	// ── Determine if there are displayable content titles ─────────────────
	displayContents := false
	for _, t := range pubTitles {
		if referenceTitle != nil && t.TitleID == referenceTitle.TitleID && t.TitleTType.String != "NOVEL" {
			continue
		}
		if t.TitleTType.String == "COVERART" {
			continue
		}
		displayContents = true
		break
	}

	// ── Pre-fetch all data needed by printTitleLine ───────────────────────
	// Collect IDs we need to batch-load.
	parentIDset := make(map[int]struct{})
	seriesIDset := make(map[int]struct{})
	var reviewIDs []int

	for _, t := range pubTitles {
		if t.TitleParent != 0 {
			parentIDset[t.TitleParent] = struct{}{}
		}
		if t.TitleTType.String == "REVIEW" {
			reviewIDs = append(reviewIDs, t.TitleID)
		}
		if t.SeriesID.Valid {
			seriesIDset[int(t.SeriesID.Int32)] = struct{}{}
		}
	}

	// Load parent titles
	parentIDs := make([]int, 0, len(parentIDset))
	for id := range parentIDset {
		parentIDs = append(parentIDs, id)
	}
	parentTitles, err := SQLloadTitlesBatch(DB, parentIDs)
	if err != nil {
		log.Println(err)
		parentTitles = map[int]*Title{}
	}

	// Parent titles may introduce additional series IDs
	for _, pt := range parentTitles {
		if pt.SeriesID.Valid {
			seriesIDset[int(pt.SeriesID.Int32)] = struct{}{}
		}
	}

	// Load series names
	seriesIDs := make([]int, 0, len(seriesIDset))
	for id := range seriesIDset {
		seriesIDs = append(seriesIDs, id)
	}
	seriesNames, err := SQLgetSeriesNamesBatch(DB, seriesIDs)
	if err != nil {
		log.Println(err)
		seriesNames = map[int]string{}
	}

	// Load reviewed title IDs
	reviewedTitles, err := SQLfindReviewedTitlesBatch(DB, reviewIDs)
	if err != nil {
		log.Println(err)
		reviewedTitles = map[int]int{}
	}

	// Load reviewed work authors (ca_status=3) for all review titles
	reviewedAuthors, err := SQLReviewedAuthorsBatch(DB, reviewIDs)
	if err != nil {
		log.Println(err)
		reviewedAuthors = map[int][]AuthorRef{}
	}

	// Collect all title IDs that need author data (titles + parents)
	authorIDset := make(map[int]struct{})
	for _, t := range pubTitles {
		authorIDset[t.TitleID] = struct{}{}
	}
	for id := range parentIDset {
		authorIDset[id] = struct{}{}
	}
	authorIDlist := make([]int, 0, len(authorIDset))
	for id := range authorIDset {
		authorIDlist = append(authorIDlist, id)
	}
	authorCache, err := SQLTitleAuthorsBatch(DB, authorIDlist)
	if err != nil {
		log.Println(err)
		authorCache = map[int][]AuthorRef{}
	}

	// ── Display container title (non-NOVEL) ───────────────────────────────
	if referenceTitle != nil && referenceTitle.TitleTType.String != "NOVEL" {
		fmt.Fprintf(w, `<span class="containertitle">%s Title:</span>`,
			titleCase(referenceTitle.TitleTType.String))
		printTitleLine(w, referenceTitle, "", referenceLangID, true, p.PubYear.String,
			parentTitles, seriesNames, authorCache, reviewedTitles, reviewedAuthors)
	}

	// ── Contents section ──────────────────────────────────────────────────
	if !displayContents {
		fmt.Fprintln(w, `</div>`)
		return
	}

	fmt.Fprintln(w, `<h2>Contents</h2>`)
	fmt.Fprintln(w, `<ul>`)

	sortedContents, err := GetSortedPubContents(DB, p.PubID)
	if err != nil {
		log.Println(err)
		fmt.Fprintln(w, `</ul>`)
		fmt.Fprintln(w, `</div>`)
		return
	}

	containers := map[string]bool{
		"OMNIBUS": true, "COLLECTION": true, "ANTHOLOGY": true,
		"NONFICTION": true, "CHAPBOOK": true,
	}
	printed := make(map[int]bool)
	firstContainer := true

	for _, pc := range sortedContents {
		t, ok := titleMap[pc.TitleID]
		if !ok {
			continue
		}
		ttype := t.TitleTType.String

		// Skip INTERIORART and REVIEW in concise mode (we always use full)
		// Skip already-printed titles
		if printed[t.TitleID] {
			continue
		}
		// Skip COVERART, EDITOR, MAGAZINE, FANZINE
		if ttype == "COVERART" || ttype == "EDITOR" || ttype == "MAGAZINE" || ttype == "FANZINE" {
			continue
		}
		// Skip titles without a type
		if ttype == "" {
			continue
		}
		// Skip non-NOVEL reference title
		if referenceTitle != nil && t.TitleID == referenceTitle.TitleID && ttype != "NOVEL" {
			continue
		}
		// Suppress first container title that matches pub type
		if containers[ttype] && ttype == p.PubCType.String {
			if firstContainer {
				firstContainer = false
				continue
			}
		}

		// Display page number (left of pipe only)
		displayPage := pc.Page
		if idx := strings.Index(displayPage, "|"); idx >= 0 {
			displayPage = displayPage[:idx]
		}

		printTitleLine(w, t, displayPage, referenceLangID, false, p.PubYear.String,
			parentTitles, seriesNames, authorCache, reviewedTitles, reviewedAuthors)
		printed[t.TitleID] = true
	}

	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `</div>`)
}

// printTitleLine renders one content item, matching Python's PrintTitleLine().
// reference=true means no <li> prefix (used for the container title).
func printTitleLine(
	w io.Writer,
	t *Title,
	page string,
	referenceLangID int,
	reference bool,
	pubYear string,
	parentTitles map[int]*Title,
	seriesNames map[int]string,
	authorCache map[int][]AuthorRef,
	reviewedTitles map[int]int,
	reviewedAuthors map[int][]AuthorRef,
) {
	var out strings.Builder
	ttype := t.TitleTType.String

	if !reference {
		out.WriteString("<li>")
	}

	// ── Page number ───────────────────────────────────────────────────────
	if page != "" {
		out.WriteString(page)
		out.WriteString(" " + bullet + " ")
	}

	// ── Title / type-specific prefix ──────────────────────────────────────
	titleLink := fmt.Sprintf(`<a href="/title.cgi?%d">%s</a>`, t.TitleID, ISFDBText(t.TitleTitle.String))

	switch {
	case ttype == "REVIEW":
		out.WriteString(enspace + " ")
		reviewLink := fmt.Sprintf(`<a href="/title.cgi?%d">Review</a>`, t.TitleID)
		out.WriteString(reviewLink + ": ")
		// Reviewed work title (linked if we know the reviewed title ID)
		if parentID, ok := reviewedTitles[t.TitleID]; ok && parentID != 0 {
			out.WriteString(fmt.Sprintf(`<a href="/title.cgi?%d">%s</a>`, parentID, ISFDBText(t.TitleTitle.String)))
		} else {
			out.WriteString("<i>" + ISFDBText(t.TitleTitle.String) + "</i>")
		}
		// Author of the reviewed work (ca_status=3)
		if workAuthors := reviewedAuthors[t.TitleID]; len(workAuthors) > 0 {
			out.WriteString(" by " + formatAuthors(workAuthors))
		}
		// Reviewer (ca_status=1)
		if reviewers := authorCache[t.TitleID]; len(reviewers) > 0 {
			out.WriteString(" - review by " + formatAuthors(reviewers))
		}

	case ttype == "INTERIORART",
		ttype == "ESSAY" && len(t.TitleTitle.String) >= 6 && t.TitleTitle.String[:6] == "Letter":
		out.WriteString(enspace + " " + titleLink)

	default:
		out.WriteString(titleLink)
	}

	// ── Language label ────────────────────────────────────────────────────
	if referenceLangID != 0 && t.TitleLanguage.Valid {
		langID := int(t.TitleLanguage.Int32)
		if langID != referenceLangID {
			if langName, ok := Languages[langID]; ok {
				out.WriteString(" [" + ISFDBText(langName) + "]")
			}
		}
	}

	// ── Flags ─────────────────────────────────────────────────────────────
	if t.TitleJVN.Valid && t.TitleJVN.String == "Yes" {
		out.WriteString(" " + bullet + " juvenile")
	}
	if t.TitleNVZ.Valid && t.TitleNVZ.String == "Yes" {
		out.WriteString(" " + bullet + " novelization")
	}
	if t.TitleNonGenre.Valid && t.TitleNonGenre.String == "Yes" {
		out.WriteString(" " + bullet + " non-genre")
	}
	if t.TitleGraphic.Valid && t.TitleGraphic.String == "Yes" {
		out.WriteString(" " + bullet + " graphic format")
	}

	// ── Series ────────────────────────────────────────────────────────────
	if t.SeriesID.Valid {
		seriesID := int(t.SeriesID.Int32)
		if sname, ok := seriesNames[seriesID]; ok && sname != "" {
			out.WriteString(" " + bullet + " [")
			out.WriteString(fmt.Sprintf(`<a href="/pe.cgi?%d">%s</a>`, seriesID, ISFDBText(sname)))
			if t.TitleSeriesNum.Valid {
				out.WriteString(fmt.Sprintf(" %s %d", bullet, t.TitleSeriesNum.Int32))
				if t.TitleSeriesNum2.Valid && t.TitleSeriesNum2.String != "" {
					out.WriteString("." + t.TitleSeriesNum2.String)
				}
			}
			out.WriteString("]")
		}
	} else if t.TitleParent != 0 {
		// Check parent's series
		if pt, ok := parentTitles[t.TitleParent]; ok && pt.SeriesID.Valid {
			seriesID := int(pt.SeriesID.Int32)
			if sname, ok := seriesNames[seriesID]; ok && sname != "" {
				out.WriteString(" " + bullet + " [")
				out.WriteString(fmt.Sprintf(`<a href="/pe.cgi?%d">%s</a>`, seriesID, ISFDBText(sname)))
				if pt.TitleSeriesNum.Valid {
					out.WriteString(fmt.Sprintf(" %s %d", bullet, pt.TitleSeriesNum.Int32))
					if pt.TitleSeriesNum2.Valid && pt.TitleSeriesNum2.String != "" {
						out.WriteString("." + pt.TitleSeriesNum2.String)
					}
				}
				out.WriteString("]")
			}
		}
	}

	// REVIEW titles are fully rendered above; skip year/type/authors tail.
	if ttype == "REVIEW" {
		fmt.Fprint(w, out.String())
		return
	}

	// ── Bullet before year/type ───────────────────────────────────────────
	if ttype != "COVERART" {
		out.WriteString(" " + bullet + " ")
	} else {
		out.WriteString(" ")
	}

	// ── Year (if different from pub year) ─────────────────────────────────
	titleYear := ""
	if t.TitleCopyright.Valid && len(t.TitleCopyright.String) >= 4 {
		titleYear = t.TitleCopyright.String[:4]
	}
	pubYear4 := ""
	if len(pubYear) >= 4 {
		pubYear4 = pubYear[:4]
	}
	if titleYear != "" && titleYear != pubYear4 {
		out.WriteString("(" + ISFDBconvertYear(titleYear) + ")")
		out.WriteString(" " + bullet + " ")
	}

	// ── Type label ────────────────────────────────────────────────────────
	switch ttype {
	case "COLLECTION":
		out.WriteString("collection")
	case "ANTHOLOGY":
		out.WriteString("anthology")
	case "SHORTFICTION":
		if t.TitleStoryLen.Valid && t.TitleStoryLen.String != "" {
			out.WriteString(t.TitleStoryLen.String)
		} else {
			out.WriteString("short fiction")
		}
	case "ESSAY":
		out.WriteString("essay")
	case "NOVEL":
		out.WriteString("novel")
	case "OMNIBUS":
		out.WriteString("omnibus")
	case "NONFICTION":
		out.WriteString("nonfiction")
	case "CHAPBOOK":
		out.WriteString("chapbook")
	case "POEM":
		out.WriteString("poem")
	case "SERIAL":
		out.WriteString("serial")
	case "INTERIORART":
		out.WriteString("interior artwork")
	case "REVIEW":
		out.WriteString("review")
	case "INTERVIEW":
		out.WriteString(" interview of ")
		if interviewees := authorCache[t.TitleID]; len(interviewees) > 0 {
			out.WriteString(formatAuthors(interviewees))
		}
		out.WriteString(" " + bullet + " interview")
	case "EDITOR":
		out.WriteString("edited")
	case "COVERART":
		// no type label
	default:
		out.WriteString(ttype)
	}

	// ── " by " + authors ──────────────────────────────────────────────────
	out.WriteString(" by ")

	if t.TitleParent != 0 {
		parentAuthors := authorCache[t.TitleParent]
		out.WriteString(formatAuthors(parentAuthors))

		if pt, ok := parentTitles[t.TitleParent]; ok {
			// Determine if variant authors differ from parent authors (pseudonym)
			variantAuthors := authorCache[t.TitleID]
			printPseudonym := !authorIDsEqual(variantAuthors, parentAuthors)

			// Determine the relationship label
			parentLangID := 0
			if pt.TitleLanguage.Valid {
				parentLangID = int(pt.TitleLanguage.Int32)
			}
			variantLangID := 0
			if t.TitleLanguage.Valid {
				variantLangID = int(t.TitleLanguage.Int32)
			}
			translation := parentLangID != 0 && variantLangID != 0 &&
				parentLangID != variantLangID &&
				ttype != "INTERIORART" && ttype != "COVERART"

			var aka string
			switch {
			case translation:
				aka = "trans. of"
			case ttype == "SERIAL":
				aka = "book publication as"
			default:
				aka = "variant of"
			}

			interiorCoverVT := false
			if ttype == "INTERIORART" && pt.TitleTType.String == "COVERART" {
				aka += " cover art for"
				interiorCoverVT = true
			}

			// For SERIALs, suppress parent display if titles are identical up to first " ("
			displayParent := true
			if ttype == "SERIAL" && !translation &&
				(pt.TitleTType.String == "NOVEL" || pt.TitleTType.String == "SHORTFICTION") {
				pos := strings.Index(t.TitleTitle.String, " (")
				if pos > 0 && pt.TitleTitle.String == t.TitleTitle.String[:pos] {
					displayParent = false
				}
			}

			if displayParent && (pt.TitleTitle.String != t.TitleTitle.String || translation || interiorCoverVT) {
				parentLink := fmt.Sprintf(`<a href="/title.cgi?%d" class="italic">%s</a>`,
					pt.TitleID, ISFDBText(pt.TitleTitle.String))
				out.WriteString(fmt.Sprintf(" (%s %s", aka, parentLink))
				parentYear := ""
				if pt.TitleCopyright.Valid && len(pt.TitleCopyright.String) >= 4 {
					parentYear = pt.TitleCopyright.String[:4]
				}
				if parentYear != "" && parentYear != titleYear {
					out.WriteString(" " + ISFDBconvertYear(parentYear))
				}
				out.WriteString(")")
			}

			if printPseudonym {
				out.WriteString(formatAuthorsWrap(variantAuthors, " [as by ", "]"))
			}
		} else {
			out.WriteString(" <b>[PARENT TITLE ERROR]</b>")
		}
	} else {
		out.WriteString(formatAuthors(authorCache[t.TitleID]))
	}

	fmt.Fprintln(w, out.String())
}
