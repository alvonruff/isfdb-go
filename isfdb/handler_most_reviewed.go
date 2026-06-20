// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// MostReviewedTableHandler serves /most_reviewed_table.cgi —
// a menu linking to all-time, pre-1900, and per-decade/year most-reviewed title lists.
func MostReviewedTableHandler(w http.ResponseWriter, r *http.Request) {
	HTMLheader(w, "Most-Reviewed Titles")
	PrintNavbar(w, "top", "", "")

	fmt.Fprintf(w, "<h3><a href=\"%s://%s/most_reviewed.cgi?all\">Most-Reviewed Titles of All Time</a></h3>\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<h3><a href=\"%s://%s/most_reviewed.cgi?pre1900\">Most-Reviewed Titles Prior to 1900</a></h3>\n",
		PROTOCOL, HTMLHOST)

	fmt.Fprintln(w, "<h3>Most-Reviewed Titles Since 1900 by Decade and Year</h3>")
	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintln(w, "<tr><th>Decade</th><th colspan=\"10\">Years</th></tr>")

	currentYear := time.Now().Year()
	currentDecade := (currentYear / 10) * 10
	bgcolor := 0
	for decade := currentDecade; decade >= 1900; decade -= 10 {
		fmt.Fprintf(w, "<tr class=\"table%d\">\n", bgcolor+1)
		fmt.Fprintf(w, "<td><a href=\"%s://%s/most_reviewed.cgi?decade+%d\">%ds</a></td>\n",
			PROTOCOL, HTMLHOST, decade, decade)
		for year := decade; year < decade+10; year++ {
			if year > currentYear {
				fmt.Fprintln(w, "<td>&nbsp;</td>")
			} else {
				fmt.Fprintf(w, "<td><a href=\"%s://%s/most_reviewed.cgi?year+%d\">%d</a></td>\n",
					PROTOCOL, HTMLHOST, year, year)
			}
		}
		fmt.Fprintln(w, "</tr>")
		bgcolor ^= 1
	}
	fmt.Fprintln(w, "</table>")

	HTMLtrailer(w)
}

// MostReviewedHandler serves /most_reviewed.cgi?SPAN[+YEAR_OR_DECADE] —
// lists titles ranked by number of reviews for the given time span.
func MostReviewedHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 1 {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	span := params[0]
	var yearOrDecade string
	if (span == "decade" || span == "year") && len(params) >= 2 {
		yearOrDecade = params[1]
	}

	var header string
	switch span {
	case "all":
		header = "Most-Reviewed Titles of All Time"
	case "pre1900":
		header = "Most-Reviewed Titles Prior to 1900"
	case "decade":
		header = fmt.Sprintf("Most-Reviewed Titles of the %ss", yearOrDecade)
	case "year":
		header = fmt.Sprintf("Most-Reviewed Titles of %s", yearOrDecade)
	default:
		http.Error(w, "Invalid span", http.StatusBadRequest)
		return
	}

	HTMLheader(w, "Most-Reviewed Titles Details")
	PrintNavbar(w, "top", "", "")

	fmt.Fprintf(w, "<h3>%s</h3>\n", header)
	fmt.Fprintln(w, "<h3>This report is generated once a day</h3>")

	where := "WHERE 1"
	args := []any{}
	switch span {
	case "year":
		where += " AND year = ?"
		n, _ := strconv.Atoi(yearOrDecade)
		args = append(args, n)
	case "decade":
		where += " AND decade = ?"
		n, _ := strconv.Atoi(yearOrDecade)
		args = append(args, n)
	case "pre1900":
		where += " AND decade = 'pre1900'"
	}

	rows, err := DB.Query(
		"SELECT title_id, year, reviews FROM most_reviewed "+where+
			" ORDER BY reviews DESC LIMIT 500",
		args...)
	if err != nil {
		fmt.Fprintf(w, "<p>Database error: %s</p>", err)
		HTMLtrailer(w)
		return
	}
	defer rows.Close()

	type reviewRow struct {
		titleID int
		year    int
		reviews int
	}
	var results []reviewRow
	for rows.Next() {
		var rr reviewRow
		if err := rows.Scan(&rr.titleID, &rr.year, &rr.reviews); err != nil {
			continue
		}
		results = append(results, rr)
	}

	if len(results) == 0 {
		fmt.Fprintln(w, "<h3>This report is currently unavailable. It will be regenerated overnight.</h3>")
		HTMLtrailer(w)
		return
	}

	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintln(w, "<tr>")
	fmt.Fprintln(w, "<th>Count</th><th>Reviews</th>")
	if span != "year" {
		fmt.Fprintln(w, "<th>Year</th>")
	}
	fmt.Fprintln(w, "<th>Title</th><th>Type</th><th>Authors</th>")
	fmt.Fprintln(w, "</tr>")

	bgcolor := 0
	for i, rr := range results {
		title, err := SQLloadTitleData(DB, rr.titleID)
		if err != nil {
			continue
		}

		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor+1)
		fmt.Fprintf(w, "<td>%d</td>\n", i+1)
		fmt.Fprintf(w, "<td>%d</td>\n", rr.reviews)

		if span != "year" {
			displayYear := fmt.Sprintf("%d", rr.year)
			if displayYear == "0" {
				displayYear = "0000"
			}
			fmt.Fprintf(w, "<td>%s</td>\n", displayYear)
		}

		fmt.Fprintf(w, "<td><a href=\"%s://%s/title.cgi?%d\">%s</a></td>\n",
			PROTOCOL, HTMLHOST, rr.titleID, ISFDBText(title.TitleTitle.String))
		fmt.Fprintf(w, "<td>%s</td>\n", title.TitleTType.String)
		fmt.Fprintln(w, "<td>")
		printTitleAuthors(w, rr.titleID)
		fmt.Fprintln(w, "</td>")
		fmt.Fprintln(w, "</tr>")
		bgcolor ^= 1
	}
	fmt.Fprintln(w, "</table>")

	HTMLtrailer(w)
}
