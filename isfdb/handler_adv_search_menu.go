// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/http"
)

// AdvSearchMenuHandler serves /adv_search_menu.cgi — the Advanced Search
// landing page.  User authentication is not yet implemented, so the full
// search menu is always shown.
func AdvSearchMenuHandler(w http.ResponseWriter, r *http.Request) {
	HTMLheader(w, "Advanced Search")
	PrintNavbar(w, "adv_search_menu", "", "")

	fmt.Fprintln(w, "<ul>")
	fmt.Fprintf(w, "<li>A downloadable version of the ISFDB database is available "+
		"<a href=\"%s://%s/index.php/ISFDB_Downloads\">here</a>\n",
		PROTOCOL, WIKILOC)
	fmt.Fprintln(w, "</ul>")

	fmt.Fprintln(w, "<hr>")
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintln(w, "<li><b>Custom Searches of Individual Record Types:</b>")
	fmt.Fprintln(w, "<ul>")

	searchTypes := []struct{ param, label string }{
		{"author", "Authors"},
		{"title", "Titles"},
		{"series", "Series"},
		{"pub", "Publications"},
		{"publisher", "Publishers"},
		{"pub_series", "Publication Series"},
		{"award_type", "Award Types"},
		{"award_cat", "Award Categories"},
		{"award", "Awards"},
	}
	for _, st := range searchTypes {
		fmt.Fprintf(w, "<li><a href=\"%s://%s/adv_search_selection.cgi?%s\">%s</a>\n",
			PROTOCOL, HTMLHOST, st.param, st.label)
	}

	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, "<li><b>Other Searches:</b>")
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintf(w, "<li><a href=\"%s://%s/adv_identifier_search.cgi\">Publication Search by External Identifier</a>\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/adv_notes_search.cgi\">Notes Search</a>\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/adv_web_page_search.cgi\">Web Page Search</a>\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, "</ul>")

	fmt.Fprintln(w, "<p><hr><p>")

	// Google search form.
	fmt.Fprintln(w, "<h2>Search the ISFDB database using Google:</h2>")
	fmt.Fprintf(w, "<form method=\"GET\" action=\"%s://%s/google_search_redirect.cgi\" accept-charset=\"utf-8\">\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, "<p>")

	fmt.Fprintln(w, "<select name=\"PAGE_TYPE\">")
	googleTypes := []struct{ val, label string }{
		{"name", "Name"},
		{"title", "Title"},
		{"series", "Series"},
		{"publication", "Publication"},
		{"pubseries", "Publication Series"},
		{"publisher", "Publisher"},
		{"award_category", "Award Category"},
	}
	for _, gt := range googleTypes {
		fmt.Fprintf(w, "<option value=\"%s\">%s\n", gt.val, gt.label)
	}
	fmt.Fprintln(w, "</select>")

	fmt.Fprintln(w, "<select name=\"OPERATOR\">")
	fmt.Fprintln(w, "<option value=\"exact\">contains exact word")
	fmt.Fprintln(w, "<option selected value=\"approximate\">contains approximate word")
	fmt.Fprintln(w, "</select>")

	fmt.Fprintln(w, "<input name=\"SEARCH_VALUE\" size=\"50\">")
	fmt.Fprintln(w, "<p>")
	fmt.Fprintln(w, "<input type=\"submit\" value=\"Submit Query\">")
	fmt.Fprintln(w, "</form>")

	HTMLtrailer(w)
}
