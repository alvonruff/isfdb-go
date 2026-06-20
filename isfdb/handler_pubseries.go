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

func PubSeriesHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 1 {
		http.Error(w, "Usage: /pubseries.cgi?pub_series_id[+display_order]", http.StatusBadRequest)
		return
	}

	pubSeriesID, err := strconv.Atoi(params[0])
	if err != nil {
		http.Error(w, "Invalid pub series ID", http.StatusBadRequest)
		return
	}

	// display_order: 0=earliest first (default), 1=latest first, 2=by series number
	displayOrder := 0
	if len(params) >= 2 {
		if n, err := strconv.Atoi(params[1]); err == nil && n >= 0 && n <= 2 {
			displayOrder = n
		}
	}

	// Load the pub series record
	seriesRecs, err := SQLLoadPubSeriesBatch(DB, []int{pubSeriesID})
	if err != nil || len(seriesRecs) == 0 {
		http.Error(w, "Publication series not found", http.StatusNotFound)
		if err != nil {
			log.Println(err)
		}
		return
	}
	ps := seriesRecs[0]

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, "Publication Series: "+ps.PubSeriesName.String)
	PrintNavbar(w, "pub_series", "", "")

	// ── ContentBox 1: pub series metadata ────────────────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<ul>`)
	fmt.Fprintf(w, "<li><b>Publication Series:</b> %s\n", ISFDBText(ps.PubSeriesName.String))

	// Webpages
	webpages, err := SQLloadPubSeriesWebpages(DB, pubSeriesID)
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
	if ps.NoteID.Valid {
		note, err := SQLgetNotes(DB, int(ps.NoteID.Int32))
		if err != nil {
			log.Println(err)
		} else if note != "" {
			fmt.Fprintln(w, "<li>")
			fmt.Fprintln(w, FormatNote(note, "Note", "short", pubSeriesID, "Pubseries", false))
		}
	}

	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `</div>`)

	// ── ContentBox 2: sort controls + publications table ──────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)

	// Sort order links — show all except the current one
	type sortLink struct {
		order int
		label string
	}
	sortLinks := []sortLink{
		{0, "Show earliest year first"},
		{1, "Show last year first"},
		{2, "Sort by series number"},
	}
	bullet := " • "
	first := true
	for _, sl := range sortLinks {
		if sl.order == displayOrder {
			continue
		}
		if !first {
			fmt.Fprint(w, bullet)
		}
		fmt.Fprintf(w, "<a href=\"/pubseries.cgi?%d+%d\">%s</a>", pubSeriesID, sl.order, sl.label)
		first = false
	}
	fmt.Fprintln(w, "<p>")

	pubs, err := SQLGetPubSeriesPubs(DB, pubSeriesID, displayOrder)
	if err != nil {
		log.Println(err)
	} else {
		PrintPubsTable(w, pubs, PubTablePubSeries)
	}

	fmt.Fprintln(w, `</div>`)
	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}
