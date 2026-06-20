// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
)

// AyHandler serves /ay.cgi?<type_id>+<year> — all awards for one type and year.
//
// The URL may also be the legacy single-parameter form /ay.cgi?ZzYYYY where
// "Zz" is the two-character award_type_code (e.g. "HU1960" for the 1960 Hugo).
func AyHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)

	var at *AwardType
	var year int
	var err error

	switch len(params) {
	case 2:
		// Normal form: type_id + year
		typeID, e1 := strconv.Atoi(params[0])
		year, err = strconv.Atoi(params[1])
		if e1 != nil || err != nil || typeID <= 0 || year <= 0 {
			http.Error(w, "Invalid parameters", http.StatusBadRequest)
			return
		}
		at, err = SQLGetAwardTypeById(DB, typeID)
		if err != nil {
			http.Error(w, "Award type not found", http.StatusNotFound)
			log.Println(err)
			return
		}
	case 1:
		// Legacy form: two-char code + four-digit year, e.g. "HU1960"
		raw := params[0]
		if len(raw) < 3 {
			http.Error(w, "Invalid parameter", http.StatusBadRequest)
			return
		}
		code := raw[:2]
		year, err = strconv.Atoi(raw[2:])
		if err != nil || year <= 0 {
			http.Error(w, "Invalid award year", http.StatusBadRequest)
			return
		}
		at, err = SQLGetAwardTypeByCode(DB, code)
		if err != nil {
			http.Error(w, "Award type not found", http.StatusNotFound)
			log.Println(err)
			return
		}
	default:
		http.Error(w, "This page requires one or two parameters", http.StatusBadRequest)
		return
	}

	// Load all distinct years for this award type (for the year grid)
	allYears, err := SQLGetAwardYears(DB, at.AwardTypeID)
	if err != nil {
		log.Println(err)
	}

	// Load all awards for this type+year
	awardRows, err := SQLloadAwardsForYearType(DB, at.AwardTypeID, year)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	// Batch-load display data for all awards
	rawAwards := make([]*Award, len(awardRows))
	for i, row := range awardRows {
		rawAwards[i] = row.Award
	}
	displays, err := LoadAwardDisplayBatch(DB, rawAwards)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	pageTitle := fmt.Sprintf("%d %s", year, at.AwardTypeName.String)
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	RecordHistory("Award", pageTitle, r.URL.RequestURI())
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "award", "", "")

	// ── Year grid ────────────────────────────────────────────────────────────
	printAwardYearGrid(w, at, allYears, year)

	// ── Award table ──────────────────────────────────────────────────────────
	if len(displays) == 0 {
		fmt.Fprintf(w, "<h2>No awards available for %d</h2>\n", year)
	} else {
		printAwardsByCategory(w, awardRows, displays)
	}

	fmt.Fprintln(w, `</div>`) // main
	HTMLtrailer(w)
}

// printAwardYearGrid renders the decade-based year navigation grid.
// current is the year currently being displayed (shown bold, not linked).
func printAwardYearGrid(w io.Writer, at *AwardType, allYears []string, current int) {
	if len(allYears) == 0 {
		return
	}

	// Build a set of years that have awards
	yearSet := make(map[int]bool, len(allYears))
	for _, y := range allYears {
		if len(y) >= 4 {
			if n, err := strconv.Atoi(y[:4]); err == nil {
				yearSet[n] = true
			}
		}
	}

	// Group by decade
	decadeSet := make(map[int]bool)
	for y := range yearSet {
		decadeSet[y/10] = true
	}
	var decades []int
	for d := range decadeSet {
		decades = append(decades, d)
	}
	sort.Ints(decades)

	fmt.Fprintln(w, `<div class="generic_centered_div">`)
	fmt.Fprintf(w, "<h3>Award Years for <a href=\"/awardtype.cgi?%d\">%s</a></h3>\n",
		at.AwardTypeID, ISFDBText(at.AwardTypeName.String))
	fmt.Fprintln(w, `<table class="generic_centered_table">`)

	for _, decade := range decades {
		fmt.Fprintln(w, `<tr align="center" class="generic_table_header">`)
		fmt.Fprintf(w, "<td>%d0's:</td>\n", decade)
		for i := 0; i < 10; i++ {
			y := decade*10 + i
			fmt.Fprintln(w, "<td>")
			if yearSet[y] {
				if y == current {
					fmt.Fprintf(w, "<b>%d</b>", y)
				} else {
					fmt.Fprintf(w, "<a href=\"/ay.cgi?%d+%d\">%d</a>",
						at.AwardTypeID, y, y)
				}
			} else {
				fmt.Fprint(w, "&nbsp;-&nbsp;")
			}
			fmt.Fprintln(w, "</td>")
		}
		fmt.Fprintln(w, "</tr>")
	}

	fmt.Fprintln(w, `</table>`)
	fmt.Fprintln(w, `</div>`)
}

// printAwardsByCategory renders the full award table, grouped by category.
// awardRows and displays are parallel slices in the same sort order.
func printAwardsByCategory(w io.Writer, awardRows []*AwardYearRow, displays []*AwardDisplay) {
	fmt.Fprintln(w, `<table>`)

	i := 0
	for i < len(awardRows) {
		// Identify the current category run
		catName := awardRows[i].CatName
		catID := 0
		if awardRows[i].Award.AwardCatID.Valid {
			catID = int(awardRows[i].Award.AwardCatID.Int32)
		}

		// Collect all displays for this category
		var catDisplays []*AwardDisplay
		for i < len(awardRows) && awardRows[i].CatName == catName {
			catDisplays = append(catDisplays, displays[i])
			i++
		}

		// Category header spanning 3 columns
		fmt.Fprintln(w, `<tr><td colspan=3> </td></tr>`)
		fmt.Fprintf(w,
			"<tr><td colspan=3><b><a href=\"/award_category.cgi?%d+0\">%s</a></b></td></tr>\n",
			catID, ISFDBText(catName))

		// Awards for this category, in level order (1 first, then higher, then specials)
		printOneCategoryAwards(w, catDisplays)
	}

	fmt.Fprintln(w, `</table>`)
}

// printOneCategoryAwards iterates levels 1-99 and prints each matching award,
// inserting a "special" header row whenever the level type changes.
func printOneCategoryAwards(w io.Writer, displays []*AwardDisplay) {
	lastLevel := 1
	for level := 1; level < 100; level++ {
		for _, d := range displays {
			lvl, _ := strconv.Atoi(d.AwardLevel)
			if lvl != level {
				continue
			}
			// Skip unrecognised special levels (>70 and not in the map)
			if level > 70 {
				if _, ok := specialAwards[d.AwardLevel]; !ok {
					continue
				}
			}
			// Insert a special-level header when we first enter a new special group
			if _, ok := specialAwards[d.AwardLevel]; ok && level != lastLevel {
				fmt.Fprintf(w,
					"<tr><td colspan=3><i>--- %s -------</i></td></tr>\n",
					ISFDBText(specialAwards[d.AwardLevel]))
			}
			printAwardYearRow(w, d)
			lastLevel = level
		}
	}
}

// printAwardYearRow renders one award row in the ay.cgi table (3 columns:
// level, title, authors).  This is the equivalent of awardClass.PrintOneAward.
func printAwardYearRow(w io.Writer, d *AwardDisplay) {
	fmt.Fprintln(w, "<tr>")

	// Column 1: level / place
	levelInt, _ := strconv.Atoi(d.AwardLevel)
	levelText := ""
	cssClass := ""
	if levelInt > 70 {
		levelText = "*"
	} else if d.TypePoll == "Yes" {
		levelText = d.AwardLevel
	} else {
		if levelInt == 1 {
			levelText = "Win"
			cssClass = "bold"
		} else {
			levelText = "Nomination"
		}
	}
	levelLink := fmt.Sprintf("<a href=\"/award_details.cgi?%d\">%s</a>", d.AwardID, ISFDBText(levelText))
	if cssClass != "" {
		levelLink = fmt.Sprintf("<a href=\"/award_details.cgi?%d\" class=\"%s\">%s</a>",
			d.AwardID, cssClass, ISFDBText(levelText))
	}
	fmt.Fprintf(w, "<td>%s</td>\n", levelLink)

	// Column 2: title (and optional IMDB link)
	fmt.Fprintln(w, "<td>")
	if d.AwardTitle == "untitled" || (d.TitleID == 0 && d.AwardTitle == "") {
		fmt.Fprint(w, "----")
	} else {
		printAwardTitle(w, d)
	}
	if d.AwardMovie != "" {
		fmt.Fprintf(w, " (<a href=\"https://www.imdb.com/title/%s/\" target=\"_blank\">IMDB</a>)",
			ISFDBText(d.AwardMovie))
	}
	fmt.Fprintln(w, "</td>")

	// Column 3: author(s)
	fmt.Fprintln(w, "<td>")
	printAwardAuthors(w, d)
	fmt.Fprintln(w, "</td>")

	fmt.Fprintln(w, "</tr>")
}
