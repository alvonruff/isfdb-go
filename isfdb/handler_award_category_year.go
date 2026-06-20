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

// AwardCategoryYearHandler serves /award_category_year.cgi?<cat_id>+<year>.
// It shows all awards for one category in one year, preceded by a metadata
// header for the category.  Matches Python's award_category_year.py /
// awardcatClass.PrintAwardCatYear + PrintAwardCatPageHeader.
func AwardCategoryYearHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) != 2 {
		http.Error(w, "This page requires two parameters", http.StatusBadRequest)
		return
	}
	catID, err := strconv.Atoi(params[0])
	year, err2 := strconv.Atoi(params[1])
	if err != nil || err2 != nil || catID <= 0 || year <= 0 {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
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

	// Load optional category note and webpages
	catNote := ""
	if cat.AwardCatNoteID.Valid {
		if n, err := SQLgetNotes(DB, int(cat.AwardCatNoteID.Int32)); err == nil {
			catNote = n
		}
	}
	catWebpages, _ := SQLloadAwardCatWebpages(DB, catID)

	// Load awards for this category + year
	awards, err := SQLloadAwardsForCatYear(DB, catID, year)
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

	pageTitle := fmt.Sprintf("Award Category: %d %s (%s)",
		year, cat.AwardCatName.String, at.AwardTypeName.String)

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "award_cat", "", "")

	// ── Category metadata header ──────────────────────────────────────────
	printAwardCatPageHeader(w, cat, at, catNote, catWebpages)

	// ── Body text ─────────────────────────────────────────────────────────
	fmt.Fprintf(w, "Displaying awards and nominations for this category for %d.\n", year)
	fmt.Fprintf(w,
		"You can also <a href=\"/award_category.cgi?%d+1\">view all awards and nominations</a> for this category for all years.\n",
		catID)

	// ── Award table for the single year ──────────────────────────────────
	printAwardCatTable(w, catID, []AwardCatYearGroup{
		{Year: strconv.Itoa(year), Displays: displays},
	})

	fmt.Fprintln(w, `</div>`) // main
	HTMLtrailer(w)
}

// printAwardCatPageHeader renders the <ul> metadata block for an award
// category, matching Python's awardcatClass.PrintAwardCatPageHeader.
func printAwardCatPageHeader(w io.Writer, cat *AwardCat, at *AwardType,
	note string, webpages []string) {

	fmt.Fprintln(w, `<ul>`)
	fmt.Fprintf(w, "<li><b>Award Category: </b> %s\n", ISFDBText(cat.AwardCatName.String))
	fmt.Fprintf(w, "<li><b>Award Type: </b> <a href=\"/awardtype.cgi?%d\">%s</a>\n",
		at.AwardTypeID, ISFDBText(at.AwardTypeName.String))

	if cat.AwardCatOrder.Valid {
		fmt.Fprintf(w, "<li><b>Display Order: </b> %d\n", cat.AwardCatOrder.Int32)
	}

	if len(webpages) > 0 {
		domains, _ := SQLLoadRecognizedDomains(DB)
		PrintWebPages(w, webpages, "<li>", domains)
	}

	if note != "" {
		formatted := FormatNote(note, "Note", "short", cat.AwardCatID, "AwardCat", false)
		fmt.Fprintf(w, "<li>\n%s\n", formatted)
	}
	fmt.Fprintln(w, `</ul>`)
}

// AwardCatYearGroup pairs a 4-digit year string with its resolved award displays.
type AwardCatYearGroup struct {
	Year     string // "YYYY"
	Displays []*AwardDisplay
}

// printAwardCatTable renders the award table, one section per year group,
// matching Python's PrintAwardCatTable.  Each year heading links to
// award_category_year.cgi.
func printAwardCatTable(w io.Writer, catID int, groups []AwardCatYearGroup) {
	fmt.Fprintln(w, `<table>`)
	for _, g := range groups {
		fmt.Fprintln(w, `<tr><td colspan=3> </td></tr>`)
		fmt.Fprintf(w,
			"<tr><th colspan=3><a href=\"/award_category_year.cgi?%d+%s\">%s</a></th></tr>\n",
			catID, g.Year, g.Year)
		printOneCategoryAwards(w, g.Displays)
	}
	fmt.Fprintln(w, `</table>`)
}
