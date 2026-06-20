// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
)

// AwardCategoryHandler serves /award_category.cgi?<cat_id>+<win_nom>.
// win_nom=0 → wins only; win_nom=1 → all awards and nominations.
// Matches Python's award_category.py / awardcatClass.PrintAwardCatSummary.
func AwardCategoryHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 1 {
		http.Error(w, "This page requires at least one parameter", http.StatusBadRequest)
		return
	}
	catID, err := strconv.Atoi(params[0])
	if err != nil || catID <= 0 {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}
	winNom := 0 // default: wins only
	if len(params) >= 2 {
		if v, err := strconv.Atoi(params[1]); err == nil && (v == 0 || v == 1) {
			winNom = v
		}
	}

	cat, err := SQLGetAwardCatById(DB, catID)
	if err != nil {
		http.Error(w, "Award category not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	typeID := 0
	if cat.AwardCatTypeID.Valid {
		typeID = int(cat.AwardCatTypeID.Int32)
	}
	at, err := SQLGetAwardTypeById(DB, typeID)
	if err != nil {
		http.Error(w, "Award type not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	catNote := ""
	if cat.AwardCatNoteID.Valid {
		if n, err := SQLgetNotes(DB, int(cat.AwardCatNoteID.Int32)); err == nil {
			catNote = n
		}
	}
	catWebpages, _ := SQLloadAwardCatWebpages(DB, catID)

	// Load awards for this category (all years)
	awards, err := SQLloadAwardsForCat(DB, catID, winNom)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	displays, err := LoadAwardDisplayBatch(DB, awards)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Group displays by year (key = first 4 chars of award_year)
	yearOrder := []string{}
	byYear := map[string][]*AwardDisplay{}
	for _, d := range displays {
		y := d.AwardYear
		if len(y) > 4 {
			y = y[:4]
		}
		if _, seen := byYear[y]; !seen {
			yearOrder = append(yearOrder, y)
		}
		byYear[y] = append(byYear[y], d)
	}
	sort.Strings(yearOrder)

	groups := make([]AwardCatYearGroup, len(yearOrder))
	for i, y := range yearOrder {
		groups[i] = AwardCatYearGroup{Year: y, Displays: byYear[y]}
	}

	pageTitle := fmt.Sprintf("Award Category: %s (%s)",
		cat.AwardCatName.String, at.AwardTypeName.String)

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "award_cat", "", "")

	// ── Category metadata header ──────────────────────────────────────────
	printAwardCatPageHeader(w, cat, at, catNote, catWebpages)

	// ── Body text ─────────────────────────────────────────────────────────
	if winNom == 0 {
		if len(awards) > 0 {
			fmt.Fprint(w, "Displaying the")
		} else {
			fmt.Fprint(w, "No")
		}
		fmt.Fprint(w, " wins for this category. ")
		fmt.Fprintf(w,
			"You can also <a href=\"/award_category.cgi?%d+1\">view all awards and nominations</a> in this category.\n",
			catID)
	} else {
		if len(awards) == 0 {
			fmt.Fprintln(w, "No wins or nominations for this category.")
			fmt.Fprintln(w, `</div>`)
			HTMLtrailer(w)
			return
		}
		fmt.Fprint(w, "Displaying all wins and nominations for this category. ")
		fmt.Fprintf(w,
			"You can also limit the list to the <a href=\"/award_category.cgi?%d+0\">wins</a> in this category.\n",
			catID)
	}

	fmt.Fprintln(w, `<p>`)

	// ── Award table grouped by year ───────────────────────────────────────
	printAwardCatTable(w, catID, groups)

	fmt.Fprintln(w, `</div>`) // main
	HTMLtrailer(w)
}
