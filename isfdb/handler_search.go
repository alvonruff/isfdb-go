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
	"strings"
	"time"
)

const searchLimit = 300

// SearchHandler serves /se.cgi — the main search page.
// Parameters come in as GET: arg=<search term>&type=<search type>.
func SearchHandler(w http.ResponseWriter, r *http.Request) {
	searchType := r.URL.Query().Get("type")
	rawArg := r.URL.Query().Get("arg")

	// Pre-fill value for the search box (with " escaped).
	searchValue := strings.ReplaceAll(rawArg, `"`, `&quot;`)

	// Normalise: trim spaces, convert * to SQL wildcard %.
	arg := strings.TrimSpace(rawArg)
	arg = strings.ReplaceAll(arg, "*", "%")
	arg = strings.ReplaceAll(arg, `\`, `\\`)

	if arg == "" || searchType == "" {
		printSearchError(w, "No search value specified", searchValue, searchType)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))

	switch {
	case strings.HasPrefix(searchType, "Name"):
		if err := lengthCheck(arg); err != nil {
			printSearchError(w, "Regular search doesn't support single character searches for names. Use Advanced Search instead.", searchValue, searchType)
			return
		}
		results, err := SQLFindAuthors(DB, arg, false)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/author.cgi?%d", results[0].AuthorID), http.StatusFound)
			return
		}
		HTMLheader(w, "Name Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Author", "author")
		if len(results) > 0 {
			printSearchAuthorTable(w, results)
		} else {
			printGoogleSearch(w, arg, "name")
		}

	case strings.HasPrefix(searchType, "Fiction Titles"):
		if err := lengthCheck(arg); err != nil {
			printSearchError(w, "Regular search doesn't support single character searches for titles. Use Advanced Search instead.", searchValue, searchType)
			return
		}
		results, err := SQLFindFictionTitles(DB, arg)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/title.cgi?%d", results[0].TitleID), http.StatusFound)
			return
		}
		HTMLheader(w, "Fiction Title Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Title", "title")
		printWholeWordNote(w)
		if len(results) > 0 {
			printSearchTitleTable(w, results)
		} else {
			printGoogleSearch(w, arg, "title")
		}

	case strings.HasPrefix(searchType, "All Titles"):
		if err := lengthCheck(arg); err != nil {
			printSearchError(w, "Regular search doesn't support single character searches for titles. Use Advanced Search instead.", searchValue, searchType)
			return
		}
		results, err := SQLFindTitles(DB, arg)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/title.cgi?%d", results[0].TitleID), http.StatusFound)
			return
		}
		HTMLheader(w, "Title Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Title", "title")
		printWholeWordNote(w)
		if len(results) > 0 {
			printSearchTitleTable(w, results)
		} else {
			printGoogleSearch(w, arg, "title")
		}

	case strings.HasPrefix(searchType, "Year of Title"):
		year, err := validateSearchYear(arg)
		if err != nil {
			printSearchError(w, err.Error(), searchValue, searchType)
			return
		}
		results, dbErr := SQLFindYear(DB, year)
		if dbErr != nil {
			log.Println(dbErr)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/title.cgi?%d", results[0].TitleID), http.StatusFound)
			return
		}
		HTMLheader(w, "Year of Title Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Title", "title")
		if len(results) > 0 {
			printSearchTitleTable(w, results)
		}

	case strings.HasPrefix(searchType, "Month of Title"):
		yearMonth, err := validateSearchMonth(arg)
		if err != nil {
			printSearchError(w, err.Error(), searchValue, searchType)
			return
		}
		results, dbErr := SQLFindMonth(DB, yearMonth)
		if dbErr != nil {
			log.Println(dbErr)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/title.cgi?%d", results[0].TitleID), http.StatusFound)
			return
		}
		HTMLheader(w, "Month of Title Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Title", "title")
		if len(results) > 0 {
			printSearchTitleTable(w, results)
		}

	case strings.HasPrefix(searchType, "Month of Publication"):
		yearMonth, err := validateSearchMonth(arg)
		if err != nil {
			printSearchError(w, err.Error(), searchValue, searchType)
			return
		}
		year, _ := strconv.Atoi(yearMonth[:4])
		month, _ := strconv.Atoi(yearMonth[5:7])
		results, dbErr := SQLGetPubsByMonth(DB, year, month)
		if dbErr != nil {
			log.Println(dbErr)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		HTMLheader(w, "Publication Month Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Publication", "pub")
		if len(results) > 0 {
			PrintPubsTable(w, results, PubTableDefault)
		}

	case strings.HasPrefix(searchType, "Series"):
		results, err := SQLFindSeries(DB, arg, false)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/pe.cgi?%d", results[0].SeriesID), http.StatusFound)
			return
		}
		HTMLheader(w, "Series Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Series", "series")
		if len(results) > 0 {
			printSearchSeriesTable(w, results)
		} else {
			printGoogleSearch(w, arg, "series")
		}

	case strings.HasPrefix(searchType, "Magazine"):
		results, err := SQLFindMagazine(DB, arg)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/pe.cgi?%d", results[0].SeriesID), http.StatusFound)
			return
		}
		HTMLheader(w, "Magazine Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Title", "title")
		if len(results) > 0 {
			printSearchMagazineTable(w, results, arg)
		}

	case strings.HasPrefix(searchType, "Publisher"):
		if err := lengthCheck(arg); err != nil {
			printSearchError(w, "Regular search doesn't support single character searches for publishers. Use Advanced Search instead.", searchValue, searchType)
			return
		}
		results, err := SQLFindPublisher(DB, arg, false)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/publisher.cgi?%d", results[0].PublisherID), http.StatusFound)
			return
		}
		HTMLheader(w, "Publisher Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Publisher", "publisher")
		if len(results) > 0 {
			printSearchPublisherTable(w, results)
		} else {
			printGoogleSearch(w, arg, "publisher")
		}

	case strings.HasPrefix(searchType, "Publication Series"):
		results, err := SQLFindPubSeries(DB, arg, false)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/pubseries.cgi?%d", results[0].PubSeriesID), http.StatusFound)
			return
		}
		HTMLheader(w, "Publication Series Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Publication Series", "pub_series")
		if len(results) > 0 {
			printSearchPubSeriesTable(w, results)
		} else {
			printGoogleSearch(w, arg, "pubseries")
		}

	case strings.HasPrefix(searchType, "ISBN"):
		if err := lengthCheck(arg); err != nil {
			printSearchError(w, "Regular search doesn't support single character searches for ISBNs. Use Advanced Search instead.", searchValue, searchType)
			return
		}
		targets := isbnVariations(arg)
		results, err := SQLFindPubsByISBN(DB, targets)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/pub.cgi?%d", results[0].PubID), http.StatusFound)
			return
		}
		HTMLheader(w, "ISBN Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Publication", "pub")
		if len(results) > 0 {
			PrintPubsTable(w, results, PubTableDefault)
		}

	case strings.HasPrefix(searchType, "Tag"):
		results, err := SQLSearchTags(DB, arg)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			// Tags don't have their own CGI yet; fall through to results page.
			_ = results
		}
		HTMLheader(w, "Tag Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Tag", "tag")
		if len(results) > 0 {
			printSearchTagTable(w, results)
		}

	case strings.HasPrefix(searchType, "Award"):
		results, err := SQLSearchAwardTypes(DB, arg)
		if err != nil {
			log.Println(err)
			printSearchError(w, "Database error", searchValue, searchType)
			return
		}
		if len(results) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/awardtype.cgi?%d", results[0].AwardTypeID), http.StatusFound)
			return
		}
		HTMLheader(w, "Award Search")
		PrintNavbar(w, "search", searchValue, searchType)
		printSearchSummary(w, arg, len(results), "Award Type", "award_type")
		if len(results) > 0 {
			printSearchAwardTypeTable(w, results)
		}

	default:
		printSearchError(w, "Unknown search type", searchValue, searchType)
		return
	}

	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// ── Shared search helpers ─────────────────────────────────────────────────────

func printSearchError(w http.ResponseWriter, msg, searchValue, searchType string) {
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, "Search Error")
	PrintNavbar(w, "search", searchValue, searchType)
	fmt.Fprintf(w, "<h2>%s</h2>\n", ISFDBText(msg))
	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

func printSearchSummary(w io.Writer, arg string, count int, searchType, advAbbrev string) {
	fmt.Fprintf(w, "<p><b>A search for '%s' found %d matches. ", ISFDBText(arg), count)
	if count >= searchLimit {
		fmt.Fprintf(w, "<br>The first %d matches are displayed below. Use ", searchLimit)
		fmt.Fprintf(w, "<a href=\"%s://%s/adv_search_selection.cgi?%s\" class=\"inverted\">Advanced %s Search</a>",
			PROTOCOL, HTMLHOST, advAbbrev, searchType)
		fmt.Fprint(w, " to see more matches.")
	}
	fmt.Fprintln(w, `</b>`)
	fmt.Fprintln(w, `<p>`)
}

func printWholeWordNote(w io.Writer) {
	fmt.Fprintf(w, `Note that All Titles and Fiction Titles searches are now limited to
complete words for performance reasons. If you want to search for a
substring, you will need to use <a href="%s://%s/adv_search_selection.cgi?title" class="inverted">Advanced Title Search</a>.`,
		PROTOCOL, HTMLHOST)
	fmt.Fprintln(w)
}

func printGoogleSearch(w io.Writer, arg, searchType string) {
	fmt.Fprintf(w, `<p>You can also try:
<form METHOD="GET" action="%s://%s/google_search_redirect.cgi" accept-charset="utf-8">
<p>
<select NAME="OPERATOR">
<option VALUE="exact">exact %s search
<option SELECTED VALUE="approximate">approximate %s search
</select>
 on <input NAME="SEARCH_VALUE" SIZE="50" VALUE="%s">
<input NAME="PAGE_TYPE" VALUE="%s" TYPE="HIDDEN">
<input TYPE="SUBMIT" VALUE="using Google">
</form>`, PROTOCOL, HTMLHOST, searchType, searchType, ISFDBText(arg), searchType)
	fmt.Fprintln(w)
}

func lengthCheck(arg string) error {
	stripped := strings.NewReplacer("_", "", "*", "", "%", "").Replace(arg)
	if len(stripped) < 2 {
		return fmt.Errorf("search term too short")
	}
	return nil
}

func validateSearchYear(s string) (int, error) {
	if len(s) != 4 {
		return 0, fmt.Errorf("Year must be specified using the YYYY format")
	}
	year, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("Year must be specified using the YYYY format")
	}
	now := time.Now().Year()
	if year < 1 || year > now+1 {
		return 0, fmt.Errorf("Year must be YYYY between 0001 and one year in the future")
	}
	return year, nil
}

// validateSearchMonth validates "YYYY-MM" format and returns the canonical
// "YYYY-MM" string (zero-padded month).
func validateSearchMonth(s string) (string, error) {
	if len(s) != 7 || s[4] != '-' {
		return "", fmt.Errorf("Month must be specified using the YYYY-MM format")
	}
	year, err1 := strconv.Atoi(s[:4])
	month, err2 := strconv.Atoi(s[5:7])
	if err1 != nil || err2 != nil {
		return "", fmt.Errorf("Month must be specified using the YYYY-MM format")
	}
	now := time.Now().Year()
	if year < 1 || year > now+1 {
		return "", fmt.Errorf("Year must be between 1 and one year in the future")
	}
	if month < 1 || month > 12 {
		return "", fmt.Errorf("Month must be between 01 and 12")
	}
	return fmt.Sprintf("%04d-%02d", year, month), nil
}

// isbnVariations builds a list of ISBN search patterns from a user-supplied string,
// mirroring the Python isbnVariations() function.
func isbnVariations(original string) []string {
	if original == "" {
		return nil
	}
	original = strings.ReplaceAll(original, "x", "X")
	variations := []string{original}

	if !ValidISBN(original) {
		return variations
	}

	collapsed := strings.ReplaceAll(strings.ReplaceAll(original, "-", ""), " ", "")
	if len(collapsed) == len(original) {
		// Original was not punctuated — add the punctuated form.
		if formatted, err := ConvertISBN(DB, original); err == nil {
			variations = append(variations, formatted)
		}
	} else {
		// Original was punctuated — add the collapsed form.
		variations = append(variations, collapsed)
	}

	var other string
	if len(collapsed) == 10 {
		other = ToISBN13(collapsed)
	} else {
		other = ToISBN10(collapsed)
	}
	if other != "" {
		variations = append(variations, other)
		if formatted, err := ConvertISBN(DB, other); err == nil {
			variations = append(variations, formatted)
		}
	}
	return variations
}

// ── Result table renderers ────────────────────────────────────────────────────

func printSearchAuthorTable(w io.Writer, authors []*Author) {
	// Batch-load pseudonym status.
	ids := make([]int, len(authors))
	for i, a := range authors {
		ids[i] = a.AuthorID
	}
	pseudoMap, _ := SQLBatchAuthorIsPseudo(DB, ids)

	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th>Author</th>`)
	fmt.Fprintln(w, `<th>Alternate Name?</th>`)
	fmt.Fprintln(w, `<th>Working Language</th>`)
	fmt.Fprintln(w, `<th>Directory Entry</th>`)
	fmt.Fprintln(w, `<th>Legal Name</th>`)
	fmt.Fprintln(w, `<th>Birth Place</th>`)
	fmt.Fprintln(w, `<th>Birth Date</th>`)
	fmt.Fprintln(w, `<th>Death Date</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, a := range authors {
		if i >= searchLimit {
			break
		}
		bgcolor := (i%2 + 1)
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor)

		// Author link.
		fmt.Fprintf(w, "<td><a href=\"/author.cgi?%d\">%s</a></td>\n",
			a.AuthorID, ISFDBText(a.AuthorCanonical.String))

		// Pseudonym indicator.
		if pseudoMap[a.AuthorID] {
			fmt.Fprintln(w, "<td>Alternate Name</td>")
		} else {
			fmt.Fprintln(w, "<td>-</td>")
		}

		// Working language.
		if a.AuthorLanguage.Valid {
			if name, ok := Languages[int(a.AuthorLanguage.Int32)]; ok {
				fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(name))
			} else {
				fmt.Fprintln(w, "<td>-</td>")
			}
		} else {
			fmt.Fprintln(w, "<td>-</td>")
		}

		// Directory entry (last name).
		if a.AuthorLastName.Valid && a.AuthorLastName.String != "" {
			fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(a.AuthorLastName.String))
		} else {
			fmt.Fprintln(w, "<td>-</td>")
		}

		// Legal name.
		if a.AuthorLegalName.Valid && a.AuthorLegalName.String != "" {
			fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(a.AuthorLegalName.String))
		} else {
			fmt.Fprintln(w, "<td>-</td>")
		}

		// Birth place.
		if a.AuthorBirthPlace.Valid && a.AuthorBirthPlace.String != "" {
			fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(a.AuthorBirthPlace.String))
		} else {
			fmt.Fprintln(w, "<td>-</td>")
		}

		// Birth date.
		if a.AuthorBirthDate.Valid && a.AuthorBirthDate.String != "" {
			fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(a.AuthorBirthDate.String))
		} else {
			fmt.Fprintln(w, "<td>-</td>")
		}

		// Death date.
		if a.AuthorDeathDate.Valid && a.AuthorDeathDate.String != "" {
			fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(a.AuthorDeathDate.String))
		} else {
			fmt.Fprintln(w, "<td>-</td>")
		}

		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)
}

func printSearchTitleTable(w io.Writer, titles []*Title) {
	// Cap at limit.
	if len(titles) > searchLimit {
		titles = titles[:searchLimit]
	}

	// Batch-load authors for all titles.
	ids := make([]int, len(titles))
	for i, t := range titles {
		ids[i] = t.TitleID
	}
	authorsByTitle, _ := SQLTitleAuthorsBatch(DB, ids)

	// Batch-load parent title data for titles that have a parent.
	parentIDs := []int{}
	for _, t := range titles {
		if t.TitleParent != 0 {
			parentIDs = append(parentIDs, t.TitleParent)
		}
	}
	parentTitles, _ := SQLloadTitlesBatch(DB, parentIDs)
	parentAuthors := map[int][]AuthorRef{}
	if len(parentIDs) > 0 {
		parentAuthors, _ = SQLTitleAuthorsBatch(DB, parentIDs)
	}

	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th>Date</th>`)
	fmt.Fprintln(w, `<th>Type</th>`)
	fmt.Fprintln(w, `<th>Language</th>`)
	fmt.Fprintln(w, `<th>Title</th>`)
	fmt.Fprintln(w, `<th>Authors</th>`)
	fmt.Fprintln(w, `<th>Parent Title</th>`)
	fmt.Fprintln(w, `<th>Parent Authors</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, t := range titles {
		bgcolor := (i%2 + 1)
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor)

		// Date.
		dateStr := ISFDBconvertDate(t.TitleCopyright.String, 1)
		fmt.Fprintf(w, "<td>%s</td>\n", dateStr)

		// Type.
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(t.TitleTType.String))

		// Language.
		if t.TitleLanguage.Valid {
			if name, ok := Languages[int(t.TitleLanguage.Int32)]; ok {
				fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(name))
			} else {
				fmt.Fprintln(w, "<td>&nbsp;</td>")
			}
		} else {
			fmt.Fprintln(w, "<td>&nbsp;</td>")
		}

		// Title link.
		fmt.Fprintf(w, "<td dir=\"ltr\"><a href=\"/title.cgi?%d\">%s</a></td>\n",
			t.TitleID, ISFDBText(t.TitleTitle.String))

		// Authors.
		authors := authorsByTitle[t.TitleID]
		fmt.Fprintf(w, "<td>%s</td>\n", formatSearchAuthors(authors))

		// Parent title and parent authors.
		if t.TitleParent != 0 {
			parentID := t.TitleParent
			if pt, ok := parentTitles[parentID]; ok {
				fmt.Fprintf(w, "<td><a href=\"/title.cgi?%d\">%s</a></td>\n",
					pt.TitleID, ISFDBText(pt.TitleTitle.String))
				pAuthors := parentAuthors[parentID]
				// Only show parent authors if different from title authors.
				if !authorSetsEqual(authors, pAuthors) {
					fmt.Fprintf(w, "<td>%s</td>\n", formatSearchAuthors(pAuthors))
				} else {
					fmt.Fprintln(w, "<td>&nbsp;</td>")
				}
			} else {
				fmt.Fprintln(w, "<td>&nbsp;</td><td>&nbsp;</td>")
			}
		} else {
			fmt.Fprintln(w, "<td>&nbsp;</td><td>&nbsp;</td>")
		}

		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)
}

// formatSearchAuthors builds a comma-separated list of linked author names.
func formatSearchAuthors(authors []AuthorRef) string {
	if len(authors) == 0 {
		return "&nbsp;"
	}
	parts := make([]string, len(authors))
	for i, a := range authors {
		parts[i] = fmt.Sprintf("<a href=\"/author.cgi?%d\">%s</a>", a.AuthorID, ISFDBText(a.Canonical))
	}
	return strings.Join(parts, ", ")
}

// authorSetsEqual returns true if both slices contain the same author IDs.
func authorSetsEqual(a, b []AuthorRef) bool {
	if len(a) != len(b) {
		return false
	}
	aSet := make(map[int]bool, len(a))
	for _, x := range a {
		aSet[x.AuthorID] = true
	}
	for _, x := range b {
		if !aSet[x.AuthorID] {
			return false
		}
	}
	return true
}

func printSearchSeriesTable(w io.Writer, series []*SearchSeriesResult) {
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th>Series</th>`)
	fmt.Fprintln(w, `<th>Parent Series</th>`)
	fmt.Fprintln(w, `<th>Position Within Parent Series</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, s := range series {
		if i >= searchLimit {
			break
		}
		bgcolor := (i%2 + 1)
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor)
		fmt.Fprintf(w, "<td><a href=\"/pe.cgi?%d\">%s</a></td>\n",
			s.SeriesID, ISFDBText(s.SeriesTitle))
		if s.ParentID != 0 && s.ParentTitle != "" {
			fmt.Fprintf(w, "<td><a href=\"/pe.cgi?%d\">%s</a></td>\n",
				s.ParentID, ISFDBText(s.ParentTitle))
		} else {
			fmt.Fprintln(w, "<td>&nbsp;</td>")
		}
		if s.ParentPosition.Valid && s.ParentPosition.String != "" {
			fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(s.ParentPosition.String))
		} else {
			fmt.Fprintln(w, "<td>&nbsp;</td>")
		}
		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)
}

func printSearchMagazineTable(w io.Writer, results []*MagazineSearchResult, arg string) {
	fmt.Fprintln(w, `<h3>Note: The search results displayed below include all series
AND magazine title records that match the entered value.
Matching magazines whose series titles do not match the
entered value have asterisks next to their titles.</h3>`)

	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr class="generic_table_header">`)
	fmt.Fprintln(w, `<th>Magazine Series</th>`)
	fmt.Fprintln(w, `<th>Parent Series</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, r := range results {
		if i >= searchLimit {
			break
		}
		bgcolor := (i%2 + 1)
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor)

		// Magazine column: series link, asterisk if title≠series_title, issue grid link.
		fmt.Fprintf(w, "<td><a href=\"/pe.cgi?%d\">%s</a>",
			r.SeriesID, ISFDBText(r.DisplayTitle))
		if r.DisplayTitle != r.SeriesTitle {
			fmt.Fprint(w, "*")
		}
		fmt.Fprintf(w, " <a href=\"/seriesgrid.cgi?%d\">(issue grid)</a></td>\n", r.SeriesID)

		// Parent column.
		if r.ParentID != 0 && r.ParentTitle != "" {
			fmt.Fprintf(w, "<td><a href=\"/pe.cgi?%d\">%s</a>",
				r.ParentID, ISFDBText(r.ParentTitle))
			fmt.Fprintf(w, " <a href=\"/seriesgrid.cgi?%d\">(issue grid)</a></td>\n", r.ParentID)
		} else {
			fmt.Fprintln(w, "<td>-</td>")
		}
		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)
}

func printSearchPublisherTable(w io.Writer, publishers []*Publisher) {
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th>Publisher</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, p := range publishers {
		if i >= searchLimit {
			break
		}
		bgcolor := (i%2 + 1)
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor)
		fmt.Fprintf(w, "<td><a href=\"/publisher.cgi?%d\">%s</a></td>\n",
			p.PublisherID, ISFDBText(p.PublisherName.String))
		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)
}

func printSearchPubSeriesTable(w io.Writer, series []*PubSeriesRecord) {
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th>Publication Series</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, ps := range series {
		if i >= searchLimit {
			break
		}
		bgcolor := (i%2 + 1)
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor)
		fmt.Fprintf(w, "<td><a href=\"/pubseries.cgi?%d\">%s</a></td>\n",
			ps.PubSeriesID, ISFDBText(ps.PubSeriesName.String))
		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)
}

func printSearchTagTable(w io.Writer, tags []*Tag) {
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th>Tag Name</th>`)
	fmt.Fprintln(w, `<th>Private?</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, t := range tags {
		if i >= searchLimit {
			break
		}
		bgcolor := (i%2 + 1)
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor)
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(t.TagName.String))
		if t.TagStatus != 0 {
			fmt.Fprintln(w, "<td><b>Private</b></td>")
		} else {
			fmt.Fprintln(w, "<td></td>")
		}
		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)
}

func printSearchAwardTypeTable(w io.Writer, awards []*AwardType) {
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th>Short Award Name</th>`)
	fmt.Fprintln(w, `<th>Full Award Name</th>`)
	fmt.Fprintln(w, `<th>Awarded For</th>`)
	fmt.Fprintln(w, `<th>Awarded By</th>`)
	fmt.Fprintln(w, `<th>Poll</th>`)
	fmt.Fprintln(w, `<th>Non-Genre</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, at := range awards {
		if i >= searchLimit {
			break
		}
		bgcolor := (i%2 + 1)
		fmt.Fprintf(w, "<tr align=left class=\"table%d\">\n", bgcolor)
		fmt.Fprintf(w, "<td><a href=\"/awardtype.cgi?%d\">%s</a></td>\n",
			at.AwardTypeID, ISFDBText(at.AwardTypeShortName.String))
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(at.AwardTypeName.String))
		awardFor := at.AwardTypeFor.String
		if !at.AwardTypeFor.Valid || awardFor == "" {
			awardFor = "-"
		}
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(awardFor))
		awardBy := at.AwardTypeBy.String
		if !at.AwardTypeBy.Valid || awardBy == "" {
			awardBy = "-"
		}
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(awardBy))
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(at.AwardTypePoll.String))
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(at.AwardTypeNonGenre.String))
		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)
}
