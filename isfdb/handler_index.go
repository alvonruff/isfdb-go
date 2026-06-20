// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"
)

// BirthdayAuthor holds the data needed to render one author card on the home page.
type BirthdayAuthor struct {
	AuthorID   int
	Canonical  string
	BirthDate  string
	ImageURL   string
}

// SQLGetTodaysBirthdayAuthors returns living marque authors who have a photo
// and whose birthday falls on today's month/day.
func SQLGetTodaysBirthdayAuthors(db *sql.DB) ([]*BirthdayAuthor, error) {
	rows, err := db.Query(`
		SELECT author_id, author_canonical, author_birthdate, author_image
		FROM authors
		WHERE author_marque = 1
		  AND author_image IS NOT NULL
		  AND author_image != ''
		  AND author_deathdate IS NULL
		  AND substr(author_birthdate, 6, 5) = strftime('%m-%d', 'now')
		ORDER BY author_lastname, author_canonical`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*BirthdayAuthor
	for rows.Next() {
		var a BirthdayAuthor
		var canonical, birthdate, image sql.NullString
		if err := rows.Scan(&a.AuthorID, &canonical, &birthdate, &image); err != nil {
			continue
		}
		a.Canonical = canonical.String
		a.BirthDate = birthdate.String
		a.ImageURL = ISFDBHostCorrection(image.String, "")
		results = append(results, &a)
	}
	return results, rows.Err()
}

// IndexHandler serves /index.cgi — the ISFDB home page.
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	HTMLheader(w, "")
	PrintNavbar(w, "frontpage", "", "")

	// ── Search bar ────────────────────────────────────────────────────────
	fmt.Fprintln(w, `<div id="homepage_search">`)
	fmt.Fprintln(w, `<h1>The Desktop ISFDB</h1>`)
	fmt.Fprintf(w, "<form method=\"get\" action=\"%s://%s/se.cgi\" id=\"homepage_form\" onsubmit=\"return homepageSubmit(this)\">\n", PROTOCOL, HTMLHOST)
	fmt.Fprintln(w, `<div class="homepage_search_row">`)
	fmt.Fprintf(w, "<select name=\"type\" id=\"homepage_type\" class=\"homepage_search_select\" onchange=\"homepageTypeChange(this, '%s://%s/adv_search_menu.cgi')\">\n", PROTOCOL, HTMLHOST)
	for _, opt := range searchTypeOptions {
		fmt.Fprintf(w, "<option>%s</option>\n", opt)
	}
	fmt.Fprintf(w, "<option value=\"__advsearch__\">Advanced Search</option>\n")
	fmt.Fprintln(w, `</select>`)
	fmt.Fprintln(w, `<input name="arg" id="homepage_search_arg" class="homepage_search_input" placeholder="Search the ISFDB…" autofocus>`)
	fmt.Fprintln(w, `<input value="Search" type="submit" class="homepage_search_button">`)
	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `</form>`)
	fmt.Fprintln(w, `<script>`)
	fmt.Fprintln(w, `function homepageTypeChange(sel, advUrl) {`)
	fmt.Fprintln(w, `  if (sel.value === '__advsearch__') { window.location.href = advUrl; }`)
	fmt.Fprintln(w, `}`)
	fmt.Fprintln(w, `function homepageSubmit(form) {`)
	fmt.Fprintln(w, `  if (form.type.value === '__advsearch__') { return false; }`)
	fmt.Fprintln(w, `  return true;`)
	fmt.Fprintln(w, `}`)
	fmt.Fprintln(w, `</script>`)
	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `<p>`)
	fmt.Fprintln(w, `<hr>`)

	// ── Update notice ─────────────────────────────────────────────────────
	if _, _, available, newDate, _ := Upd.Snapshot(); available {
		fmt.Fprintf(w, "<p class=\"bottomlinks\" style=\"color:darkorange;text-align:center\">"+
			"A new database version is available (%s). "+
			"<a href=\"/update.cgi\">Update now</a></p>\n", ISFDBText(newDate))
	}

	// ── Birthday authors ──────────────────────────────────────────────────
	authors, err := SQLGetTodaysBirthdayAuthors(DB)
	if err == nil && len(authors) > 0 {
		today := time.Now().Format("January 2")
		fmt.Fprintf(w, "<div id=\"homepage_birthdays\">\n")
		fmt.Fprintf(w, "<h3>Select living authors born on this day, %s</h3>\n", today)
		fmt.Fprintln(w, `<table cellspacing="6">`)

		// Row 1: photos
		fmt.Fprintln(w, `<tr>`)
		for _, a := range authors {
			fmt.Fprintf(w, "<td style=\"background-color:black\"><img src=\"%s\" alt=\"%s\" class=\"covermainpage\" style=\"width:135px\"></td>\n",
				a.ImageURL, ISFDBText(a.Canonical))
		}
		fmt.Fprintln(w, `</tr>`)

		// Row 2: names (with birth year and age)
		fmt.Fprintln(w, `<tr>`)
		for _, a := range authors {
			age := ""
			if len(a.BirthDate) >= 4 {
				if birthYear := parseInt(a.BirthDate[:4]); birthYear > 0 {
					age = fmt.Sprintf("%d", time.Now().Year()-birthYear)
				}
			}
			fmt.Fprintf(w, "<td><a href=\"/author.cgi?%d\">%s</a>", a.AuthorID, ISFDBText(a.Canonical))
			if age != "" {
				fmt.Fprintf(w, "<br><small>b. %s (age %s)</small>", ISFDBText(a.BirthDate[:4]), age)
			}
			fmt.Fprintln(w, `</td>`)
		}
		fmt.Fprintln(w, `</tr>`)

		fmt.Fprintln(w, `</table>`)
		// Link to the full calendar day page.
		fmt.Fprintf(w, "<p class=\"bottomlinks\"><a href=\"%s://%s/calendar_day.cgi?%d+%d\">View all authors born or who died on this day</a></p>\n",
			PROTOCOL, HTMLHOST, int(time.Now().Month()), time.Now().Day())
		fmt.Fprintln(w, `</div>`) // homepage_birthdays
	}

	// ── DB sync notice ────────────────────────────────────────────────────
	fmt.Fprintln(w, `<hr>`)
	fmt.Fprintln(w, `<div id="isfdb_notice">`)
	fmt.Fprintln(w, `<p>A community effort to catalog works of science fiction, fantasy, and horror.</p>`)
	if info, err := os.Stat(DBPath); err == nil {
		fmt.Fprintf(w, "<p>Database last updated: %s</p>\n",
			info.ModTime().Format("January 2, 2006"))
	}
	fmt.Fprintln(w, `</div>`)

	HTMLtrailer(w)
}

// parseInt parses a string to int, returning 0 on failure.
func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
