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

// AuthorsByDebutYearTableHandler serves /authors_by_debut_year_table.cgi —
// a decade grid linking to per-year author lists.
func AuthorsByDebutYearTableHandler(w http.ResponseWriter, r *http.Request) {
	HTMLheader(w, "Authors By Debut Year")
	PrintNavbar(w, "authors_by_debut_year", "", "")

	fmt.Fprintf(w, "<a href=\"%s://%s/authors_by_debut_year.cgi?0\">Prior to 1900</a>\n", PROTOCOL, HTMLHOST)

	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintln(w, `<tr><th colspan="10">Years</th></tr>`)

	currentYear := time.Now().Year()
	currentDecade := (currentYear / 10) * 10
	bgcolor := 0
	for decade := currentDecade; decade >= 1900; decade -= 10 {
		fmt.Fprintf(w, "<tr class=\"table%d\">\n", bgcolor+1)
		for year := decade; year < decade+10; year++ {
			if year > currentYear {
				fmt.Fprintln(w, "<td>&nbsp;</td>")
			} else {
				fmt.Fprintf(w, "<td><a href=\"%s://%s/authors_by_debut_year.cgi?%d\">%d</a></td>\n",
					PROTOCOL, HTMLHOST, year, year)
			}
		}
		fmt.Fprintln(w, "</tr>")
		bgcolor ^= 1
	}
	fmt.Fprintln(w, "</table>")

	HTMLtrailer(w)
}

// AuthorsByDebutYearHandler serves /authors_by_debut_year.cgi?YEAR —
// lists authors who debuted in the given year (0 = prior to 1900).
// Only authors with at least 6 qualifying titles appear (pre-filtered in the DB table).
func AuthorsByDebutYearHandler(w http.ResponseWriter, r *http.Request) {
	year, _ := strconv.Atoi(r.URL.RawQuery)

	title := "Authors By Debut Year"
	if year > 1899 {
		title += fmt.Sprintf(" - %d", year)
	} else {
		title += " - Prior to 1900"
		year = 0
	}

	HTMLheader(w, title)
	PrintNavbar(w, "authors_by_debut_year", "", "")

	fmt.Fprintln(w, "<h3>Includes authors with at least 6 novels, short fiction, poems or collections:</h3>")
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align=left class="table1">`)
	fmt.Fprintln(w, "<th>Debut Year</th><th>Author</th><th>Number of Titles</th>")
	fmt.Fprintln(w, "</tr>")

	var query string
	var args []any
	if year > 0 {
		query = `SELECT ad.debut_year, ad.author_id, a.author_canonical, ad.title_count
			FROM authors_by_debut_date ad
			JOIN authors a ON ad.author_id = a.author_id
			WHERE ad.debut_year = ?
			ORDER BY ad.debut_year, a.author_lastname, a.author_canonical`
		args = []any{year}
	} else {
		query = `SELECT ad.debut_year, ad.author_id, a.author_canonical, ad.title_count
			FROM authors_by_debut_date ad
			JOIN authors a ON ad.author_id = a.author_id
			WHERE ad.debut_year < 1900
			ORDER BY ad.debut_year, a.author_lastname, a.author_canonical`
	}

	rows, err := DB.Query(query, args...)
	if err != nil {
		fmt.Fprintf(w, "<p>Database error: %s</p>", err)
		HTMLtrailer(w)
		return
	}
	defer rows.Close()

	bgcolor := 0
	for rows.Next() {
		var debutYear, authorID, titleCount int
		var authorName string
		if err := rows.Scan(&debutYear, &authorID, &authorName, &titleCount); err != nil {
			continue
		}
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor+1)
		fmt.Fprintf(w, "<td>%d</td>\n", debutYear)
		fmt.Fprintf(w, "<td><a href=\"%s://%s/author.cgi?%d\">%s</a></td>\n",
			PROTOCOL, HTMLHOST, authorID, ISFDBText(authorName))
		fmt.Fprintf(w, "<td>%d</td>\n", titleCount)
		fmt.Fprintln(w, "</tr>")
		bgcolor ^= 1
	}

	fmt.Fprintln(w, "</table><p>")
	HTMLtrailer(w)
}
