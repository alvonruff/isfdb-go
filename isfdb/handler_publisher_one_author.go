// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
)

func PublisherOneAuthorHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 2 {
		http.Error(w, "Usage: /publisher_one_author.cgi?publisher_id+author_id", http.StatusBadRequest)
		return
	}

	publisherID, err := strconv.Atoi(params[0])
	if err != nil {
		http.Error(w, "Invalid publisher ID", http.StatusBadRequest)
		return
	}
	authorID, err := strconv.Atoi(params[1])
	if err != nil {
		http.Error(w, "Invalid author ID", http.StatusBadRequest)
		return
	}

	publisherName, err := SQLgetPublisherName(DB, publisherID)
	if err != nil || publisherName == "" {
		http.Error(w, "Publisher not found", http.StatusNotFound)
		if err != nil {
			log.Println(err)
		}
		return
	}

	author, err := SQLloadAuthorData(DB, authorID)
	if err != nil {
		http.Error(w, "Author not found", http.StatusNotFound)
		log.Println(err)
		return
	}
	authorName := author.AuthorCanonical.String

	pubs, err := SQLGetPubsForAuthorPublisher(DB, publisherID, authorID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	title := fmt.Sprintf("Publications for Author %s Published by %s", authorName, publisherName)

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, title)
	PrintNavbar(w, "publisher_one_author", "", "")

	// Navigation links
	fmt.Fprintf(w, "<a href=\"/publisher.cgi?%d\">Return to the publisher page</a>", publisherID)
	fmt.Fprintf(w, " • <a href=\"/publisher_authors.cgi?%d+name\">Return to the Authors for Publisher %s page</a>\n",
		publisherID, ISFDBText(publisherName))

	fmt.Fprintln(w, `<p>The statistics below count the number of publications for the`)
	fmt.Fprintln(w, `specified author. They do not include individual titles (stories, poems, etc.)`)
	fmt.Fprintln(w, `contained in publications. Each edition of a book increments the count.`)
	fmt.Fprintln(w, `Only the currently selected form of the author's name is counted, e.g.`)
	fmt.Fprintln(w, `'Mary Shelley' does not include books published as by 'Mary W. Shelley'.`)

	// ── Decade/year summary grid ──────────────────────────────────────────
	// Build decade and year counts from the pub list.
	decades := make(map[int]int) // decade-key → count
	years := make(map[int]int)   // year → count
	for _, p := range pubs {
		yearStr := p.PubYear.String
		if len(yearStr) < 4 {
			continue
		}
		year, _ := strconv.Atoi(yearStr[:4])
		decade := year / 10
		years[year]++
		decades[decade]++
	}

	decadeKeys := make([]int, 0, len(decades))
	for d := range decades {
		decadeKeys = append(decadeKeys, d)
	}
	sort.Ints(decadeKeys)

	fmt.Fprintln(w, `<table class="seriesgrid">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th colspan="1">Decade</th>`)
	fmt.Fprintln(w, `<th colspan="13">Years</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, decade := range decadeKeys {
		rowCSS := "table1"
		if i%2 != 0 {
			rowCSS = "table2"
		}
		fmt.Fprintf(w, "<tr align=\"center\" class=\"%s\">\n", rowCSS)

		switch decade {
		case 888: // year 8888 = unpublished
			fmt.Fprintf(w, "<td>unpublished (%d)</td>\n", decades[decade])
			for x := 0; x < 10; x++ {
				fmt.Fprintln(w, "<td>-</td>")
			}
		case 0: // year 0000 = unknown
			fmt.Fprintf(w, "<td>unknown (%d)</td>\n", decades[decade])
			for x := 0; x < 10; x++ {
				fmt.Fprintln(w, "<td>-</td>")
			}
		default:
			fmt.Fprintf(w, "<td>%ds (%d)</td>\n", decade*10, decades[decade])
			for year := decade * 10; year < decade*10+10; year++ {
				if count, ok := years[year]; ok {
					fmt.Fprintf(w, "<td>%d (%d)</td>\n", year, count)
				} else {
					fmt.Fprintln(w, "<td>-</td>")
				}
			}
		}
		fmt.Fprintln(w, "</tr>")
	}

	fmt.Fprintln(w, `</table>`)
	fmt.Fprintln(w, `<p>`)

	// ── Publications table ────────────────────────────────────────────────
	PrintPubsTable(w, pubs, PubTablePublisher)

	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}
