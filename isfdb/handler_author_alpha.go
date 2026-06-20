// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"net/http"
	"log"
	"sort"
	"strings"
)

// AuthorAlphaHandler serves /author_alpha.cgi?author_id — the alphabetical
// bibliography page.  It matches Python's ae.py / biblio.py (page_type='Alphabetical').
//
// Key differences from the Summary page (author.cgi):
//   - Titles are sorted by (lower(title), copyright) — canonical AND variant
//     titles merged together as first-class alphabetical entries.
//   - No series grouping sections.
//   - No variant sub-lists under each title (variants appear as their own
//     alphabetical entries instead).
func AuthorAlphaHandler(w http.ResponseWriter, r *http.Request) {
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

	// ── Load titles: canonical + VTs merged and sorted alphabetically ──────
	canonicals, err := sqlLoadCanonicalTitlesAlpha(DB, id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// VTs for all canonical titles
	vts, err := sqlLoadVTsForAuthor(DB, id)
	if err != nil {
		log.Println(err)
	}

	// Merge and re-sort by (lower(title), copyright) — matches Python's
	// records.sort(key=lambda tup: (tup[TITLE_TITLE].lower(), tup[TITLE_YEAR]))
	allTitles := make([]*Title, 0, len(canonicals)+len(vts))
	allTitles = append(allTitles, canonicals...)
	allTitles = append(allTitles, vts...)
	sort.SliceStable(allTitles, func(i, j int) bool {
		ti := strings.ToLower(allTitles[i].TitleTitle.String)
		tj := strings.ToLower(allTitles[j].TitleTitle.String)
		if ti != tj {
			return ti < tj
		}
		return allTitles[i].TitleCopyright.String < allTitles[j].TitleCopyright.String
	})

	// ── Load parent authors for all titles (canonical + VTs) ─────────────
	allIDs := make([]int, len(allTitles))
	for i, t := range allTitles {
		allIDs[i] = t.TitleID
	}
	parentAuthors, err := sqlLoadCoAuthors(DB, allIDs, id)
	if err != nil {
		log.Println(err)
		parentAuthors = map[int][]AuthorRef{}
	}

	// ── Render ────────────────────────────────────────────────────────────
	pageTitle := "Alphabetical Bibliography: " + a.AuthorCanonical.String
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "author", "", "")

	printAuthorMetadataBox(w, a, id)

	fmt.Fprintln(w, `<div class="ContentBox">`)

	// "Other views" navigation bar
	printBiblioViewLinks(w, id, "Alphabetical")

	// Genre pass, then non-genre pass
	printAlphaSummary(w, allTitles, parentAuthors, id, authorLangID, false)
	if hasNonGenreTitles(allTitles) {
		fmt.Fprintln(w, `<hr>`)
		fmt.Fprintln(w, `<div class="nongenre"><b>Non-Genre Titles</b></div><br>`)
		printAlphaSummary(w, allTitles, parentAuthors, id, authorLangID, true)
	}

	fmt.Fprintln(w, `</div>`) // ContentBox
	fmt.Fprintln(w, `</div>`) // content
	HTMLtrailer(w)
}

// printBiblioViewLinks renders the "Other views:" navigation bar used on all
// bibliography sub-pages.  currentView is one of "Summary", "Alphabetical",
// "Chronological", "Awards".
func printBiblioViewLinks(w io.Writer, authorID int, currentView string) {
	fmt.Fprintln(w, `<table class="bibliolinks">`)
	fmt.Fprintln(w, `<tr>`)
	fmt.Fprintln(w, `<td><b>Other views:</b></td>`)
	fmt.Fprint(w, `<td class="authorbiblios"><b>`)
	if currentView != "Summary" {
		fmt.Fprintf(w, ` <a href="/author.cgi?%d">Summary</a>`, authorID)
	}
	if currentView != "Awards" {
		fmt.Fprintf(w, ` <a href="/author_awards.cgi?%d">Awards</a>`, authorID)
	}
	if currentView != "Alphabetical" {
		fmt.Fprintf(w, ` <a href="/author_alpha.cgi?%d">Alphabetical</a>`, authorID)
	}
	if currentView != "Chronological" {
		fmt.Fprintf(w, ` <a href="/author_chrono.cgi?%d">Chronological</a>`, authorID)
	}
	fmt.Fprintln(w, `</b></td>`)
	fmt.Fprintln(w, `</tr>`)
	fmt.Fprintln(w, `</table>`)
	fmt.Fprintln(w, `<br>`)
}

// hasNonGenreTitles returns true if any title in the list is non-genre.
func hasNonGenreTitles(titles []*Title) bool {
	for _, t := range titles {
		if t.TitleNonGenre.String == "Yes" {
			return true
		}
	}
	return false
}

// printAlphaSummary renders the genre or non-genre pass of the alphabetical
// bibliography.  It iterates over orderedSections, skipping series buckets
// (ints), and for each title-type string calls printAlphaWorks.
func printAlphaSummary(w io.Writer, titles []*Title, parentAuthors map[int][]AuthorRef,
	authorID, authorLangID int, nongenre bool) {

	for _, section := range orderedSections {
		titleType, ok := section.(string)
		if !ok {
			continue // skip series-type int buckets
		}
		printAlphaWorks(w, titles, parentAuthors, titleType, authorID, authorLangID, nongenre)
	}
}

// printAlphaWorks renders one title-type section of the alphabetical
// bibliography.  Titles are already sorted; we just filter and display.
// Uses SERIES_TYPE_OTHER (= SeriesTypeOther) for all titles so that type-tags
// are suppressed within their own section — matching Python's displayWorks
// which passes SERIES_TYPE_OTHER to displayTitle.
func printAlphaWorks(w io.Writer, titles []*Title, parentAuthors map[int][]AuthorRef,
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
		coAuthors := parentAuthors[t.TitleID]
		displayMainTitle(w, t, authorID, coAuthors, SeriesTypeOther, authorLangID, nongenre)
		// No displayVariants — variants appear as their own flat entries
	}

	if !first {
		fmt.Fprintln(w, `</ul>`)
	}
}
