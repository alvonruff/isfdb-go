// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

const maxPubsNotInSeries = 500

// PubsNotInSeriesHandler serves /pubs_not_in_series.cgi?<publisher_id>+<mode>
//
// mode 0 (default) — earliest year first, table view
// mode 1           — latest year first, table view
// mode 2           — earliest year first, cover thumbnails
func PubsNotInSeriesHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 1 {
		http.Error(w, "Publisher ID required", http.StatusBadRequest)
		return
	}
	publisherID, err := strconv.Atoi(params[0])
	if err != nil || publisherID <= 0 {
		http.Error(w, "Invalid publisher ID", http.StatusBadRequest)
		return
	}
	mode := 0
	if len(params) >= 2 {
		if v, err := strconv.Atoi(params[1]); err == nil && v >= 0 && v <= 2 {
			mode = v
		}
	}

	publisher, err := SQLloadPublisherData(DB, publisherID)
	if err != nil {
		http.Error(w, "Publisher not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	desc := mode == 1
	pubs, err := SQLGetPubsNotInSeries(DB, publisherID, desc)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	pubName := publisher.PublisherName.String
	pageTitle := "Publications not in a Publication Series for Publisher: " + pubName

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "pubs_not_in_series", "", "")
	fmt.Fprintln(w, `<div class="ContentBox">`)

	fmt.Fprintf(w,
		"Publications not in a Publication Series for Publisher: <a href=\"/publisher.cgi?%d\">%s</a><p>\n",
		publisherID, ISFDBText(pubName))

	if len(pubs) > maxPubsNotInSeries {
		fmt.Fprintf(w, "%d publications not in a publication series - too many to display on one page\n", len(pubs))
	} else {
		// Sort-order and view toggle links (bullet-separated)
		bullet := " &#8226; "
		switch mode {
		case 0:
			fmt.Fprintf(w, "<a href=\"/pubs_not_in_series.cgi?%d+1\">Show last year first</a>%s", publisherID, bullet)
			fmt.Fprintf(w, "<a href=\"/pubs_not_in_series.cgi?%d+2\">Show Covers</a>", publisherID)
		case 1:
			fmt.Fprintf(w, "<a href=\"/pubs_not_in_series.cgi?%d+0\">Show earliest year first</a>%s", publisherID, bullet)
			fmt.Fprintf(w, "<a href=\"/pubs_not_in_series.cgi?%d+2\">Show Covers</a>", publisherID)
		case 2:
			fmt.Fprintf(w, "<a href=\"/pubs_not_in_series.cgi?%d+0\">Show earliest year first</a>%s", publisherID, bullet)
			fmt.Fprintf(w, "<a href=\"/pubs_not_in_series.cgi?%d+1\">Show last year first</a>", publisherID)
		}
		fmt.Fprintln(w, "<p>")

		if mode == 2 {
			// Cover thumbnails view
			count := 0
			for _, p := range pubs {
				if p.PubFrontImage.Valid && p.PubFrontImage.String != "" {
					fmt.Fprintln(w, ISFDBScan(p.PubID, p.PubFrontImage.String))
					count++
				}
			}
			if count == 0 {
				fmt.Fprintln(w, "<h3>No covers to display</h3>")
			}
		} else {
			PrintPubsTable(w, pubs, PubTableDefault)
		}
	}

	fmt.Fprintln(w, `</div>`) // ContentBox
	fmt.Fprintln(w, `</div>`) // content
	HTMLtrailer(w)
}
