// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
)

// AwardTypeHandler serves /awardtype.cgi?<award_type_id> — overview page for
// one award type.  Matches Python's awardtype.py.
func AwardTypeHandler(w http.ResponseWriter, r *http.Request) {
	typeID, err := strconv.Atoi(r.URL.RawQuery)
	if err != nil || typeID <= 0 {
		http.Error(w, "Invalid award type ID", http.StatusBadRequest)
		return
	}

	at, err := SQLGetAwardTypeById(DB, typeID)
	if err != nil {
		http.Error(w, "Award type not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	// Optional note and webpages
	typeNote := ""
	if at.AwardTypeNoteID.Valid {
		if n, err := SQLgetNotes(DB, int(at.AwardTypeNoteID.Int32)); err == nil {
			typeNote = n
		}
	}
	webpages, _ := SQLloadAwardTypeWebpages(DB, typeID)

	// Year grid data
	allYears, err := SQLGetAwardYears(DB, typeID)
	if err != nil {
		log.Println(err)
	}

	// Category breakdown (non-empty and empty)
	breakdown, err := SQLGetAwardCatBreakdown(DB, typeID)
	if err != nil {
		log.Println(err)
	}
	emptyCats, err := SQLGetEmptyAwardCategories(DB, typeID)
	if err != nil {
		log.Println(err)
	}

	pageTitle := "Overview of " + at.AwardTypeName.String
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "award_type", "", "")

	// ── Metadata <ul> ────────────────────────────────────────────────────
	fmt.Fprintln(w, `<ul>`)

	if at.AwardTypeShortName.String != "" {
		fmt.Fprintf(w, "<li><b>Short Name:</b> %s\n", ISFDBText(at.AwardTypeShortName.String))
	}
	if at.AwardTypeName.String != "" {
		fmt.Fprintf(w, "<li><b>Full Name:</b> %s\n", ISFDBText(at.AwardTypeName.String))
	}
	if at.AwardTypeFor.String != "" {
		fmt.Fprintf(w, "<li><b>Awarded For:</b> %s\n", ISFDBText(at.AwardTypeFor.String))
	}
	if at.AwardTypeBy.String != "" {
		fmt.Fprintf(w, "<li><b>Awarded By:</b> %s\n", ISFDBText(at.AwardTypeBy.String))
	}
	if at.AwardTypePoll.String != "" {
		fmt.Fprintf(w, "<li><b>Poll:</b> %s\n", ISFDBText(at.AwardTypePoll.String))
	}
	if at.AwardTypeNonGenre.String != "" {
		fmt.Fprintf(w, "<li><b>Covers more than just SF:</b> %s\n", ISFDBText(at.AwardTypeNonGenre.String))
	}

	if len(webpages) > 0 {
		domains, _ := SQLLoadRecognizedDomains(DB)
		PrintWebPages(w, webpages, "<li>", domains)
	}

	if typeNote != "" {
		formatted := FormatNote(typeNote, "Note", "short", typeID, "AwardType", false)
		fmt.Fprintf(w, "<li>\n%s\n", formatted)
	}

	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `<p>`)

	// ── Year grid (all years linked, current=0 so none are bolded) ───────
	printAwardYearGrid(w, at, allYears, 0)

	// ── Categories ───────────────────────────────────────────────────────
	printAwardTypeCategories(w, breakdown, emptyCats)

	fmt.Fprintln(w, `</div>`) // main
	HTMLtrailer(w)
}

// printAwardTypeCategories renders the category breakdown and empty-category
// tables, matching Python's awardtypeClass.display_categories.
func printAwardTypeCategories(w io.Writer, breakdown []*AwardCatBreakdown, empty []*AwardCat) {
	if len(breakdown) > 0 {
		fmt.Fprintln(w, `<div class="generic_centered_div">`)
		fmt.Fprintln(w, `<h3>Categories</h3>`)
		fmt.Fprintln(w, `<table class="generic_centered_table">`)
		fmt.Fprintln(w, `<tr class="generic_table_header">`)
		fmt.Fprintln(w, `<th>Display Order</th>`)
		fmt.Fprintln(w, `<th>Category</th>`)
		fmt.Fprintln(w, `<th>Wins</th>`)
		fmt.Fprintln(w, `<th>All awards and nominations</th>`)
		fmt.Fprintln(w, `</tr>`)

		for _, b := range breakdown {
			fmt.Fprintln(w, `<tr class="generic_table_header">`)
			dispOrder := ""
			if b.CatOrder.Valid {
				dispOrder = strconv.Itoa(int(b.CatOrder.Int32))
			}
			fmt.Fprintf(w, "<td>%s</td>\n", dispOrder)
			fmt.Fprintf(w, "<td><a href=\"/award_category.cgi?%d+0\">%s</a></td>\n",
				b.CatID, ISFDBText(b.CatName))
			fmt.Fprintf(w, "<td><a href=\"/award_category.cgi?%d+0\">%d</a></td>\n",
				b.CatID, b.Wins)
			fmt.Fprintf(w, "<td><a href=\"/award_category.cgi?%d+1\">%d</a></td>\n",
				b.CatID, b.Total)
			fmt.Fprintln(w, `</tr>`)
		}

		fmt.Fprintln(w, `</table>`)
		fmt.Fprintln(w, `</div>`)
	}

	if len(empty) > 0 {
		fmt.Fprintln(w, `<div class="generic_centered_div">`)
		fmt.Fprintln(w, `<h3>Empty Categories</h3>`)
		fmt.Fprintln(w, `<table class="generic_centered_table">`)
		fmt.Fprintln(w, `<tr class="generic_table_header">`)
		fmt.Fprintln(w, `<th>Display Order</th>`)
		fmt.Fprintln(w, `<th>Category</th>`)
		fmt.Fprintln(w, `</tr>`)

		for _, c := range empty {
			fmt.Fprintln(w, `<tr class="generic_table_header">`)
			dispOrder := ""
			if c.AwardCatOrder.Valid {
				dispOrder = strconv.Itoa(int(c.AwardCatOrder.Int32))
			}
			fmt.Fprintf(w, "<td>%s</td>\n", dispOrder)
			fmt.Fprintf(w, "<td><a href=\"/award_category.cgi?%d+1\">%s</a></td>\n",
				c.AwardCatID, ISFDBText(c.AwardCatName.String))
			fmt.Fprintln(w, `</tr>`)
		}

		fmt.Fprintln(w, `</table>`)
		fmt.Fprintln(w, `</div>`)
	}
}
