// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

// PrintAwardTable renders a list of awards as an HTML table.
// printTitle controls whether the Title column is shown.
// printAuthors controls whether the Author(s) column is shown.
func PrintAwardTable(w io.Writer, awards []*Award, printTitle bool, printAuthors bool) {
	fmt.Fprintln(w, `<table>`)
	fmt.Fprintln(w, `<tr class="table2">`)
	fmt.Fprintln(w, `<th>Place</th>`)
	fmt.Fprintln(w, `<th>Year and Award</th>`)
	if printTitle {
		fmt.Fprintln(w, `<th>Title</th>`)
	}
	if printAuthors {
		fmt.Fprintln(w, `<th>Author(s)</th>`)
	}
	fmt.Fprintln(w, `<th>Category</th>`)
	fmt.Fprintln(w, `</tr>`)

	displays, err := LoadAwardDisplayBatch(DB, awards)
	if err != nil {
		log.Println(err)
		return
	}
	for i, d := range displays {
		rowClass := "table1"
		if i%2 == 0 {
			rowClass = "table2"
		}
		printAwardRow(w, d, printTitle, printAuthors, rowClass)
	}

	fmt.Fprintln(w, `</table>`)
}

func printAwardRow(w io.Writer, d *AwardDisplay, printTitle bool, printAuthors bool, rowClass string) {
	fmt.Fprintf(w, "<tr class=\"%s\">\n", rowClass)

	// Place / level column
	fmt.Fprintln(w, "<td>")
	printAwardLevel(w, d, printTitle)
	fmt.Fprintln(w, "</td>")

	// Year and award type column
	printAwardYear(w, d)

	// Title column
	if printTitle {
		fmt.Fprintln(w, "<td>")
		printAwardTitle(w, d)
		fmt.Fprintln(w, "</td>")
	}

	// Authors column
	if printAuthors {
		fmt.Fprintln(w, "<td>")
		printAwardAuthors(w, d)
		fmt.Fprintln(w, "</td>")
	}

	// Category column — link to award_category_year page
	year := ""
	if len(d.AwardYear) >= 4 {
		year = d.AwardYear[:4]
	}
	fmt.Fprintf(w, "<td><a href=\"/award_category_year.cgi?%d+%s\">%s</a></td>\n",
		d.CatID, year, ISFDBText(d.CatName))

	fmt.Fprintln(w, "</tr>")
}

func printAwardLevel(w io.Writer, d *AwardDisplay, printTitle bool) {
	level := ""
	cssClass := ""

	levelInt, _ := strconv.Atoi(d.AwardLevel)
	if levelInt > 70 {
		level = specialAwards[d.AwardLevel]
		cssClass = "italic"
	} else if d.TypePoll == "Yes" {
		if levelInt == 1 {
			level = "1"
			cssClass = "bold"
		} else {
			level = d.AwardLevel
		}
	} else {
		if levelInt == 1 {
			level = "Win"
			cssClass = "bold"
		} else {
			level = "Nomination"
		}
	}

	// Build the level link
	var levelLink string
	if cssClass != "" {
		levelLink = fmt.Sprintf("<a href=\"/award_details.cgi?%d\" class=\"%s\">%s</a>",
			d.AwardID, cssClass, ISFDBText(level))
	} else {
		levelLink = fmt.Sprintf("<a href=\"/award_details.cgi?%d\">%s</a>",
			d.AwardID, ISFDBText(level))
	}

	// Append formatted note as title attribute if present
	if d.AwardNote != "" {
		formatted := FormatNote(d.AwardNote, "", "full", 0, "", false)
		levelLink += fmt.Sprintf(" <abbr title=\"%s\">&#9432;</abbr>", ISFDBText(formatted))
	}

	fmt.Fprintln(w, levelLink)
}

func printAwardYear(w io.Writer, d *AwardDisplay) {
	year := ""
	if len(d.AwardYear) >= 4 {
		year = d.AwardYear[:4]
	}
	fmt.Fprintf(w, "<td><a href=\"/ay.cgi?%d+%s\">%s %s</a></td>\n",
		d.TypeID, year, year, ISFDBText(d.TypeShortName))
}

func printAwardTitle(w io.Writer, d *AwardDisplay) {
	if d.TitleID != 0 {
		// Load title to get name and check for parent
		title, err := SQLloadTitleData(DB, d.TitleID)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Fprintf(w, "<a href=\"/title.cgi?%d\">%s</a>", d.TitleID, ISFDBText(title.TitleTitle.String))

		// Check for variant/translation
		if title.TitleParent != 0 {
			parent, err := SQLloadTitleData(DB, title.TitleParent)
			if err == nil {
				if title.TitleLanguage.Valid && parent.TitleLanguage.Valid &&
					title.TitleLanguage.Int32 != parent.TitleLanguage.Int32 {
					fmt.Fprintf(w, " (translation of <a href=\"/title.cgi?%d\">%s</a>)",
						parent.TitleID, ISFDBText(parent.TitleTitle.String))
				} else if parent.TitleTitle.String != title.TitleTitle.String {
					fmt.Fprintf(w, " (variant of <a href=\"/title.cgi?%d\">%s</a>)",
						parent.TitleID, ISFDBText(parent.TitleTitle.String))
				}
			}
		}
	} else if d.AwardTitle != "" && d.AwardTitle != "untitled" {
		fmt.Fprint(w, ISFDBText(d.AwardTitle))
	}
}

func printAwardAuthors(w io.Writer, d *AwardDisplay) {
	if d.TitleID != 0 {
		// Title-based: use resolved AuthorRef slice
		for i, a := range d.Authors {
			if i > 0 {
				fmt.Fprint(w, " <b>and</b> ")
			}
			fmt.Fprint(w, BuildAuthorLink(a.AuthorID, a.Canonical))
		}
	} else {
		// Non-title-based: use raw author strings from the award record
		for i, author := range d.AwardAuthors {
			if i > 0 {
				fmt.Fprint(w, " <b>and</b> ")
			}
			actual := strings.SplitN(author, "^", 2)
			name := actual[0]
			if strings.Contains(name, "***") {
				fmt.Fprint(w, "-")
			} else if name == "No Award" {
				fmt.Fprint(w, "No Award")
			} else {
				// Try to resolve to an author record for a link
				a, err := SQLgetAuthorByName(DB, name)
				if err == nil && a != nil {
					fmt.Fprint(w, BuildAuthorLink(a.AuthorID, name))
				} else {
					fmt.Fprint(w, ISFDBText(name))
				}
			}
		}
	}
}
