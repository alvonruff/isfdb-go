// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"
)

// normalizeDirectoryRune maps a rune to its lowercase ASCII equivalent for
// directory key generation.  It mirrors Python's unicodedata.normalize('NFKD')
// followed by encode('ascii', 'ignore').  Returns 0 if there is no mapping.
func normalizeDirectoryRune(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	if (r >= 'a' && r <= 'z') || r == '\'' || r == '.' || r == '/' || r == '*' || r == ' ' {
		return r
	}
	// Common Latin extended characters → their ASCII base.
	switch {
	case r >= 'À' && r <= 'Å':
		return 'a'
	case r == 'Æ' || r == 'æ':
		return 'a'
	case r >= 'à' && r <= 'å':
		return 'a'
	case r == 'Ç' || r == 'ç':
		return 'c'
	case r >= 'È' && r <= 'Ë':
		return 'e'
	case r >= 'è' && r <= 'ë':
		return 'e'
	case r >= 'Ì' && r <= 'Ï':
		return 'i'
	case r >= 'ì' && r <= 'ï':
		return 'i'
	case r == 'Ñ' || r == 'ñ':
		return 'n'
	case r >= 'Ò' && r <= 'Ö':
		return 'o'
	case r >= 'ò' && r <= 'ö':
		return 'o'
	case r == 'Ø' || r == 'ø':
		return 'o'
	case r >= 'Ù' && r <= 'Ü':
		return 'u'
	case r >= 'ù' && r <= 'ü':
		return 'u'
	case r == 'Ý' || r == 'ý':
		return 'y'
	case r == 'ß':
		return 's'
	case r == 'Ð' || r == 'ð':
		return 'd'
	case r == 'Þ' || r == 'þ':
		return 't'
	}
	// Strip Unicode combining marks (category Mn) — they arise from
	// NFKD decomposition but Go strings are already NFC, so we see the
	// precomposed form above.  Any other non-ASCII rune: no mapping.
	if unicode.Is(unicode.Mn, r) {
		return 0
	}
	return 0
}

// directoryKey returns the two-character lowercase key for a name, or ""
// if the name is too short / starts with unmappable characters.
func directoryKey(name string) string {
	runes := []rune(name)
	var out []rune
	for _, r := range runes {
		mapped := normalizeDirectoryRune(r)
		if mapped != 0 {
			out = append(out, mapped)
			if len(out) == 2 {
				break
			}
		}
	}
	if len(out) < 2 {
		if len(out) == 1 {
			return string(out) + " "
		}
		return ""
	}
	return string(out)
}

// -----------------------------------------------------------------------------
// SQL helpers
// -----------------------------------------------------------------------------

// SQLGetAuthorDirectoryMap returns the set of populated 2-char prefix keys
// for the Author Directory.
func SQLGetAuthorDirectoryMap(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(
		"SELECT lower(substr(author_lastname,1,2)) FROM authors WHERE author_lastname IS NOT NULL AND author_lastname != ''",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			continue
		}
		if len([]rune(key)) >= 2 {
			m[key] = true
		}
	}
	return m, rows.Err()
}

// SQLGetPublisherDirectoryMap returns the set of populated 2-char prefix keys
// for the Publisher Directory (publisher_name UNION trans_publisher_name).
func SQLGetPublisherDirectoryMap(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(
		"SELECT publisher_name FROM publishers WHERE publisher_name IS NOT NULL AND publisher_name != ''" +
			" UNION " +
			"SELECT trans_publisher_name FROM trans_publisher WHERE trans_publisher_name IS NOT NULL AND trans_publisher_name != ''",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		if k := directoryKey(name); k != "" {
			m[k] = true
		}
	}
	return m, rows.Err()
}

// SQLGetMagazineDirectoryMap returns the set of populated 2-char prefix keys
// for the Magazine Directory.  It combines series titles, editor title_titles,
// and trans_title_titles — exactly mirroring the Python approach.
func SQLGetMagazineDirectoryMap(db *sql.DB) (map[string]bool, error) {
	// Collect distinct 2-char directory keys directly in SQLite to avoid
	// fetching tens of thousands of full title strings into Go.

	type prefixQuery struct {
		sql  string
		args []any
	}
	queries := []prefixQuery{
		// 1. Series titles for series that contain EDITOR titles.
		{`SELECT DISTINCT s.series_title
		  FROM series s
		  JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		  WHERE s.series_title IS NOT NULL AND s.series_title != ''`, nil},
		// 2. title_title for EDITOR titles.
		{`SELECT DISTINCT title_title FROM titles
		  WHERE title_ttype = 'EDITOR' AND title_title IS NOT NULL AND title_title != ''`, nil},
		// 3. trans_title_title for EDITOR titles.
		{`SELECT DISTINCT tt.trans_title_title
		  FROM trans_titles tt
		  JOIN titles t ON t.title_id = tt.title_id AND t.title_ttype = 'EDITOR'
		  WHERE tt.trans_title_title IS NOT NULL AND tt.trans_title_title != ''`, nil},
	}

	m := map[string]bool{}
	for _, q := range queries {
		rows, err := db.Query(q.sql, q.args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var s string
			if rows.Scan(&s) == nil {
				if k := directoryKey(s); k != "" {
					m[k] = true
				}
			}
		}
		rows.Close()
	}
	return m, nil
}

// PublisherDirEntry is one row in a publisher sub-directory listing.
type PublisherDirEntry struct {
	PublisherID   int
	PublisherName string
}

// SQLGetPublishersByPrefix returns publishers whose name starts with the
// given two-character prefix (case-insensitive LIKE), including matches via
// translated names.  Results are sorted by publisher_name.
func SQLGetPublishersByPrefix(db *sql.DB, prefix string) ([]*PublisherDirEntry, error) {
	like := prefix + "%"
	rows, err := db.Query(`
		SELECT DISTINCT p.publisher_id, p.publisher_name
		FROM publishers p
		WHERE p.publisher_name LIKE ? COLLATE NOCASE
		UNION
		SELECT DISTINCT p.publisher_id, p.publisher_name
		FROM publishers p
		JOIN trans_publisher tp ON tp.publisher_id = p.publisher_id
		WHERE tp.trans_publisher_name LIKE ? COLLATE NOCASE
		ORDER BY publisher_name`,
		like, like,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*PublisherDirEntry
	for rows.Next() {
		var e PublisherDirEntry
		var name sql.NullString
		if err := rows.Scan(&e.PublisherID, &name); err != nil {
			return nil, err
		}
		e.PublisherName = name.String
		result = append(result, &e)
	}
	return result, rows.Err()
}

// SQLFindMagazineByPrefix returns magazine search results whose display title
// starts with the given prefix (used for Magazine Directory sub-pages).
// It reuses the MagazineSearchResult type from biblio.go.
func SQLFindMagazineByPrefix(db *sql.DB, prefix string) ([]*MagazineSearchResult, error) {
	like := prefix + "%"

	// Step 1: series whose series_title starts with the prefix.
	rows, err := db.Query(`
		SELECT DISTINCT s.series_id, s.series_title, COALESCE(s.series_parent,0)
		FROM series s
		JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		WHERE s.series_title LIKE ? COLLATE NOCASE
		UNION
		SELECT DISTINCT s.series_id, s.series_title, COALESCE(s.series_parent,0)
		FROM series s
		JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		JOIN trans_series ts ON ts.series_id = s.series_id
		WHERE ts.trans_series_name LIKE ? COLLATE NOCASE`,
		like, like,
	)
	if err != nil {
		return nil, err
	}
	seenIDs := map[int]bool{}
	byID := map[int]*MagazineSearchResult{}
	var order []string
	for rows.Next() {
		var r MagazineSearchResult
		if err := rows.Scan(&r.SeriesID, &r.SeriesTitle, &r.ParentID); err != nil {
			rows.Close()
			return nil, err
		}
		r.DisplayTitle = r.SeriesTitle
		if !seenIDs[r.SeriesID] {
			seenIDs[r.SeriesID] = true
			byID[r.SeriesID] = &r
			order = append(order, r.DisplayTitle)
		}
	}
	rows.Close()

	// Step 2: EDITOR title_titles that start with the prefix but whose series title doesn't.
	rows2, err := db.Query(`
		SELECT DISTINCT t.title_title, s.series_id, s.series_title, COALESCE(s.series_parent,0)
		FROM series s
		JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		WHERE t.title_title LIKE ? COLLATE NOCASE AND s.series_title NOT LIKE ? COLLATE NOCASE
		UNION
		SELECT DISTINCT t.title_title, s.series_id, s.series_title, COALESCE(s.series_parent,0)
		FROM series s
		JOIN titles t ON t.series_id = s.series_id AND t.title_ttype = 'EDITOR'
		JOIN trans_titles tt ON tt.title_id = t.title_id
		WHERE tt.trans_title_title LIKE ? COLLATE NOCASE AND s.series_title NOT LIKE ? COLLATE NOCASE`,
		like, like, like, like,
	)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var titleTitle, seriesTitle string
			var seriesID, parentID int
			if err := rows2.Scan(&titleTitle, &seriesID, &seriesTitle, &parentID); err != nil {
				continue
			}
			if seenIDs[seriesID] {
				continue
			}
			if i := strings.LastIndex(titleTitle, " - "); i != -1 {
				titleTitle = titleTitle[:i]
			}
			seenIDs[seriesID] = true
			r := &MagazineSearchResult{
				DisplayTitle: titleTitle,
				SeriesID:     seriesID,
				SeriesTitle:  seriesTitle,
				ParentID:     parentID,
			}
			byID[r.SeriesID] = r
			order = append(order, titleTitle)
		}
	}

	// Sort alphabetically (case-insensitive).
	sort.Slice(order, func(i, j int) bool {
		return strings.ToLower(order[i]) < strings.ToLower(order[j])
	})

	// Deduplicate: walk order, emit each series once.
	seen2 := map[int]bool{}
	var results []*MagazineSearchResult
	for _, title := range order {
		for _, r := range byID {
			if r.DisplayTitle == title && !seen2[r.SeriesID] {
				seen2[r.SeriesID] = true
				results = append(results, r)
			}
		}
	}

	// Batch-load parent series titles.
	parentIDs := map[int]bool{}
	for _, r := range results {
		if r.ParentID != 0 {
			parentIDs[r.ParentID] = true
		}
	}
	if len(parentIDs) > 0 {
		ids := make([]int, 0, len(parentIDs))
		for id := range parentIDs {
			ids = append(ids, id)
		}
		ph := make([]string, len(ids))
		args := make([]any, len(ids))
		for i, id := range ids {
			ph[i] = "?"
			args[i] = id
		}
		prows, err := db.Query(
			"SELECT series_id, series_title FROM series WHERE series_id IN ("+strings.Join(ph, ",")+")",
			args...,
		)
		if err == nil {
			defer prows.Close()
			parentTitles := map[int]string{}
			for prows.Next() {
				var id int
				var t string
				if prows.Scan(&id, &t) == nil {
					parentTitles[id] = t
				}
			}
			for _, r := range results {
				if r.ParentID != 0 {
					r.ParentTitle = parentTitles[r.ParentID]
				}
			}
		}
	}

	return results, nil
}

// -----------------------------------------------------------------------------
// HTTP handler
// -----------------------------------------------------------------------------

// DirectoryHandler serves /directory.cgi?author, /directory.cgi?publisher,
// and /directory.cgi?magazine, plus sub-directory pages like
// /directory.cgi?publisher+ab.
func DirectoryHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) == 0 {
		http.Error(w, "Missing directory type", http.StatusBadRequest)
		return
	}

	dirType := strings.ToLower(params[0])
	section := ""
	if len(params) > 1 {
		section = strings.ToLower(params[1])
	}

	// Authors do not support sub-directory pages.
	if dirType == "author" && section != "" {
		http.Error(w, "Second parameter not allowed for Author Directory", http.StatusBadRequest)
		return
	}

	// Page title.
	title := strings.Title(dirType) + " Directory"
	if section != "" {
		title += ": " + strings.ToUpper(section)
	}

	HTMLheader(w, title)
	PrintNavbar(w, "directory", "", "")

	// Build the records map (which 2-char slots are populated).
	db := DB
	var recordsMap map[string]bool
	var dbErr error
	switch dirType {
	case "publisher":
		recordsMap, dbErr = SQLGetPublisherDirectoryMap(db)
	case "magazine":
		recordsMap, dbErr = SQLGetMagazineDirectoryMap(db)
	default:
		recordsMap, dbErr = SQLGetAuthorDirectoryMap(db)
	}
	if dbErr != nil {
		fmt.Fprintf(w, "<p>Database error: %v</p>\n", dbErr)
		HTMLtrailer(w)
		return
	}

	// Characters used for the grid.
	firstChars := "abcdefghijklmnopqrstuvwxyz'"
	secondChars := firstChars + "./"
	if dirType != "author" {
		// Publisher and magazine directories support '*' in names.
		secondChars += "*"
	}

	if section == "" {
		// ---------------------------------------------------------------
		// Top-level directory: show the 2-char grid.
		// ---------------------------------------------------------------

		// if dirType == "publisher" {
	        //	fmt.Fprintf(w, `<p>Also see the ISFDB Wiki <a href="%s://%s/index.php/Category:Publishers">category</a> for publishers</p>`+"\n",
	        // 			PROTOCOL, WIKILOC)
	        // 	} else if dirType == "magazine" {
	        // 		fmt.Fprintf(w, `<p>Also see the ISFDB Wiki pages for `+
	        // 			`<a href="%s://%s/index.php/Magazines">magazines</a> and `+
	        // 			`<a href="%s://%s/index.php/Fanzines">fanzines</a></p>`+"\n",
	        // 			PROTOCOL, WIKILOC, PROTOCOL, WIKILOC)
	        // 	}

		if dirType != "magazine" {
			fmt.Fprintf(w, "<h2>Directory of %s names starting with:</h2><p>\n", dirType)
		} else {
			fmt.Fprintf(w, "<h2>Directory of magazine and fanzine names starting with:</h2><p>\n")
		}

		fmt.Fprintln(w, `<table class="authordirectory">`)
		for _, x := range firstChars {
			fmt.Fprintln(w, "<tr>")
			for _, y := range secondChars {
				key := string(x) + string(y)
				if recordsMap[key] {
					if dirType == "author" {
						// Link to Advanced Author Search.
						href := fmt.Sprintf("%s://%s/adv_search_results.cgi?START=0&USE_1=author_lastname&OPERATOR_1=starts_with&TERM_1=%s&ORDERBY=author_lastname&TYPE=Author&C=AND",
							PROTOCOL, HTMLHOST, key)
						fmt.Fprintf(w, "<td><a href=\"%s\"><b>%s%s</b></a></td>\n",
							href,
							strings.ToUpper(string(x)),
							string(y))
					} else {
						// Link to the sub-directory page.
						subURL := fmt.Sprintf("%s://%s/directory.cgi?%s+%s",
							PROTOCOL, HTMLHOST, dirType, key)
						fmt.Fprintf(w, "<td><a href=\"%s\" class=\"bold\"><b>%s%s</b></a></td>\n",
							subURL,
							strings.ToUpper(string(x)),
							string(y))
					}
				} else {
					fmt.Fprintf(w, "<td class=\"authordirectorynolink\">%s%s</td>\n",
						strings.ToUpper(string(x)),
						string(y))
				}
			}
			fmt.Fprintln(w, "</tr>")
		}
		fmt.Fprintln(w, "</table>")
		fmt.Fprintln(w, "<p>")

	} else {
		// ---------------------------------------------------------------
		// Sub-directory page: list matching records.
		// ---------------------------------------------------------------
		if len([]rune(section)) == 1 {
			fmt.Fprintln(w, "<h3>ERROR: Single character directories are not allowed due to excessive load on the server</h3>")
			HTMLtrailer(w)
			return
		}

		// Navigation: find the previous populated slot.
		xChar := rune(section[0])
		yChar := rune(section[1])
		yIdx := strings.IndexRune(secondChars, yChar)

		for i := yIdx - 1; i >= 0; i-- {
			key := string(xChar) + string([]rune(secondChars)[i])
			if recordsMap[key] {
				backURL := fmt.Sprintf("%s://%s/directory.cgi?%s+%s",
					PROTOCOL, HTMLHOST, dirType, key)
				fmt.Fprintf(w, "<small><a href=\"%s\" class=\"bold\">Back to %s%s</a></small>\n",
					backURL,
					strings.ToUpper(string(xChar)),
					string([]rune(secondChars)[i]))
				break
			}
		}

		// Link back up to the top-level directory.
		upURL := fmt.Sprintf("%s://%s/directory.cgi?%s", PROTOCOL, HTMLHOST, dirType)
		fmt.Fprintf(w, "<a href=\"%s\" class=\"bold\">Up to %s Directory</a>\n",
			upURL, strings.Title(dirType))

		// Find the next populated slot.
		for i := yIdx + 1; i < len([]rune(secondChars)); i++ {
			key := string(xChar) + string([]rune(secondChars)[i])
			if recordsMap[key] {
				fwdURL := fmt.Sprintf("%s://%s/directory.cgi?%s+%s",
					PROTOCOL, HTMLHOST, dirType, key)
				fmt.Fprintf(w, "<small><a href=\"%s\" class=\"bold\">Forward to %s%s</a></small>\n",
					fwdURL,
					strings.ToUpper(string(xChar)),
					string([]rune(secondChars)[i]))
				break
			}
		}

		fmt.Fprintln(w, "<br><br>")

		if dirType == "magazine" {
			// Magazine sub-directory.
			results, err := SQLFindMagazineByPrefix(db, section)
			if err != nil {
				fmt.Fprintf(w, "<p>Database error: %v</p>\n", err)
				HTMLtrailer(w)
				return
			}
			if len(results) == 0 {
				fmt.Fprintf(w, "<b>No magazine names found starting with: %s</b>\n", strings.ToUpper(section))
			} else {
				fmt.Fprintf(w, `<b>Note: Matching magazines whose series titles do not match the `+
					`entered value have asterisks next to their titles.<br><br>`+
					`Number of magazine names starting with "%s": %d</b><br><br>`+"\n",
					strings.ToUpper(section), len(results))
				fmt.Fprintln(w, `<table class="generic_table">`)
				fmt.Fprintln(w, `<tr class="generic_table_header"><th>Magazine</th><th>Parent Series</th></tr>`)
				for i, res := range results {
					rowClass := "table1"
					if i%2 == 1 {
						rowClass = "table2"
					}
					fmt.Fprintf(w, "<tr align=\"left\" class=\"%s\">\n", rowClass)
					// Magazine title link + optional asterisk + issue grid link.
					fmt.Fprintf(w, "<td>%s",
						ISFDBLink("pe.cgi", res.SeriesID, res.DisplayTitle))
					if res.DisplayTitle != res.SeriesTitle {
						fmt.Fprint(w, "*")
					}
					fmt.Fprintf(w, " %s</td>\n",
						ISFDBLinkNoName("seriesgrid.cgi", res.SeriesID, "(issue grid)"))
					// Parent series column.
					if res.ParentID != 0 {
						fmt.Fprintf(w, "<td>%s %s</td>\n",
							ISFDBLink("pe.cgi", res.ParentID, res.ParentTitle),
							ISFDBLinkNoName("seriesgrid.cgi", res.ParentID, "(issue grid)"))
					} else {
						fmt.Fprintln(w, "<td>-</td>")
					}
					fmt.Fprintln(w, "</tr>")
				}
				fmt.Fprintln(w, "</table>")
			}
		} else {
			// Publisher sub-directory.
			entries, err := SQLGetPublishersByPrefix(db, section)
			if err != nil {
				fmt.Fprintf(w, "<p>Database error: %v</p>\n", err)
				HTMLtrailer(w)
				return
			}
			if len(entries) == 0 {
				fmt.Fprintf(w, "<h3>No publisher names found starting with: %s</h3>\n",
					strings.ToUpper(section))
			} else {
				fmt.Fprintf(w, "<h3>Number of publisher names starting with \"%s\": %d</h3>\n",
					strings.ToUpper(section), len(entries))
				fmt.Fprintln(w, `<table class="generic_table">`)
				fmt.Fprintln(w, `<tr class="generic_table_header"><td><b>Publisher</b></td></tr>`)
				for i, e := range entries {
					rowClass := "table1"
					if i%2 == 1 {
						rowClass = "table2"
					}
					fmt.Fprintf(w, "<tr align=\"left\" class=\"%s\"><td>%s</td></tr>\n",
						rowClass,
						ISFDBLink("publisher.cgi", e.PublisherID, ISFDBText(e.PublisherName)))
				}
				fmt.Fprintln(w, "</table><p>")
			}
		}
	}

	HTMLtrailer(w)
}
