// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
)

// AuthorAwardsHandler serves /author_awards.cgi?author_id — the author award
// bibliography page.  It matches Python's eaw.py / biblio.py (page_type='Award').
//
// Unlike the other bibliography pages it does not display the title list at all;
// instead it shows a single award table covering:
//   - Untitled (non-title-linked) awards whose award_author matches this
//     author's canonical name or any of their pseudonyms.
//   - Title-linked awards for all of this author's canonical titles.
func AuthorAwardsHandler(w http.ResponseWriter, r *http.Request) {
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

	authorName := a.AuthorCanonical.String

	// ── Collect pseudonym names ───────────────────────────────────────────
	pseudoRefs, err := SQLgetBriefPseudoFromActual(DB, id)
	if err != nil {
		log.Println(err)
	}
	pseudoNames := make([]string, 0, len(pseudoRefs))
	for _, ref := range pseudoRefs {
		if ref.Canonical != "" {
			pseudoNames = append(pseudoNames, ref.Canonical)
		}
	}

	// ── Load canonical title IDs for this author ──────────────────────────
	// Awards may be linked to variant/translation titles rather than the
	// canonical parent, so we expand the list to include all VT IDs too.
	canonicals, err := sqlLoadCanonicalTitlesChrono(DB, id) // any sort; we just need the IDs
	if err != nil {
		log.Println(err)
	}
	titleIDs := make([]int, len(canonicals))
	for i, t := range canonicals {
		titleIDs[i] = t.TitleID
	}
	vts, err := sqlLoadVTsForAuthor(DB, id)
	if err != nil {
		log.Println(err)
	}
	for _, v := range vts {
		titleIDs = append(titleIDs, v.TitleID)
	}

	// ── Load awards ───────────────────────────────────────────────────────
	awards, err := SQLloadAwardsForAuthor(DB, authorName, pseudoNames, titleIDs)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// ── Render ────────────────────────────────────────────────────────────
	pageTitle := "Award Bibliography: " + authorName
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "author", "", "")

	printAuthorMetadataBox(w, a, id)

	fmt.Fprintln(w, `<div class="ContentBox">`)
	printBiblioViewLinks(w, id, "Awards")

	if len(awards) == 0 {
		fmt.Fprintf(w, "<h2>No awards found for %s</h2>\n", ISFDBText(authorName))
	} else {
		fmt.Fprintln(w, `<p>`)
		PrintAwardTable(w, awards, true, false)
	}

	fmt.Fprintln(w, `</div>`) // ContentBox
	fmt.Fprintln(w, `</div>`) // content
	HTMLtrailer(w)
}
