// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
)

func PublisherHandler(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p, err := SQLloadPublisherData(DB, id)
	if err != nil {
		http.Error(w, "Publisher not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, "Publisher: "+p.PublisherName.String)
	PrintNavbar(w, "publisher", "", "")

	// ── ContentBox 1: Publisher metadata ─────────────────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<ul>`)
	fmt.Fprintf(w, "<li><b>Publisher:</b> %s\n", ISFDBText(p.PublisherName.String))

	// Webpages
	webpages, err := SQLloadPublisherWebpages(DB, id)
	if err != nil {
		log.Println(err)
	} else if len(webpages) > 0 {
		domains, err := SQLLoadRecognizedDomains(DB)
		if err != nil {
			log.Println(err)
		} else {
			PrintWebPages(w, webpages, "<li>", domains)
		}
	}

	// Note
	if p.NoteID.Valid {
		note, err := SQLgetNotes(DB, int(p.NoteID.Int32))
		if err != nil {
			log.Println(err)
		} else if note != "" {
			fmt.Fprintln(w, "<li>")
			fmt.Fprintln(w, FormatNote(note, "Note", "short", id, "Publisher", false))
		}
	}

	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `</div>`)

	// ── ContentBox 2: Publication series data ─────────────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<h3 class="contentheader">Publication series data</h3>`)
	fmt.Fprintln(w, `<ul class="unindent">`)

	// Count of pubs not in any pub series
	notInSeries, err := SQLCountPubsNotInPubSeries(DB, id)
	if err != nil {
		log.Println(err)
	} else if notInSeries > 0 {
		plural := ""
		if notInSeries > 1 {
			plural = "s"
		}
		display := fmt.Sprintf("%d publication%s not in a publication series", notInSeries, plural)
		if notInSeries > maxPubsNotInSeriesDisplay {
			display += " (too many to display on one page)"
			fmt.Fprintf(w, "<li><b>%s</b>\n", display)
		} else {
			fmt.Fprintf(w, "<li><b><a href=\"/pubs_not_in_series.cgi?%d\">%s</a></b>\n",
				id, display)
		}
	}

	// All pub series used by this publisher
	seriesIDs, err := SQLFindPubSeriesForPublisher(DB, id)
	if err != nil {
		log.Println(err)
	} else if len(seriesIDs) > 0 {
		pubSeriesList, err := SQLLoadPubSeriesBatch(DB, seriesIDs)
		if err != nil {
			log.Println(err)
		} else {
			for _, ps := range pubSeriesList {
				fmt.Fprintf(w, "<li><a href=\"/pubseries.cgi?%d\">%s</a>\n",
					ps.PubSeriesID, ISFDBText(ps.PubSeriesName.String))
			}
		}
	}

	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `</div>`)

	// ── ContentBox 3: Years when books were published ─────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<h3 class="contentheader">Years When Books Were Published</h3>`)

	years, err := SQLGetPublisherYears(DB, id)
	if err != nil {
		log.Println(err)
	} else {
		fmt.Fprintln(w, `<table class="yearblock">`)

		// First pass: handle special-value years and find decade range
		lowDecade, hiDecade := 10000, 0
		yearSet := make(map[int]bool, len(years))

		for _, year := range years {
			yearSet[year] = true
			switch year {
			case 0:
				fmt.Fprintln(w, "<tr>")
				fmt.Fprintf(w, "<td colspan=\"10\"><a href=\"/publisheryear.cgi?%d+%d\">unknown</a></td>\n", id, year)
				fmt.Fprintln(w, "</tr>")
			case 8888:
				fmt.Fprintln(w, "<tr>")
				fmt.Fprintf(w, "<td colspan=\"10\"><a href=\"/publisheryear.cgi?%d+%d\">unpublished</a></td>\n", id, year)
				fmt.Fprintln(w, "</tr>")
			case 9999:
				fmt.Fprintln(w, "<tr>")
				fmt.Fprintf(w, "<td colspan=\"10\"><a href=\"/publisheryear.cgi?%d+%d\">forthcoming</a></td>\n", id, year)
				fmt.Fprintln(w, "</tr>")
			default:
				if year > 0 && year < 2100 {
					decade := (year / 10) * 10
					if decade < lowDecade {
						lowDecade = decade
					}
					if decade > hiDecade {
						hiDecade = decade
					}
				}
			}
		}

		// Second pass: render the decade grid
		if lowDecade <= hiDecade {
			for decade := lowDecade; decade <= hiDecade; decade += 10 {
				fmt.Fprintln(w, "<tr>")
				for year := decade; year < decade+10; year++ {
					if yearSet[year] {
						fmt.Fprintf(w, "<td><a href=\"/publisheryear.cgi?%d+%d\">%d</a></td>\n",
							id, year, year)
					} else {
						fmt.Fprintln(w, "<td>------</td>")
					}
				}
				fmt.Fprintln(w, "</tr>")
			}
		}

		fmt.Fprintln(w, `</table>`)
	}

	fmt.Fprintln(w, `</div>`)

	// ── ContentBox 4: Publication breakdown by author ─────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<b>Publication Breakdown by Author:</b>`)
	fmt.Fprintln(w, `<ul>`)
	fmt.Fprintf(w, "<li><a href=\"/publisher_authors.cgi?%d+name\">Sort by Author Name</a>\n", id)
	fmt.Fprintf(w, "<li><a href=\"/publisher_authors.cgi?%d+count\">Sort by Publication Count</a>\n", id)
	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `</div>`)

	fmt.Fprintln(w, `</div>`) // #content
	HTMLtrailer(w)
}
