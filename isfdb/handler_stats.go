// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/http"
)

// StatsHandler serves /stats-and-tops.cgi — the Statistics and Top Lists menu.
func StatsHandler(w http.ResponseWriter, r *http.Request) {
	HTMLheader(w, "Statistics and Top Lists")
	PrintNavbar(w, "stats", "", "")

	fmt.Fprintln(w, "<h4>Author Statistics</h4>")
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintf(w, "<li><a href=\"%s://%s/authors_by_debut_year_table.cgi\">Authors By Debut Year</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, "<li>Authors by Age:")
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintf(w, "<li><a href=\"%s://%s/stats.cgi?16\">Oldest Living Authors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/stats.cgi?17\">Oldest Non-Living Authors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/stats.cgi?18\">Youngest Living Authors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/stats.cgi?19\">Youngest Non-Living Authors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, "<li>Authors/Editors Ranked by Awards and Nominations:")
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintf(w, "<li><a href=\"%s://%s/popular_authors_table.cgi?0\">All Authors and Editors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/popular_authors_table.cgi?1\">Novel Authors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/popular_authors_table.cgi?2\">Short Fiction Authors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/popular_authors_table.cgi?3\">Collection Authors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/popular_authors_table.cgi?4\">Anthology Editors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/popular_authors_table.cgi?5\">Non-Fiction Authors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/popular_authors_table.cgi?6\">Other Types Authors and Editors</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, "</ul>")

	fmt.Fprintln(w, "<h4>Title Statistics</h4>")
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintf(w, "<li><a href=\"%s://%s/stats.cgi?5\">Titles by Year of First Publication</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/stats.cgi?7\">Titles by Author Age</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/stats.cgi?8\">Percent of Titles in Series by Year</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/most_reviewed_table.cgi\">Most-Reviewed Titles (in genre publications)</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, "<li>Titles Ranked by Awards and Nominations:")
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintf(w, "<li><a href=\"%s://%s/most_popular_table.cgi?0\">All Titles</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/most_popular_table.cgi?1\">Novels</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/most_popular_table.cgi?2\">Short Fiction</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/most_popular_table.cgi?3\">Collections</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/most_popular_table.cgi?4\">Anthologies</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/most_popular_table.cgi?5\">Non-Fiction</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/most_popular_table.cgi?6\">Other Types</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, "</ul>")

	HTMLtrailer(w)
}
