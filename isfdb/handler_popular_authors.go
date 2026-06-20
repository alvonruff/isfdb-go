// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
)

// authorTypeLabel maps type_id (0–6) to display label and SQL title_type value.
// An empty title_type means no type filter; "OTHER" is a sentinel handled specially.
var authorTypeLabel = []struct {
	label     string
	titleType string // empty = all; "OTHER" = not in the main 5
}{
	{label: "Authors and Editors", titleType: ""},
	{label: "Novel Authors", titleType: "NOVEL"},
	{label: "Short Fiction Authors", titleType: "SHORTFICTION"},
	{label: "Collection Authors", titleType: "COLLECTION"},
	{label: "Anthology Editors", titleType: "ANTHOLOGY"},
	{label: "Non-Fiction Authors", titleType: "NONFICTION"},
	{label: "Other Title Types Authors", titleType: "OTHER"},
}

// PopularAuthorsTableHandler serves /popular_authors_table.cgi?TYPE —
// a menu of links to all-time, pre-1950, and per-decade author rankings.
func PopularAuthorsTableHandler(w http.ResponseWriter, r *http.Request) {
	typeID, err := strconv.Atoi(r.URL.RawQuery)
	if err != nil || typeID < 0 || typeID > 6 {
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}
	label := authorTypeLabel[typeID].label

	HTMLheader(w, label+" Ranked by Awards and Nominations")
	PrintNavbar(w, "top", "", "")

	fmt.Fprintf(w, "<h3><a href=\"%s://%s/popular_authors.cgi?%d+all\">Highest Ranked %s of All Time</a></h3>\n",
		PROTOCOL, HTMLHOST, typeID, label)
	fmt.Fprintf(w, "<h3><a href=\"%s://%s/popular_authors.cgi?%d+pre1950\">Highest Ranked %s Prior to 1950</a></h3>\n",
		PROTOCOL, HTMLHOST, typeID, label)

	fmt.Fprintf(w, "<h3>Highest Ranked %s Since 1950 by Decade:</h3>\n", label)
	fmt.Fprintln(w, "<ul>")
	currentDecade := (time.Now().Year() / 10) * 10
	for decade := 1950; decade <= currentDecade; decade += 10 {
		fmt.Fprintf(w, "<li><a href=\"%s://%s/popular_authors.cgi?%d+decade+%d\">%ds</a>\n",
			PROTOCOL, HTMLHOST, typeID, decade, decade)
	}
	fmt.Fprintln(w, "</ul>")

	HTMLtrailer(w)
}

// PopularAuthorsHandler serves /popular_authors.cgi?TYPE+SPAN[+DECADE] —
// ranks authors by their award score for the given type and time span.
func PopularAuthorsHandler(w http.ResponseWriter, r *http.Request) {
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

	var decade string
	if span == "decade" {
		if len(params) < 3 {
			http.Error(w, "Missing decade", http.StatusBadRequest)
			return
		}
		decade = params[2]
	}

	typeInfo := authorTypeLabel[typeID]
	var header string
	switch span {
	case "all":
		header = "Highest Ranked " + typeInfo.label + " of All Time"
	case "pre1950":
		header = "Highest Ranked " + typeInfo.label + " Prior to 1950"
	case "decade":
		header = fmt.Sprintf("Highest Ranked %s of the %ss", typeInfo.label, decade)
	default:
		http.Error(w, "Invalid span", http.StatusBadRequest)
		return
	}

	HTMLheader(w, header)
	PrintNavbar(w, "top", "", "")

	// Build query against award_titles_report.
	where := "WHERE 1"
	args := []any{}
	if span == "decade" {
		where += " AND decade = ?"
		args = append(args, decade)
	} else if span == "pre1950" {
		where += " AND decade = 'pre1950'"
	}
	switch typeInfo.titleType {
	case "":
		// no filter for type 0
	case "OTHER":
		where += " AND title_type NOT IN ('NOVEL','SHORTFICTION','COLLECTION','ANTHOLOGY','NONFICTION')"
	default:
		where += " AND title_type = ?"
		args = append(args, typeInfo.titleType)
	}

	// Single JOIN query: aggregate scores per author directly.
	query := `SELECT ca.author_id, SUM(atr.score)
		FROM award_titles_report atr
		JOIN canonical_author ca ON atr.title_id = ca.title_id AND ca.ca_status = 1
		` + where + `
		GROUP BY ca.author_id`
	rows, err := DB.Query(query, args...)
	if err != nil {
		fmt.Fprintf(w, "<p>Database error: %s</p>", err)
		HTMLtrailer(w)
		return
	}
	defer rows.Close()

	authorScores := map[int]int{}
	for rows.Next() {
		var authorID, score int
		if err := rows.Scan(&authorID, &score); err != nil {
			continue
		}
		authorScores[authorID] += score
	}

	// Sort authors by score descending.
	type authorScore struct {
		authorID int
		score    int
	}
	ranked := make([]authorScore, 0, len(authorScores))
	for id, sc := range authorScores {
		ranked = append(ranked, authorScore{id, sc})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
	if len(ranked) > 500 {
		ranked = ranked[:500]
	}

	fmt.Fprintln(w, "<h3>This report is generated once a day</h3>")
	fmt.Fprintln(w, "<b>Note</b>: Some recent awards are yet to be integrated into the database. Only title-based awards are used for ranking purposes.<br>")
	fmt.Fprintln(w, "<b>Scoring</b>: Wins are worth 50 points, nominations and second places are worth 35 points. For polls, third and lower places are worth (33-poll position) points.")

	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintln(w, "<tr>")
	fmt.Fprintln(w, "<th>Place</th><th>Score</th>")
	if typeID == 4 {
		fmt.Fprintln(w, "<th>Editor</th>")
	} else {
		fmt.Fprintln(w, "<th>Author</th>")
	}
	fmt.Fprintln(w, "</tr>")

	bgcolor := 0
	for i, as := range ranked {
		author, err := SQLloadAuthorData(DB, as.authorID)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor+1)
		fmt.Fprintf(w, "<td>%d</td>\n", i+1)
		fmt.Fprintf(w, "<td>%d</td>\n", as.score)
		fmt.Fprintf(w, "<td><a href=\"%s://%s/author.cgi?%d\">%s</a></td>\n",
			PROTOCOL, HTMLHOST, author.AuthorID, ISFDBText(author.AuthorCanonical.String))
		fmt.Fprintln(w, "</tr>")
		bgcolor ^= 1
	}
	fmt.Fprintln(w, "</table>")

	HTMLtrailer(w)
}
