// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

var monthNames = [13]string{
	"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// daysInMonth returns the number of days in a given month (using year 2000
// as the reference year for leap-year handling of February).
func daysInMonth(month int) int {
	// Day 0 of the next month = last day of this month.
	return time.Date(2000, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

// CalendarMenuHandler serves /calendar_menu.cgi — a 4×3 grid of all 12 months.
func CalendarMenuHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	curMonth := int(now.Month())
	curDay := now.Day()

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, "SF Calendar")
	PrintNavbar(w, "calendar_menu", "", "")

	fmt.Fprintln(w, `<table class="calendar_table">`)
	month := 0
	for row := 0; row < 4; row++ {
		fmt.Fprintln(w, `<tr>`)
		for col := 0; col < 3; col++ {
			month++
			fmt.Fprintln(w, `<td>`)
			printCalendarMonth(w, month, daysInMonth(month), curMonth, curDay)
			fmt.Fprintln(w, `</td>`)
		}
		fmt.Fprintln(w, `</tr>`)
	}
	fmt.Fprintln(w, `</table>`)

	fmt.Fprintln(w, `</div>`) // main
	HTMLtrailer(w)
}

func printCalendarMonth(w http.ResponseWriter, month, numDays, curMonth, curDay int) {
	fmt.Fprintln(w, `<table class="calendar_row">`)
	// Month header
	fmt.Fprintln(w, `<tr>`)
	fmt.Fprintf(w, "<th colspan=\"10\" class=\"calendar\">%s</th>\n", monthNames[month])
	fmt.Fprintln(w, `</tr>`)

	// Three rows of 10 days + 1 overflow cell
	day := 0
	for decade := 0; decade < 3; decade++ {
		fmt.Fprintln(w, `<tr>`)
		for cell := 0; cell < 10; cell++ {
			day++
			fmt.Fprintln(w, `<td>`)
			if day <= numDays {
				printCalendarDay(w, month, day, curMonth, curDay)
			} else {
				fmt.Fprint(w, "&nbsp;")
			}
			fmt.Fprintln(w, `</td>`)
		}
		// 11th cell: day 31 appears only in the last decade row
		fmt.Fprintln(w, `<td>`)
		if decade == 2 && numDays == 31 {
			printCalendarDay(w, month, 31, curMonth, curDay)
		} else {
			fmt.Fprint(w, "&nbsp;")
		}
		fmt.Fprintln(w, `</td>`)
		fmt.Fprintln(w, `</tr>`)
	}

	fmt.Fprintln(w, `</table>`)
}

func printCalendarDay(w http.ResponseWriter, month, day, curMonth, curDay int) {
	extra := ""
	if month == curMonth && day == curDay {
		extra = ` class="inverted"`
	}
	fmt.Fprintf(w, "<a href=\"/calendar_day.cgi?%d+%d\" dir=\"ltr\"%s>%d</a>\n", month, day, extra, day)
}

// ── calendar_day.cgi ────────────────────────────────────────────────────────

type calendarAuthor struct {
	AuthorID    int
	Canonical   string
	BirthDate   string
	DeathDate   string
}

func sqlCalendarAuthors(db *sql.DB, month, day int, dateField string) ([]*calendarAuthor, error) {
	query := fmt.Sprintf(`
		SELECT author_id, author_canonical, author_birthdate, author_deathdate
		FROM authors
		WHERE CAST(substr(%s, 6, 2) AS INTEGER) = ?
		  AND CAST(substr(%s, 9, 2) AS INTEGER) = ?
		ORDER BY author_birthdate`, dateField, dateField)
	rows, err := db.Query(query, month, day)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*calendarAuthor
	for rows.Next() {
		var a calendarAuthor
		var birth, death sql.NullString
		if err := rows.Scan(&a.AuthorID, &a.Canonical, &birth, &death); err != nil {
			continue
		}
		a.BirthDate = birth.String
		a.DeathDate = death.String
		result = append(result, &a)
	}
	return result, rows.Err()
}

func calendarLifeSpan(a *calendarAuthor) string {
	birth := ""
	if len(a.BirthDate) >= 4 {
		birth = a.BirthDate[:4]
		if birth == "0000" {
			birth = "unknown"
		}
	}
	death := ""
	if len(a.DeathDate) >= 4 {
		death = a.DeathDate[:4]
		if death == "0000" {
			death = "unknown"
		}
	}
	if death != "" {
		return fmt.Sprintf(" (%s-%s)", birth, death)
	}
	return fmt.Sprintf(" (%s)", birth)
}

// CalendarDayHandler serves /calendar_day.cgi?<month>+<day>
func CalendarDayHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) != 2 {
		http.Error(w, "Expected month+day parameters", http.StatusBadRequest)
		return
	}
	month, err1 := strconv.Atoi(params[0])
	day, err2 := strconv.Atoi(params[1])
	if err1 != nil || err2 != nil || month < 1 || month > 12 || day < 1 || day > daysInMonth(month) {
		http.Error(w, "Invalid date", http.StatusBadRequest)
		return
	}

	born, err := sqlCalendarAuthors(DB, month, day, "author_birthdate")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	died, err := sqlCalendarAuthors(DB, month, day, "author_deathdate")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	pageTitle := fmt.Sprintf("On This Day in SF - %s %d", monthNames[month], day)
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "calendar_day", "", "")

	fmt.Fprintln(w, `<table class="mainauthors">`)
	fmt.Fprintln(w, `<tr>`)
	fmt.Fprintln(w, `<th class="dividerrow">Authors Born On This Day:</th>`)
	fmt.Fprintln(w, `<th class="dividerrow">Authors Who Died On This Day:</th>`)
	fmt.Fprintln(w, `</tr>`)
	fmt.Fprintln(w, `<tr>`)
	printCalendarAuthorList(w, born)
	printCalendarAuthorList(w, died)
	fmt.Fprintln(w, `</tr>`)
	fmt.Fprintln(w, `</table>`)

	fmt.Fprintln(w, `</div>`) // main
	HTMLtrailer(w)
}

func printCalendarAuthorList(w http.ResponseWriter, authors []*calendarAuthor) {
	fmt.Fprintln(w, `<td>`)
	if len(authors) > 0 {
		fmt.Fprintln(w, `<ul>`)
		for _, a := range authors {
			fmt.Fprintf(w, "<li><a href=\"/ea.cgi?%d\">%s</a>%s\n",
				a.AuthorID, ISFDBText(a.Canonical), calendarLifeSpan(a))
		}
		fmt.Fprintln(w, `</ul>`)
	}
	fmt.Fprintln(w, `</td>`)
}
