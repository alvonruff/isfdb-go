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

// popularTitleTypes maps type_id (0–6) to display label and SQL title_type value.
var popularTitleTypes = []struct {
	label     string
	titleType string // empty = all; "OTHER" = not in main 5
}{
	{label: "Titles", titleType: ""},
	{label: "Novels", titleType: "NOVEL"},
	{label: "Short Fiction", titleType: "SHORTFICTION"},
	{label: "Collections", titleType: "COLLECTION"},
	{label: "Anthologies", titleType: "ANTHOLOGY"},
	{label: "Non-Fiction", titleType: "NONFICTION"},
	{label: "Other Title Types", titleType: "OTHER"},
}

// MostPopularTableHandler serves /most_popular_table.cgi?TYPE —
// a menu linking to all-time, pre-1950, and per-decade/year ranked title lists.
func MostPopularTableHandler(w http.ResponseWriter, r *http.Request) {
	typeID, err := strconv.Atoi(r.URL.RawQuery)
	if err != nil || typeID < 0 || typeID > 6 {
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}
	label := popularTitleTypes[typeID].label

	HTMLheader(w, label+" Ranked by Awards and Nominations")
	PrintNavbar(w, "top", "", "")

	fmt.Fprintf(w, "<h3><a href=\"%s://%s/most_popular.cgi?%d+all\">Highest Ranked %s of All Time</a></h3>\n",
		PROTOCOL, HTMLHOST, typeID, label)
	fmt.Fprintf(w, "<h3><a href=\"%s://%s/most_popular.cgi?%d+pre1950\">Highest Ranked %s Prior to 1950</a></h3>\n",
		PROTOCOL, HTMLHOST, typeID, label)

	fmt.Fprintf(w, "<h3>Highest Ranked %s Since 1950 by Decade and Year</h3>\n", label)
	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintln(w, "<tr><th>Decade</th><th colspan=\"10\">Years</th></tr>")

	currentYear := time.Now().Year()
	currentDecade := (currentYear / 10) * 10
	bgcolor := 0
	for decade := currentDecade; decade >= 1950; decade -= 10 {
		fmt.Fprintf(w, "<tr class=\"table%d\">\n", bgcolor+1)
		fmt.Fprintf(w, "<td><a href=\"%s://%s/most_popular.cgi?%d+decade+%d\">%ds</a></td>\n",
			PROTOCOL, HTMLHOST, typeID, decade, decade)
		for year := decade; year < decade+10; year++ {
			if year > currentYear {
				fmt.Fprintln(w, "<td>&nbsp;</td>")
			} else {
				fmt.Fprintf(w, "<td><a href=\"%s://%s/most_popular.cgi?%d+year+%d\">%d</a></td>\n",
					PROTOCOL, HTMLHOST, typeID, year, year)
			}
		}
		fmt.Fprintln(w, "</tr>")
		bgcolor ^= 1
	}
	fmt.Fprintln(w, "</table>")

	HTMLtrailer(w)
}

// MostPopularHandler serves /most_popular.cgi?TYPE+SPAN[+YEAR_OR_DECADE] —
// lists titles ranked by award score for the given type and time span.
func MostPopularHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 2 {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	typeID, err := strconv.Atoi(params[0])
	if err != nil || typeID < 0 || typeID > 6 {
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}
	span := params[1]

	var yearOrDecade string
	if (span == "decade" || span == "year") && len(params) >= 3 {
		yearOrDecade = params[2]
	}

	typeInfo := popularTitleTypes[typeID]
	var header string
	switch span {
	case "all":
		header = "Highest Ranked " + typeInfo.label + " of All Time"
	case "pre1950":
		header = "Highest Ranked " + typeInfo.label + " Prior to 1950"
	case "decade":
		header = fmt.Sprintf("Highest Ranked %s of the %ss", typeInfo.label, yearOrDecade)
	case "year":
		header = fmt.Sprintf("Highest Ranked %s published in %s", typeInfo.label, yearOrDecade)
	default:
		http.Error(w, "Invalid span", http.StatusBadRequest)
		return
	}

	HTMLheader(w, header)
	PrintNavbar(w, "top", "", "")

	fmt.Fprintln(w, "<h3>This report is generated once a day</h3>")

	where := "WHERE 1"
	args := []any{}
	switch span {
	case "year":
		where += " AND year = ?"
		args = append(args, yearOrDecade)
	case "decade":
		where += " AND decade = ?"
		args = append(args, yearOrDecade)
	case "pre1950":
		where += " AND decade = 'pre1950'"
	}
	switch typeInfo.titleType {
	case "":
		// no filter
	case "OTHER":
		where += " AND title_type NOT IN ('NOVEL','SHORTFICTION','COLLECTION','ANTHOLOGY','NONFICTION')"
	default:
		where += " AND title_type = ?"
		args = append(args, typeInfo.titleType)
	}

	rows, err := DB.Query(
		"SELECT title_id, score, year, title_type FROM award_titles_report "+where+
			" ORDER BY score DESC, year DESC LIMIT 500",
		args...)
	if err != nil {
		fmt.Fprintf(w, "<p>Database error: %s</p>", err)
		HTMLtrailer(w)
		return
	}
	defer rows.Close()

	type titleRow struct {
		titleID   int
		score     int
		year      int
		titleType string
	}
	var results []titleRow
	for rows.Next() {
		var tr titleRow
		if err := rows.Scan(&tr.titleID, &tr.score, &tr.year, &tr.titleType); err != nil {
			continue
		}
		results = append(results, tr)
	}

	if len(results) == 0 {
		fmt.Fprintln(w, "<h3>No awards or nominations for the specified period</h3>")
		HTMLtrailer(w)
		return
	}

	fmt.Fprintln(w, "<b>Note</b>: Some recent awards are yet to be integrated into the database.<br>")
	fmt.Fprintln(w, "<b>Scoring</b>: Wins are worth 50 points, nominations and second places are worth 35 points. For polls, third and lower places are worth (33-poll position) points.")

	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintln(w, "<tr>")
	fmt.Fprintln(w, "<th>Place</th><th>Score</th>")
	if span != "year" {
		fmt.Fprintln(w, "<th>Year</th>")
	}
	fmt.Fprintln(w, "<th>Title</th><th>Type</th><th>Authors</th>")
	fmt.Fprintln(w, "</tr>")

	bgcolor := 0
	for i, tr := range results {
		title, err := SQLloadTitleData(DB, tr.titleID)
		if err != nil {
			continue
		}

		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor+1)
		fmt.Fprintf(w, "<td>%d</td>\n", i+1)
		fmt.Fprintf(w, "<td>%d</td>\n", tr.score)

		if span != "year" {
			displayYear := fmt.Sprintf("%d", tr.year)
			if displayYear == "0" {
				displayYear = "0000"
			}
			fmt.Fprintf(w, "<td>%s</td>\n", displayYear)
		}

		fmt.Fprintf(w, "<td><a href=\"%s://%s/title.cgi?%d\">%s</a></td>\n",
			PROTOCOL, HTMLHOST, tr.titleID, ISFDBText(title.TitleTitle.String))
		fmt.Fprintf(w, "<td>%s</td>\n", tr.titleType)

		fmt.Fprintln(w, "<td>")
		printTitleAuthors(w, tr.titleID)
		fmt.Fprintln(w, "</td>")

		fmt.Fprintln(w, "</tr>")
		bgcolor ^= 1
	}
	fmt.Fprintln(w, "</table>")

	HTMLtrailer(w)
}

// printTitleAuthors writes comma-separated author links for a title.
func printTitleAuthors(w http.ResponseWriter, titleID int) {
	rows, err := DB.Query(`
		SELECT ca.author_id, a.author_canonical
		FROM canonical_author ca
		JOIN authors a ON ca.author_id = a.author_id
		WHERE ca.title_id = ? AND ca.ca_status = 1
		ORDER BY ca.ca_id`, titleID)
	if err != nil {
		return
	}
	defer rows.Close()

	first := true
	for rows.Next() {
		var authorID int
		var name string
		if err := rows.Scan(&authorID, &name); err != nil {
			continue
		}
		if !first {
			fmt.Fprint(w, ", ")
		}
		fmt.Fprintf(w, "<a href=\"%s://%s/author.cgi?%d\">%s</a>",
			PROTOCOL, HTMLHOST, authorID, ISFDBText(name))
		first = false
	}
}
