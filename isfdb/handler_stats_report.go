// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

var statsHeaders = map[int]string{
	5:  "Titles by Year of First Publication",
	7:  "Titles by Author Age",
	8:  "Percent of Titles in Series by Year",
	16: "Oldest Living Authors",
	17: "Oldest Non-Living Authors",
	18: "Youngest Living Authors",
	19: "Youngest Non-Living Authors",
}

// StatsReportHandler serves /stats.cgi?N — individual statistical reports.
func StatsReportHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "Invalid report number", http.StatusBadRequest)
		return
	}
	header, ok := statsHeaders[id]
	if !ok {
		http.Error(w, "Invalid report number", http.StatusBadRequest)
		return
	}

	HTMLheader(w, header)
	PrintNavbar(w, "stats", "", "")
	fmt.Fprintln(w, "<h3>This report is generated on demand</h3>")

	switch id {
	case 5:
		statsTitlesByYear(w)
	case 7:
		statsTitlesByAuthorAge(w)
	case 8:
		statsSeriesByYear(w)
	case 16:
		statsAuthorAges(w, 16)
	case 17:
		statsAuthorAges(w, 17)
	case 18:
		statsAuthorAges(w, 18)
	case 19:
		statsAuthorAges(w, 19)
	}

	HTMLtrailer(w)
}

// ── SVG chart helpers ────────────────────────────────────────────────────────

type svgSeries struct {
	color string
	label string
	// points indexed by x position (year-startX or age)
	data map[int]float64
}

// renderSVGChart outputs an SVG line chart matching the original Python outputGraph layout.
// startX is the first x value (year or age), numX is the number of x positions.
// maxY is the chart ceiling; if 0, it is derived from the data.
// series is a list of coloured data lines.
func renderSVGChart(w http.ResponseWriter, startX, numX int, maxY float64, series []svgSeries) {
	const (
		height  = 200
		xscale  = 6
		xoffset = 15
		yoffset = 10
	)

	// Derive maxY from data if not provided.
	if maxY == 0 {
		for _, s := range series {
			for _, v := range s.data {
				if v > maxY {
					maxY = v
				}
			}
		}
	}
	if maxY == 0 {
		maxY = 1
	}

	svgW := xoffset + 40 + numX*xscale
	svgH := height + 30 + yoffset

	fmt.Fprintf(w, "<svg width=\"%d\" height=\"%d\" version=\"1.1\" xmlns=\"http://www.w3.org/2000/svg\">\n", svgW, svgH)

	// Horizontal grid lines and value labels (5 lines: 0, 50, 100, 150, 200 px from top).
	increment := maxY / 4
	for i := 0; i <= 4; i++ {
		y := i * 50
		value := maxY - float64(i)*increment
		fmt.Fprintf(w, "<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" class=\"svg1\"/>\n",
			xoffset, y+yoffset, xoffset+5+numX*xscale, y+yoffset)
		fmt.Fprintf(w, "<text x=\"%d\" y=\"%d\" font-size=\"10\">%d</text>\n",
			xoffset+10+numX*xscale, y+5+yoffset, int(math.Round(value)))
	}

	// Vertical grid lines and x-axis labels every 10 units.
	for x := 0; x < numX; x += 10 {
		px := xoffset + xscale*x
		fmt.Fprintf(w, "<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" class=\"svg1\"/>\n",
			px, yoffset, px, height+10+yoffset)
		fmt.Fprintf(w, "<text x=\"%d\" y=\"%d\" font-size=\"10\">%d</text>\n",
			px-12, height+20+yoffset, x+startX)
	}

	// Data lines.
	yscale := float64(height) / maxY
	for _, s := range series {
		var lastX, lastY int
		first := true
		for xi := 0; xi < numX; xi++ {
			v := s.data[xi]
			px := xoffset + xscale*xi
			py := yoffset + int(yscale*float64(maxY-v))
			if !first {
				fmt.Fprintf(w, "<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" class=\"svg%s\"/>\n",
					lastX, lastY, px, py, s.color)
			}
			lastX, lastY = px, py
			first = false
		}
	}

	// Legend (if more than one series).
	if len(series) > 1 {
		lx := xoffset
		ly := svgH - 5
		for _, s := range series {
			fmt.Fprintf(w, "<line x1=\"%d\" y1=\"%d\" x2=\"%d\" y2=\"%d\" class=\"svg%s\"/>\n",
				lx, ly, lx+20, ly, s.color)
			fmt.Fprintf(w, "<text x=\"%d\" y=\"%d\" font-size=\"10\">%s</text>\n",
				lx+22, ly+4, s.label)
			lx += 80
		}
	}

	fmt.Fprintln(w, "</svg>")
}

// ── Stat 5: Titles by Year ───────────────────────────────────────────────────

func statsTitlesByYear(w http.ResponseWriter) {
	endYear := time.Now().Year() - 1
	startYear := 1900
	numYears := endYear - startYear + 1

	types := []struct {
		ttype string
		label string
		color string
	}{
		{"NOVEL", "Novels", "black"},
		{"SHORTFICTION", "Short Fiction", "blue"},
		{"POEM", "Poems", "green"},
		{"REVIEW", "Reviews", "red"},
	}

	var series []svgSeries
	for _, t := range types {
		rows, err := DB.Query(`
			SELECT CAST(substr(title_copyright,1,4) AS INTEGER) as yr, COUNT(*)
			FROM titles
			WHERE title_ttype = ? AND title_parent = 0
			AND CAST(substr(title_copyright,1,4) AS INTEGER) > ?
			AND CAST(substr(title_copyright,1,4) AS INTEGER) < ?
			GROUP BY yr ORDER BY yr`, t.ttype, startYear-1, endYear+1)
		if err != nil {
			continue
		}
		data := map[int]float64{}
		for rows.Next() {
			var yr, cnt int
			if err := rows.Scan(&yr, &cnt); err == nil {
				data[yr-startYear] = float64(cnt)
			}
		}
		rows.Close()
		series = append(series, svgSeries{color: t.color, label: t.label, data: data})
	}

	fmt.Fprintf(w, "<h3>Legend: %s</h3>\n", legendText(types[0].color, types[0].label,
		types[1].color, types[1].label, types[2].color, types[2].label, types[3].color, types[3].label))
	renderSVGChart(w, startYear, numYears, 0, series)
}

func legendText(args ...string) string {
	out := ""
	for i := 0; i+1 < len(args); i += 2 {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("<span style=\"color:%s\">%s</span>", args[i], args[i+1])
	}
	return out
}

// ── Stat 7: Titles by Author Age ─────────────────────────────────────────────

func statsTitlesByAuthorAge(w http.ResponseWriter) {
	charts := []struct {
		ttype string
		kind  string // "all" or "first"
		label string
	}{
		{"NOVEL", "all", "All Novels"},
		{"NOVEL", "first", "First Novels"},
		{"SHORTFICTION", "all", "All Short Fiction"},
		{"SHORTFICTION", "first", "First Short Fiction"},
	}

	for _, c := range charts {
		fmt.Fprintf(w, "<h3>%s</h3>\n", c.label)
		data := map[int]float64{}

		if c.kind == "all" {
			rows, err := DB.Query(`
				SELECT CAST(substr(t.title_copyright,1,4) AS INTEGER) - CAST(substr(a.author_birthdate,1,4) AS INTEGER) AS age,
				       COUNT(t.title_id)
				FROM titles t
				JOIN canonical_author ca ON t.title_id = ca.title_id
				JOIN authors a ON ca.author_id = a.author_id
				WHERE t.title_ttype = ?
				AND t.title_parent = 0
				AND a.author_birthdate IS NOT NULL
				AND substr(a.author_birthdate,1,4) != '0000'
				AND CAST(substr(t.title_copyright,1,4) AS INTEGER) < 8888
				AND CAST(substr(t.title_copyright,1,4) AS INTEGER) > 0
				AND CAST(substr(t.title_copyright,1,4) AS INTEGER) - CAST(substr(a.author_birthdate,1,4) AS INTEGER) > 0
				AND CAST(substr(t.title_copyright,1,4) AS INTEGER) - CAST(substr(a.author_birthdate,1,4) AS INTEGER) < 101
				GROUP BY age ORDER BY age`, c.ttype)
			if err == nil {
				for rows.Next() {
					var age, cnt int
					if err := rows.Scan(&age, &cnt); err == nil {
						data[age] = float64(cnt)
					}
				}
				rows.Close()
			}
		} else {
			// First publication of this type per author.
			rows, err := DB.Query(`
				SELECT v.first_year - v.birth_year AS age, COUNT(*) AS cnt
				FROM (
					SELECT CAST(substr(a.author_birthdate,1,4) AS INTEGER) AS birth_year,
					       (SELECT MIN(CAST(substr(t.title_copyright,1,4) AS INTEGER))
					        FROM titles t
					        JOIN canonical_author ca2 ON t.title_id = ca2.title_id
					        WHERE t.title_copyright != '0000-00-00'
					        AND t.title_ttype = ?
					        AND t.title_parent = 0
					        AND ca2.author_id = a.author_id
					        AND ca2.ca_status = 1) AS first_year
					FROM authors a
					WHERE a.author_birthdate IS NOT NULL
					AND substr(a.author_birthdate,1,4) != '0000'
				) v
				WHERE v.first_year IS NOT NULL
				AND v.first_year - v.birth_year > 0
				AND v.first_year - v.birth_year < 101
				GROUP BY age ORDER BY age`, c.ttype)
			if err == nil {
				for rows.Next() {
					var age, cnt int
					if err := rows.Scan(&age, &cnt); err == nil {
						data[age] = float64(cnt)
					}
				}
				rows.Close()
			}
		}

		renderSVGChart(w, 0, 101, 0, []svgSeries{{color: "black", label: c.label, data: data}})
	}
}

// ── Stat 8: Percent of Titles in Series by Year ───────────────────────────────

func statsSeriesByYear(w http.ResponseWriter) {
	endYear := time.Now().Year()
	startYear := 1900
	numYears := endYear - startYear + 1

	fmt.Fprintln(w, "<h3>Legend: <span style=\"color:red\">Novels</span>, <span style=\"color:blue\">Short Fiction</span></h3>")

	types := []struct {
		ttype string
		color string
	}{
		{"NOVEL", "red"},
		{"SHORTFICTION", "blue"},
	}

	var series []svgSeries
	for _, t := range types {
		rows, err := DB.Query(`
			SELECT CAST(substr(title_copyright,1,4) AS INTEGER) AS yr,
			       COUNT(*) AS total,
			       SUM(CASE WHEN series_id IS NOT NULL THEN 1 ELSE 0 END) AS in_series
			FROM titles
			WHERE title_ttype = ?
			AND title_parent = 0
			AND CAST(substr(title_copyright,1,4) AS INTEGER) > ?
			AND CAST(substr(title_copyright,1,4) AS INTEGER) < ?
			GROUP BY yr ORDER BY yr`, t.ttype, startYear-1, endYear+1)
		if err != nil {
			continue
		}
		data := map[int]float64{}
		for rows.Next() {
			var yr, total, inSeries int
			if err := rows.Scan(&yr, &total, &inSeries); err == nil && total > 0 {
				data[yr-startYear] = math.Round(float64(inSeries) * 100 / float64(total))
			}
		}
		rows.Close()
		series = append(series, svgSeries{color: t.color, data: data})
	}

	renderSVGChart(w, startYear, numYears, 100, series)
}

// ── Stats 16-19: Author age tables ───────────────────────────────────────────

func statsAuthorAges(w http.ResponseWriter, reportID int) {
	currentYear := time.Now().Year()

	var note, query string
	var headers [3]string

	switch reportID {
	case 16:
		note = "Authors whose year of birth is between 85 and 116 years in the past and who do not have a year of death on file."
		headers = [3]string{"Age", "Date of Birth", "Author"}
		query = fmt.Sprintf(`
			SELECT %d - CAST(substr(author_birthdate,1,4) AS INTEGER) AS age,
			       author_canonical, author_birthdate, author_id
			FROM authors
			WHERE author_birthdate IS NOT NULL
			AND substr(author_birthdate,1,4) != '0000'
			AND (author_deathdate IS NULL OR author_deathdate = '0000-00-00' OR author_deathdate = '')
			AND %d - CAST(substr(author_birthdate,1,4) AS INTEGER) > 84
			AND %d - CAST(substr(author_birthdate,1,4) AS INTEGER) < 117
			ORDER BY author_birthdate`, currentYear, currentYear, currentYear)
	case 17:
		note = "Authors whose year of birth is between 85 and 116 years in the past and who have a known year of death on file."
		headers = [3]string{"Age", "Date of Birth", "Author"}
		query = `
			SELECT CAST(substr(author_deathdate,1,4) AS INTEGER) - CAST(substr(author_birthdate,1,4) AS INTEGER) AS age,
			       author_canonical, author_birthdate, author_id
			FROM authors
			WHERE author_birthdate IS NOT NULL
			AND author_deathdate IS NOT NULL
			AND substr(author_birthdate,1,4) != '0000'
			AND substr(author_deathdate,1,4) != '0000'
			AND CAST(substr(author_deathdate,1,4) AS INTEGER) - CAST(substr(author_birthdate,1,4) AS INTEGER) > 84
			ORDER BY age DESC`
	case 18:
		headers = [3]string{"Age", "Date of Birth", "Author"}
		query = fmt.Sprintf(`
			SELECT %d - CAST(substr(author_birthdate,1,4) AS INTEGER) AS age,
			       author_canonical, author_birthdate, author_id
			FROM authors
			WHERE author_birthdate IS NOT NULL
			AND substr(author_birthdate,1,4) != '0000'
			AND (author_deathdate IS NULL OR author_deathdate = '0000-00-00' OR author_deathdate = '')
			AND %d - CAST(substr(author_birthdate,1,4) AS INTEGER) < 40
			ORDER BY author_birthdate DESC`, currentYear, currentYear)
	case 19:
		headers = [3]string{"Age", "Date of Birth", "Author"}
		query = `
			SELECT CAST(substr(author_deathdate,1,4) AS INTEGER) - CAST(substr(author_birthdate,1,4) AS INTEGER) AS age,
			       author_canonical, author_birthdate, author_id
			FROM authors
			WHERE author_birthdate IS NOT NULL
			AND author_deathdate IS NOT NULL
			AND substr(author_birthdate,1,4) != '0000'
			AND substr(author_deathdate,1,4) != '0000'
			AND CAST(substr(author_deathdate,1,4) AS INTEGER) - CAST(substr(author_birthdate,1,4) AS INTEGER) < 40
			AND CAST(substr(author_deathdate,1,4) AS INTEGER) - CAST(substr(author_birthdate,1,4) AS INTEGER) > 0
			ORDER BY age`
	}

	if note != "" {
		fmt.Fprintf(w, "<p><b>Note:</b> %s</p>\n", note)
	}

	rows, err := DB.Query(query)
	if err != nil {
		fmt.Fprintf(w, "<p>Database error: %s</p>", err)
		return
	}
	defer rows.Close()

	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintf(w, "<tr><th>%s</th><th>%s</th><th>%s</th></tr>\n",
		headers[0], headers[1], headers[2])

	bgcolor := 0
	for rows.Next() {
		var age, authorID int
		var name, birthdate string
		if err := rows.Scan(&age, &name, &birthdate, &authorID); err != nil {
			continue
		}
		fmt.Fprintf(w, "<tr class=\"table%d\">\n", bgcolor+1)
		fmt.Fprintf(w, "<td>%d</td>\n", age)
		fmt.Fprintf(w, "<td>%s</td>\n", birthdate)
		fmt.Fprintf(w, "<td><a href=\"%s://%s/author.cgi?%d\">%s</a></td>\n",
			PROTOCOL, HTMLHOST, authorID, ISFDBText(name))
		fmt.Fprintln(w, "</tr>")
		bgcolor ^= 1
	}
	fmt.Fprintln(w, "</table>")
}
