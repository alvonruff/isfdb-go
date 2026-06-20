// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"net/http"
)

// AdvSearchSelectionHandler serves /adv_search_selection.cgi?<type>
// where <type> is one of: author, title, series, pub, publisher, pub_series,
// award_type, award_cat, award.
func AdvSearchSelectionHandler(w http.ResponseWriter, r *http.Request) {
	// The type is passed as the raw query string: ?author, ?title, etc.
	urlKey := r.URL.RawQuery

	st := advSearchTypeByURLKey(urlKey)
	if st == nil {
		http.Error(w, "Unknown search type: "+urlKey, http.StatusBadRequest)
		return
	}

	pageTitle := "Advanced " + st.TypeName + " Search"
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "adv_search_selection", "", "")

	// Header notes.
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintf(w, "<li>A downloadable version of the ISFDB database is available "+
		"<a href=\"%s://%s/index.php/ISFDB_Downloads\">here</a>\n",
		PROTOCOL, WIKILOC)
	fmt.Fprintln(w, "<li>Supported wildcards: * and % match any number of characters, _ matches one character")
	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, "<hr>")

	// Search form.
	fmt.Fprintln(w, "<h2>Selection Criteria</h2>")
	fmt.Fprintf(w, "<form method=\"GET\" action=\"%s://%s/adv_search_results.cgi\">\n",
		PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, "<p>")

	// Per-type messages.
	if len(st.Messages) > 0 {
		fmt.Fprintln(w, "<ul>")
		for _, msg := range st.Messages {
			fmt.Fprintf(w, "<li>%s\n", msg)
		}
		fmt.Fprintln(w, "</ul>")
	}

	// Term rows.
	for n := 1; n <= advMaxTerms; n++ {
		fmt.Fprintf(w, "<p id=\"%s_selectors_%d\">\n", urlKey, n)

		// Field selector.
		fmt.Fprintf(w, "<select name=\"USE_%d\" id=\"%s_%d\">\n", n, urlKey, n)
		for _, f := range st.Fields {
			fmt.Fprintf(w, "<option value=\"%s\">%s\n", f.Field, f.Label)
		}
		fmt.Fprintln(w, "</select>")

		// Operator selector.
		fmt.Fprintf(w, "<select name=\"O_%d\" id=\"%s_operator_%d\">\n", n, urlKey, n)
		for _, op := range advSearchOperators {
			fmt.Fprintf(w, "<option value=\"%s\">%s\n", op.Key, op.Label)
		}
		fmt.Fprintln(w, "</select>")

		// Term input.
		fmt.Fprintf(w, "<input id=\"%sterm_%d\" name=\"TERM_%d\" type=\"text\" size=\"50\">\n",
			urlKey, n, n)

		// AND/OR radios on first row only.
		if n == 1 {
			fmt.Fprintln(w, "<input type=\"radio\" name=\"C\" value=\"AND\" checked>AND")
			fmt.Fprintln(w, "<input type=\"radio\" name=\"C\" value=\"OR\">OR")
		}
		fmt.Fprintln(w, "<p>")
	}

	// Sort-by row.
	printAdvSortBy(w, st)

	// Hidden fields and submit buttons.
	fmt.Fprintln(w, "<button type=\"submit\" name=\"ACTION\" value=\"query\">Get Results</button>")
	fmt.Fprintln(w, "<button type=\"submit\" name=\"ACTION\" value=\"count\">Get Count</button>")
	fmt.Fprintln(w, "<input name=\"START\" value=\"0\" type=\"hidden\">")
	fmt.Fprintf(w, "<input name=\"TYPE\" value=\"%s\" type=\"hidden\">\n", st.TypeName)
	fmt.Fprintln(w, "</form>")

	HTMLtrailer(w)
}

// printAdvSortBy renders the sort-by row for the given search type.
// If there is only one sort option it is rendered as a hidden input.
func printAdvSortBy(w http.ResponseWriter, st *advSearchType) {
	if len(st.SortBy) == 0 {
		return
	}
	if len(st.SortBy) == 1 {
		fmt.Fprintf(w, "<input name=\"ORDERBY\" value=\"%s\" type=\"hidden\">\n", st.SortBy[0].Field)
		return
	}
	fmt.Fprintln(w, "<b>Sort Results By:</b>")
	fmt.Fprintln(w, "<select name=\"ORDERBY\">")
	for _, s := range st.SortBy {
		fmt.Fprintf(w, "<option value=\"%s\">%s\n", s.Field, s.Label)
	}
	fmt.Fprintln(w, "</select>")
}
