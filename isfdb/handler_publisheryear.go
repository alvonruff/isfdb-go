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

func PublisherYearHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 2 {
		http.Error(w, "Usage: /publisheryear.cgi?publisher_id+year", http.StatusBadRequest)
		return
	}

	publisherID, err := strconv.Atoi(params[0])
	if err != nil {
		http.Error(w, "Invalid publisher ID", http.StatusBadRequest)
		return
	}
	year, err := strconv.Atoi(params[1])
	if err != nil {
		http.Error(w, "Invalid year", http.StatusBadRequest)
		return
	}

	publisherName, err := SQLgetPublisherName(DB, publisherID)
	if err != nil || publisherName == "" {
		http.Error(w, "Publisher not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	// Display year: 0 → "0000", 8888 → "unpublished", 9999 → "forthcoming"
	displayYear := ISFDBconvertYear(fmt.Sprintf("%04d", year))

	title := fmt.Sprintf("Publisher %s: Books Published in %s", publisherName, displayYear)

	pubs, err := SQLGetPubsByPublisherYear(DB, publisherID, year)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, title)
	PrintNavbar(w, "publisheryear", "", "")

	if len(pubs) > 0 {
		fmt.Fprintln(w, "<p>")
		fmt.Fprintf(w, "<a href=\"/publisher.cgi?%d\">Return to the publisher page</a>\n", publisherID)
		fmt.Fprintln(w, "<p>")
		PrintPubsTable(w, pubs, PubTablePublisher)
	} else {
		fmt.Fprintf(w, "<h3>No publications found for %s in %s</h3>\n",
			ISFDBText(publisherName), displayYear)
		fmt.Fprintf(w, "<p><a href=\"/publisher.cgi?%d\">Return to the publisher page</a>\n", publisherID)
	}

	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}
