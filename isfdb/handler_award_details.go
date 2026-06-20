// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// AwardDetailsHandler serves /award_details.cgi?<award_id> — a single award
// detail page.  It matches Python's award_details.py / awardClass.PrintAwardSummary.
func AwardDetailsHandler(w http.ResponseWriter, r *http.Request) {
	awardID, err := strconv.Atoi(r.URL.RawQuery)
	if err != nil || awardID <= 0 {
		http.Error(w, "Invalid award ID", http.StatusBadRequest)
		return
	}

	a, err := SQLloadAwardData(DB, awardID)
	if err != nil {
		http.Error(w, "Award not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	d, err := LoadAwardDisplay(DB, a)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	year := ""
	if len(d.AwardYear) >= 4 {
		year = d.AwardYear[:4]
	}
	pageTitle := fmt.Sprintf("%s %s: %s", d.TypeShortName, year, d.CatName)

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "award", "", "")
	fmt.Fprintln(w, `<div class="ContentBox">`)

	fmt.Fprintln(w, `<ul>`)

	// Title
	fmt.Fprint(w, `<li><b>Title: </b>`)
	if d.TitleID != 0 {
		printAwardTitle(w, d)
	} else if d.AwardTitle != "" && d.AwardTitle != "untitled" {
		fmt.Fprintf(w, "%s (<i>no ISFDB title record</i>)", ISFDBText(d.AwardTitle))
	} else {
		fmt.Fprint(w, "untitled (<i>no ISFDB title record</i>)")
	}
	fmt.Fprintln(w)

	// Author(s) — only emit the item if there is something to show; label is
	// singular/plural based on the count of authors (matching Python).
	numAuthors := 0
	if d.TitleID != 0 {
		numAuthors = len(d.Authors)
	} else {
		numAuthors = len(d.AwardAuthors)
	}
	if numAuthors > 0 {
		authorLabel := "Author"
		if numAuthors > 1 {
			authorLabel = "Authors"
		}
		fmt.Fprintf(w, "<li><b>%s: </b> ", authorLabel)
		printAwardAuthors(w, d)
		fmt.Fprintln(w)
	}

	// Award name
	fmt.Fprintf(w, "<li><b>Award Name: </b> <a href=\"/awardtype.cgi?%d\">%s</a>\n",
		d.TypeID, ISFDBText(d.TypeName))

	// Year — links to the award-year page
	fmt.Fprintf(w, "<li><b>Year: </b> <a href=\"/ay.cgi?%d+%s\">%s</a>\n",
		d.TypeID, year, year)

	// Category — links to award_category page (second param 0 = no year filter)
	fmt.Fprintf(w, "<li><b>Category: </b> <a href=\"/award_category.cgi?%d+0\">%s</a>\n",
		d.CatID, ISFDBText(d.CatName))

	// Award level — Win/Nomination, poll place, or special description
	fmt.Fprint(w, "<li><b>Award Level: </b> ")
	levelInt, _ := strconv.Atoi(d.AwardLevel)
	if levelInt > 70 {
		fmt.Fprintf(w, "<i>%s</i>", ISFDBText(specialAwards[d.AwardLevel]))
	} else if d.TypePoll == "Yes" {
		fmt.Fprintf(w, "<i>Poll Place</i>: %s", d.AwardLevel)
	} else {
		if levelInt == 1 {
			fmt.Fprint(w, " Win")
		} else {
			fmt.Fprint(w, " Nomination")
		}
	}
	fmt.Fprintln(w)

	// IMDB record (if present)
	if d.AwardMovie != "" {
		imdbLink := fmt.Sprintf(
			`<a href="https://www.imdb.com/title/%s/" target="_blank">%s</a>`,
			ISFDBText(d.AwardMovie), ISFDBText(d.AwardMovie))
		fmt.Fprintf(w, "<li><b>IMDB record: </b> %s\n", imdbLink)
	}

	// Note (if present) — 'short' mode matches Python's FormatNote call
	if d.AwardNote != "" {
		formatted := FormatNote(d.AwardNote, "Note", "short", awardID, "Award", false)
		fmt.Fprintf(w, "<li>%s\n", formatted)
	}

	fmt.Fprintln(w, `</ul>`)

	fmt.Fprintln(w, `</div>`) // ContentBox
	fmt.Fprintln(w, `</div>`) // content
	HTMLtrailer(w)
}
