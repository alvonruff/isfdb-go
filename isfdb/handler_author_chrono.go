// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

// AuthorChronoHandler serves /author_chrono.cgi?author_id — the chronological
// bibliography page.  It matches Python's ch.py / biblio.py (page_type='Chronological').
//
// Key differences from the Summary page (author.cgi):
//   - Titles are sorted by date (unknown dates last), then title — no series grouping.
//   - Variant sub-lists ARE shown under each canonical title (same as Summary).
//   - Type tags are suppressed within their own section (SERIES_TYPE_OTHER).
//
// Key difference from the Alphabetical page (author_alpha.cgi):
//   - Only canonical titles are listed; variant titles appear as sub-entries
//     under their parent, not as first-class entries in the flat list.
func AuthorChronoHandler(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a, err := SQLloadAuthorData(DB, id)
	if err != nil {
		http.Error(w, "Author not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	authorLangID := 0
	if a.AuthorLanguage.Valid {
		authorLangID = int(a.AuthorLanguage.Int32)
	}

	// ── Load canonical titles in chronological order ───────────────────────
	canonicals, err := sqlLoadCanonicalTitlesChrono(DB, id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// ── Load variant / serial titles and build display maps ───────────────
	vts, err := sqlLoadVTsForAuthor(DB, id)
	if err != nil {
		log.Println(err)
	}
	variantDict, serialDict := buildVariants(canonicals, vts)

	// Collect all VT IDs for the authors and parentsWithPubs queries
	var vtIDs []int
	for _, vs := range variantDict {
		for _, v := range vs {
			vtIDs = append(vtIDs, v.TitleID)
		}
	}
	for _, vs := range serialDict {
		for _, v := range vs {
			vtIDs = append(vtIDs, v.TitleID)
		}
	}

	// ── Load parent (canonical) authors — exclude main author ─────────────
	canonicalIDs := make([]int, len(canonicals))
	for i, t := range canonicals {
		canonicalIDs[i] = t.TitleID
	}
	parentAuthors, err := sqlLoadCoAuthors(DB, canonicalIDs, id)
	if err != nil {
		log.Println(err)
		parentAuthors = map[int][]AuthorRef{}
	}

	// ── Load variant authors ───────────────────────────────────────────────
	variantAuthors, err := sqlLoadCoAuthors(DB, vtIDs, id)
	if err != nil {
		log.Println(err)
		variantAuthors = map[int][]AuthorRef{}
	}

	// ── Determine which parent titles appear directly in a publication ─────
	parentsWithPubs, err := sqlTitlesWithPubs(DB, vtIDs)
	if err != nil {
		log.Println(err)
		parentsWithPubs = map[int]bool{}
	}

	// ── Assemble display context ───────────────────────────────────────────
	bd := &BibliographyData{
		ParentAuthors:        parentAuthors,
		VariantTitles:        variantDict,
		SerialTitles:         serialDict,
		ParentTitlesWithPubs: parentsWithPubs,
		VariantAuthors:       variantAuthors,
	}

	// ── Render ────────────────────────────────────────────────────────────
	pageTitle := "Chronological Bibliography: " + a.AuthorCanonical.String
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "author", "", "")

	printAuthorMetadataBox(w, a, id)

	fmt.Fprintln(w, `<div class="ContentBox">`)
	printBiblioViewLinks(w, id, "Chronological")

	// Genre pass, then non-genre pass
	printChronoSummary(w, canonicals, bd, id, authorLangID, false)
	if hasNonGenreTitles(canonicals) {
		fmt.Fprintln(w, `<hr>`)
		fmt.Fprintln(w, `<div class="nongenre"><b>Non-Genre Titles</b></div><br>`)
		printChronoSummary(w, canonicals, bd, id, authorLangID, true)
	}

	fmt.Fprintln(w, `</div>`) // ContentBox
	fmt.Fprintln(w, `</div>`) // content
	HTMLtrailer(w)
}

// printChronoSummary renders one genre pass of the chronological bibliography.
// It iterates over orderedSections, skips series buckets, and calls
// printChronoWorks for each title-type string.
func printChronoSummary(w io.Writer, titles []*Title, bd *BibliographyData,
	authorID, authorLangID int, nongenre bool) {

	for _, section := range orderedSections {
		titleType, ok := section.(string)
		if !ok {
			continue // skip series-type int buckets
		}
		printChronoWorks(w, titles, bd, titleType, authorID, authorLangID, nongenre)
	}
}

// printChronoWorks renders one title-type section of the chronological
// bibliography.  Unlike the alphabetical page, variant sub-lists ARE shown
// under each canonical title.
func printChronoWorks(w io.Writer, titles []*Title, bd *BibliographyData,
	titleType string, authorID, authorLangID int, nongenre bool) {

	first := true
	for _, t := range titles {
		if t.TitleTType.String != titleType {
			continue
		}
		isNonGenre := t.TitleNonGenre.String == "Yes"
		if nongenre && !isNonGenre {
			continue
		}
		if !nongenre && isNonGenre {
			continue
		}

		if first {
			label, ok := titleTypeLabel[titleType]
			if !ok {
				label = titleType
			}
			fmt.Fprintf(w, "<b>%s</b>\n", label)
			fmt.Fprintln(w, `<ul>`)
			first = false
		}

		fmt.Fprintln(w, `<li>`)
		coAuthors := bd.ParentAuthors[t.TitleID]
		displayMainTitle(w, t, authorID, coAuthors, SeriesTypeOther, authorLangID, nongenre)
		displayVariants(w, t, bd, authorID, authorLangID)
	}

	if !first {
		fmt.Fprintln(w, `</ul>`)
	}
}
