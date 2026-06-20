// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
)

// searchTypeOptions are the entries in the search-type dropdown, in order.
var searchTypeOptions = []string{
	"Name",
	"Fiction Titles",
	"All Titles",
	"Year of Title",
	"Month of Title",
	"Month of Publication",
	"Series",
	"Publication Series",
	"Magazine",
	"Publisher",
	"ISBN",
	"Award",
}

// contentDivPageTypes lists the page types that open <div id="content">;
// all other page types open <div id="main">.
var contentDivPageTypes = map[string]bool{
	"author":      true,
	"publication": true,
	"title":       true,
	"publisher":   true,
	"pub_series":  true,
	"series":      true,
	"seriestags":  true,
	"seriesgrid":  true,
}

// PrintNavbar writes the <div id="nav"> sidebar and then opens the appropriate
// content div for the page type. It must be called immediately after HTMLheader.
//
// pageType controls which content div is opened after the nav:
//   - "author", "publication", "title", "publisher", "pub_series",
//     "series", "seriestags", "seriesgrid" → <div id="content">
//   - all other page types                 → <div id="main">
//
// searchValue and searchType pre-fill the search box (pass "" for both on
// pages that are not search results).
func PrintNavbar(w io.Writer, pageType, searchValue, searchType string) {
	fmt.Fprintln(w, `<div id="nav">`)

	printNavSearchBox(w, pageType, searchValue, searchType)
	printNavOtherPages(w, pageType)
	printNavHistory(w)
	printNavLicense(w)

	fmt.Fprintln(w, `</div>`) // end nav

	// Open the content div appropriate for this page type.
	if contentDivPageTypes[pageType] {
		fmt.Fprintln(w, `<div id="content">`)
	} else {
		fmt.Fprintln(w, `<div id="main">`)
	}
}

// printNavSearchBox renders the logo and (on non-frontpage) the search form.
func printNavSearchBox(w io.Writer, pageType, searchValue, searchType string) {
	fmt.Fprintln(w, `<div id="search">`)

	// Logo — always shown; larger variant on the front page.
	fmt.Fprintf(w, "<a href=\"%s://%s/index.cgi\">\n", PROTOCOL, HTMLHOST)
	if pageType == "frontpage" {
		fmt.Fprintf(w, "<img src=\"%s://%s/isfdb_logo.jpg\" width=\"129\" height=\"85\" alt=\"ISFDB logo\">\n",
			PROTOCOL, HTMLHOST)
	} else {
		fmt.Fprintf(w, "<img src=\"%s://%s/isfdb.gif\" width=\"130\" height=\"77\" alt=\"ISFDB logo\">\n",
			PROTOCOL, HTMLHOST)
	}
	fmt.Fprintln(w, `</a>`)

	// Search form — suppressed on the front page (which has its own search bar).
	if pageType != "frontpage" {
		fmt.Fprintf(w, "<form method=\"get\" action=\"%s://%s/se.cgi\" name=\"searchform\" id=\"searchform\">\n",
			PROTOCOL, HTMLHOST)
		fmt.Fprintln(w, `<p>`)
		fmt.Fprintf(w, "<input name=\"arg\" id=\"searchform_arg\" class=\"search\" value=\"%s\">\n",
			ISFDBText(searchValue))
		fmt.Fprintln(w, `<select name="type" class="search">`)
		for _, opt := range searchTypeOptions {
			selected := ""
			if opt == searchType {
				selected = ` selected="selected"`
			}
			fmt.Fprintf(w, "<option%s>%s</option>\n", selected, opt)
		}
		fmt.Fprintln(w, `</select>`)
		fmt.Fprintln(w, `<input value="Go" type="submit" >`)
		fmt.Fprintln(w, `</form>`)

		fmt.Fprintln(w, `<p class="bottomlinks">`)
		fmt.Fprintf(w, "<a href=\"%s://%s/adv_search_menu.cgi\" class=\"inverted\">Advanced Search</a>\n",
			PROTOCOL, HTMLHOST)
	}

	fmt.Fprintln(w, `</div>`) // end search
}

// printNavOtherPages renders the "Other Pages" nav section.
func printNavOtherPages(w io.Writer, pageType string) {
	fmt.Fprintln(w, `<div class="divider">`)
	fmt.Fprintln(w, `Other Pages:`)
	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `<ul class="navbar">`)

	if pageType != "frontpage" {
		fmt.Fprintf(w, "<li><a href=\"%s://%s/index.cgi\">Home Page</a>\n", PROTOCOL, HTMLHOST)
	}
	fmt.Fprintf(w, "<li><a href=\"%s://%s/calendar_menu.cgi\">SF Calendar</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/directory.cgi?author\">Author Directory</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/award_directory.cgi\">Award Directory</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/directory.cgi?publisher\">Publisher Directory</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/directory.cgi?magazine\">Magazine Directory</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/stats-and-tops.cgi\">Statistics/Top Lists</a>\n", PROTOCOL, HTMLHOST)
	fmt.Fprintf(w, "<li><a href=\"%s://%s/update.cgi\">Database Update</a>\n", PROTOCOL, HTMLHOST)

	fmt.Fprintln(w, `</ul>`)
}

// printNavHistory renders the recent-page history section of the nav bar.
func printNavHistory(w io.Writer) {
	entries := GetHistory()
	if len(entries) == 0 {
		return
	}
	fmt.Fprintln(w, `<div class="divider">`)
	fmt.Fprintln(w, `History:`)
	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `<ul class="navbar">`)
	for _, e := range entries {
		fmt.Fprintf(w, "<li><a href=\"%s\">%s: %s</a>\n", e.URL, e.Label, ISFDBText(e.Name))
	}
	fmt.Fprintln(w, `</ul>`)
}

// printNavLicense renders the Creative Commons license block at the bottom of
// the nav bar.
func printNavLicense(w io.Writer) {
	fmt.Fprintln(w, `<div class="divider">`)
	fmt.Fprintln(w, `License:`)
	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `<div id="cclicense">`)
	fmt.Fprintln(w, `<a rel="license" href="https://creativecommons.org/licenses/by/4.0/">`)
	fmt.Fprintln(w, `<img alt="Creative Commons License" src="https://i.creativecommons.org/l/by/4.0/88x31.png"></a><br>`)
	fmt.Fprintln(w, `The data presented here is from the <a href="https://www.isfdb.org">ISFDB</a>, licensed under a <a rel="license" href="https://creativecommons.org/licenses/by/4.0/">Creative Commons Attribution 4.0 International License</a>.`)
	fmt.Fprintln(w, `</div>`)
}
