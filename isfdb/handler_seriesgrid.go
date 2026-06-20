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
	"strings"
)

var gridMonthNames = [13]string{
	"", "Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

// gridMonths is the column order: Jan–Dec then "no month" (00).
var gridMonths = []string{
	"01", "02", "03", "04", "05", "06",
	"07", "08", "09", "10", "11", "12", "00",
}

// gridPub is one publication cell in the issue grid.
type gridPub struct {
	Pub         *Pub
	VerifStatus int // 0=unverified, 1=primary, 2=secondary
}

// gridData is the nested year→month→day→[]pub map.
type gridData map[string]map[string]map[string][]*gridPub

// SeriesGridHandler serves /seriesgrid.cgi?<series_id>+<display_order>.
// display_order=1 (default) = latest year first; 0 = earliest year first.
func SeriesGridHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 1 {
		http.Error(w, "Series ID required", http.StatusBadRequest)
		return
	}
	seriesID, err := strconv.Atoi(params[0])
	if err != nil || seriesID <= 0 {
		http.Error(w, "Invalid series ID", http.StatusBadRequest)
		return
	}
	displayOrder := 1 // default: latest year first
	if len(params) >= 2 {
		if v, err := strconv.Atoi(params[1]); err == nil && (v == 0 || v == 1) {
			displayOrder = v
		}
	}

	ser, err := SQLLoadSeries(DB, seriesID)
	if err != nil {
		http.Error(w, "Series not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	// Load parent series name for "Sub-series of:" link
	parentID := 0
	parentName := ""
	if ser.SeriesParent.Valid && ser.SeriesParent.Int32 > 0 {
		parentID = int(ser.SeriesParent.Int32)
		if p, err := SQLLoadSeries(DB, parentID); err == nil {
			parentName = p.SeriesTitle
		}
	}

	// Load note and webpages
	noteText := ""
	if ser.NoteID.Valid {
		if n, err := SQLgetNotes(DB, int(ser.NoteID.Int32)); err == nil {
			noteText = n
		}
	}
	webpages, _ := SQLloadSeriesWebpages(DB, seriesID)
	domains, _ := SQLLoadRecognizedDomains(DB)

	// ── Build grid data ──────────────────────────────────────────────────
	grid := make(gridData)
	formats := map[string]int{}

	// Load root series + all child series
	seriesIDs := []int{seriesID}
	children, err := SQLFindSeriesChildren(DB, seriesID)
	if err != nil {
		log.Println(err)
	}
	seriesIDs = append(seriesIDs, children...)

	// Fetch all pubs for all series in 3 queries instead of N×3.
	allPubs, err := SQLGetPubsForSeriesIDs(DB, seriesIDs)
	if err != nil {
		log.Println(err)
	}

	// Collect pub IDs and build per-pub date parts for the grid.
	type pubEntry struct {
		pub   *Pub
		year  string
		month string
		day   string
	}
	var allPubIDs []int
	var pubEntries []pubEntry
	for _, p := range allPubs {
		yr := "0000"
		mo := "00"
		dy := "00"
		if p.PubYear.Valid && len(p.PubYear.String) >= 4 {
			yr = p.PubYear.String[:4]
			if yr == "" {
				yr = "0000"
			}
			if len(p.PubYear.String) >= 7 {
				mo = p.PubYear.String[5:7]
			}
			if len(p.PubYear.String) >= 10 {
				dy = p.PubYear.String[8:10]
			}
		}
		if p.PubPType.Valid && p.PubPType.String != "" {
			formats[p.PubPType.String]++
		}
		allPubIDs = append(allPubIDs, p.PubID)
		pubEntries = append(pubEntries, pubEntry{p, yr, mo, dy})
	}

	// Batch-load verification statuses
	verifStatus, err := SQLBatchVerificationStatus(DB, allPubIDs)
	if err != nil {
		log.Println(err)
		verifStatus = map[int]int{}
	}

	// Populate the grid (deduplicate pub IDs across series)
	seenPubs := map[int]bool{}
	for _, e := range pubEntries {
		if seenPubs[e.pub.PubID] {
			continue
		}
		seenPubs[e.pub.PubID] = true

		if grid[e.year] == nil {
			grid[e.year] = make(map[string]map[string][]*gridPub)
		}
		if grid[e.year][e.month] == nil {
			grid[e.year][e.month] = make(map[string][]*gridPub)
		}
		grid[e.year][e.month][e.day] = append(
			grid[e.year][e.month][e.day],
			&gridPub{e.pub, verifStatus[e.pub.PubID]},
		)
	}

	// Determine default (most common) format
	defaultFormat := ""
	highestCount := 0
	for f, c := range formats {
		if c > highestCount {
			defaultFormat = f
			highestCount = c
		}
	}

	pageTitle := "Issue Grid: " + ser.SeriesTitle
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "seriesgrid", "", "")

	// ── ContentBox 1: series metadata ────────────────────────────────────
	printSeriesGridMetadata(w, ser, seriesID, parentID, parentName,
		webpages, domains, noteText)

	// ── ContentBox 2: format/legend/links ────────────────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintf(w, "<b>Format:</b> %s", ISFDBText(ISFDBPubFormat(defaultFormat)))
	if len(formats) > 1 {
		fmt.Fprint(w, " (unless indicated otherwise)")
	}
	fmt.Fprintln(w, "<br>")
	fmt.Fprintln(w, "<b>Legend:</b> Unverified issues are gold, secondary verifications are light blue.")

	fmt.Fprintf(w, `<p class="textindent"> <a href="/pe.cgi?%d">View this magazine as a series</a> &#8226; `, seriesID)
	if displayOrder == 1 {
		fmt.Fprintf(w, "<a href=\"/seriesgrid.cgi?%d+0\">Show earliest year first</a>", seriesID)
	} else {
		fmt.Fprintf(w, "<a href=\"/seriesgrid.cgi?%d+1\">Show last year first</a>", seriesID)
	}
	fmt.Fprintln(w)

	// ── Grid table ───────────────────────────────────────────────────────
	printSeriesGridTable(w, grid, defaultFormat, displayOrder)

	fmt.Fprintln(w, `</div>`) // ContentBox 2
	fmt.Fprintln(w, `</div>`) // content
	HTMLtrailer(w)
}

// printSeriesGridMetadata renders ContentBox 1 — the series metadata ul.
func printSeriesGridMetadata(w io.Writer, ser *Series, seriesID, parentID int,
	parentName string, webpages []string, domains []RecognizedDomain, noteText string) {

	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<ul>`)
	fmt.Fprintf(w, "<li><b>Series: </b>%s\n", ISFDBText(ser.SeriesTitle))

	if parentID > 0 && parentName != "" {
		fmt.Fprintf(w, "<li><b>Sub-series of:</b> <a href=\"/pe.cgi?%d\">%s</a> <a href=\"/seriesgrid.cgi?%d\">(View Issue Grid)</a>\n",
			parentID, ISFDBText(parentName), parentID)
	}

	if len(webpages) > 0 {
		PrintWebPages(w, webpages, "<li>", domains)
	}

	if noteText != "" {
		fmt.Fprintln(w, "<li>")
		fmt.Fprintln(w, FormatNote(noteText, "Note", "short", seriesID, "Series", false))
	}

	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `</div>`)
}

// printSeriesGridTable renders the year × month grid table.
func printSeriesGridTable(w io.Writer, grid gridData, defaultFormat string, displayOrder int) {
	// Collect and sort years
	years := make([]string, 0, len(grid))
	for y := range grid {
		years = append(years, y)
	}
	sort.Slice(years, func(i, j int) bool {
		if displayOrder == 1 {
			return years[i] > years[j] // descending
		}
		return years[i] < years[j] // ascending
	})

	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintln(w, `<tr><th>&nbsp;</th>`)
	for m := 1; m <= 12; m++ {
		fmt.Fprintf(w, "<th>%s</th>", gridMonthNames[m])
	}
	fmt.Fprintln(w, `<th>No month</th></tr>`)

	for i, year := range years {
		bgcolor := (i % 2) + 1
		printSeriesGridYear(w, grid, year, defaultFormat, bgcolor)
	}

	fmt.Fprintln(w, `</table>`)
}

// printSeriesGridYear renders one row of the grid (one year, all months).
func printSeriesGridYear(w io.Writer, grid gridData, year, defaultFormat string, bgcolor int) {
	yearDisplay := year
	switch year {
	case "0000":
		yearDisplay = "unknown"
	case "8888":
		yearDisplay = "unpublished"
	case "9999":
		yearDisplay = "forthcoming"
	}
	fmt.Fprintf(w, "<tr align=center class=\"table%d\">\n", bgcolor)
	fmt.Fprintf(w, "<th class=\"year\">%s</th>\n", yearDisplay)

	for _, month := range gridMonths {
		monthData := grid[year][month]
		if len(monthData) == 0 {
			fmt.Fprintln(w, "<td>-</td>")
			continue
		}

		// Sort days within the month
		days := make([]string, 0, len(monthData))
		for d := range monthData {
			days = append(days, d)
		}
		sort.Strings(days)

		fmt.Fprintln(w, `<td class="seriesgridinner"><table class="seriesgridinner">`)
		for _, day := range days {
			pubs := monthData[day]
			// Sort pubs within a day by pub_id for stability
			sort.Slice(pubs, func(i, j int) bool {
				return pubs[i].Pub.PubID < pubs[j].Pub.PubID
			})
			for _, gp := range pubs {
				title := processGridTitle(gp.Pub.PubTitle.String, year)
				cssClass := "notverified"
				if gp.VerifStatus == 1 {
					cssClass = "verifiedprimary"
				} else if gp.VerifStatus == 2 {
					cssClass = "verifiedsecondary"
				}
				formatTag := ""
				if gp.Pub.PubPType.Valid && gp.Pub.PubPType.String != defaultFormat {
					formatTag = fmt.Sprintf(" <small>[%s]</small>", ISFDBText(gp.Pub.PubPType.String))
				}
				fmt.Fprintln(w, `<tr class="seriesgridinner">`)
				fmt.Fprintf(w, "<td class=\"%s\"><a href=\"/pub.cgi?%d\">%s</a>%s</td>\n",
					cssClass, gp.Pub.PubID, ISFDBText(title), formatTag)
				fmt.Fprintln(w, `</tr>`)
			}
		}
		fmt.Fprintln(w, `</table></td>`)
	}
	fmt.Fprintln(w, `</tr>`)
}

// processGridTitle shortens a publication title for display in the grid cell.
// It mirrors Python's SeriesGrid.PrintOneYear title processing logic.
func processGridTitle(title, year string) string {
	original := title
	// Strip everything up to and including the first comma
	if i := strings.Index(title, ","); i != -1 {
		title = title[i+1:]
	}
	// If the last space-delimited word equals the year, remove it
	words := strings.Fields(title)
	if len(words) > 0 && words[len(words)-1] == year {
		title = strings.TrimRight(title[:len(title)-4], " ")
	}
	// Strip leading/trailing space, comma, period
	title = stripGridPunctuation(title)
	if title == "" {
		return original
	}
	return title
}

// stripGridPunctuation removes leading and trailing ' ', ',', '.' from a string.
func stripGridPunctuation(s string) string {
	const chars = " ,."
	// Leading
	i := 0
	for i < len(s) && strings.ContainsRune(chars, rune(s[i])) {
		i++
	}
	s = s[i:]
	// Trailing
	j := len(s)
	for j > 0 && strings.ContainsRune(chars, rune(s[j-1])) {
		j--
	}
	s = s[:j]
	// Single-character edge case from Python
	if s == " " || s == "," || s == "." {
		return ""
	}
	return s
}
