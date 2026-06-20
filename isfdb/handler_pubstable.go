// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

// PrintPubsTable renders the standard publications table used by publisheryear,
// pubseries, and other listing pages.  It matches Python's PrintPubsTable() and
// PrintOnePub() in biblio/common.py.

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"strings"
)

// PubTableDisplayType controls column layout.
type PubTableDisplayType int

const (
	PubTablePublisher  PubTableDisplayType = iota // publisheryear.cgi — pub series column
	PubTablePubSeries                             // pubseries.cgi      — publisher column
	PubTableDefault                               // title.cgi etc.     — combined column
)

// SQLGetPrimaryVerifiedBatch returns the set of pub IDs that have at least one
// primary verification entry.
func SQLGetPrimaryVerifiedBatch(db *sql.DB, pubIDs []int) (map[int]bool, error) {
	result := make(map[int]bool)
	if len(pubIDs) == 0 {
		return result, nil
	}
	ph := make([]string, len(pubIDs))
	args := make([]any, len(pubIDs))
	for i, id := range pubIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT DISTINCT pub_id FROM primary_verifications WHERE pub_id IN ("+
			strings.Join(ph, ",")+") ",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
}

// PrintPubsTable renders a <table class="publications"> with one row per pub.
func PrintPubsTable(w io.Writer, pubs []*Pub, displayType PubTableDisplayType) {
	if len(pubs) == 0 {
		return
	}

	// ── Batch pre-fetch all supporting data ──────────────────────────────
	pubIDs := make([]int, len(pubs))
	publisherIDset := make(map[int]bool)
	pubSeriesIDset := make(map[int]bool)
	for i, p := range pubs {
		pubIDs[i] = p.PubID
		if p.PublisherID.Valid {
			publisherIDset[int(p.PublisherID.Int32)] = true
		}
		if p.PubSeriesID.Valid {
			pubSeriesIDset[int(p.PubSeriesID.Int32)] = true
		}
	}

	pubAuthors, err := SQLPubAuthorsBatch(DB, pubIDs)
	if err != nil {
		log.Println(err)
		pubAuthors = map[int][]AuthorRef{}
	}

	publisherIDs := make([]int, 0, len(publisherIDset))
	for id := range publisherIDset {
		publisherIDs = append(publisherIDs, id)
	}
	publisherNames, err := SQLgetPublisherNamesBatch(DB, publisherIDs)
	if err != nil {
		log.Println(err)
		publisherNames = map[int32]string{}
	}

	pubSeriesIDs := make([]int, 0, len(pubSeriesIDset))
	for id := range pubSeriesIDset {
		pubSeriesIDs = append(pubSeriesIDs, id)
	}
	pubSeriesRecs, err := SQLLoadPubSeriesBatch(DB, pubSeriesIDs)
	if err != nil {
		log.Println(err)
	}
	pubSeriesNames := make(map[int32]string, len(pubSeriesRecs))
	for _, ps := range pubSeriesRecs {
		pubSeriesNames[int32(ps.PubSeriesID)] = ps.PubSeriesName.String
	}

	coverArtists, err := SQLGetCoverAuthorsForPubs(DB, pubIDs)
	if err != nil {
		log.Println(err)
		coverArtists = map[int][]AuthorRef{}
	}

	// ── Table header ──────────────────────────────────────────────────────
	fmt.Fprintln(w, `<table class="publications">`)
	fmt.Fprintln(w, `<tr class="table2">`)

	if displayType == PubTablePubSeries {
		fmt.Fprintln(w, `<th class="publication_date">Date</th>`)
		fmt.Fprintln(w, `<th class="publication_series_number">Pub. Series #</th>`)
		fmt.Fprintln(w, `<th class="publication_title">Title</th>`)
	} else {
		fmt.Fprintln(w, `<th class="publication_title">Title</th>`)
	}

	if displayType != PubTablePubSeries {
		fmt.Fprintln(w, `<th class="publication_date">Date</th>`)
	}
	fmt.Fprintln(w, `<th class="publication_author_editor">Author/Editor</th>`)

	switch displayType {
	case PubTablePublisher:
		fmt.Fprintln(w, `<th class="publication_publisher">Publication series</th>`)
	case PubTablePubSeries:
		fmt.Fprintln(w, `<th class="publication_publisher">Publisher</th>`)
	default:
		fmt.Fprintln(w, `<th class="publication_publisher">Publisher/Pub. Series</th>`)
	}

	fmt.Fprintln(w, `<th class="publication_isbn_catalog">ISBN/Catalog ID</th>`)
	fmt.Fprintln(w, `<th class="publication_price">Price</th>`)
	fmt.Fprintln(w, `<th class="publication_pages">Pages</th>`)
	fmt.Fprintln(w, `<th class="publication_format">Format</th>`)
	fmt.Fprintln(w, `<th class="publication_type">Type</th>`)
	fmt.Fprintln(w, `<th class="publication_cover_artist">Cover Artist</th>`)
	fmt.Fprintln(w, `</tr>`)

	// ── Table rows ────────────────────────────────────────────────────────
	for i, p := range pubs {
		bgcolor := i % 2
		printOnePub(w, p, pubAuthors, publisherNames, pubSeriesNames,
			coverArtists, bgcolor, displayType)
	}

	fmt.Fprintln(w, `</table>`)
}

// pubDateDisplay formats a pub_year for display in the publications table.
// It matches Python's ISFDBconvertDate(date, 1) behaviour: with a truthy
// precision the raw YYYY-MM-DD string is returned unchanged, with special
// cases for unknown/unpublished/forthcoming sentinel values.
func pubDateDisplay(date string, _ int) string {
	switch date {
	case "", "0000-00-00":
		return "date unknown"
	case "8888-00-00":
		return "unpublished"
	case "9999-00-00":
		return "forthcoming"
	}
	return date
}

func printOnePub(w io.Writer, p *Pub,
	pubAuthors map[int][]AuthorRef,
	publisherNames map[int32]string,
	pubSeriesNames map[int32]string,
	coverArtists map[int][]AuthorRef,
	bgcolor int,
	displayType PubTableDisplayType,
) {
	fmt.Fprintf(w, "<tr class=\"table%d\">\n", bgcolor)

	// pubseries mode: Date and Pub. Series # come first
	if displayType == PubTablePubSeries {
		fmt.Fprintf(w, "<td>%s</td>\n", pubDateDisplay(p.PubYear.String, 1))
		if p.PubSeriesNum.Valid && p.PubSeriesNum.String != "" {
			fmt.Fprintf(w, "<td dir=\"ltr\">%s</td>\n", ISFDBText(p.PubSeriesNum.String))
		} else {
			fmt.Fprintln(w, "<td>&nbsp;</td>")
		}
	}

	// Title
	fmt.Fprintf(w, "<td dir=\"ltr\"><a href=\"/pub.cgi?%d\">%s</a></td>\n",
		p.PubID, ISFDBText(p.PubTitle.String))

	// Date (not for pubseries — already shown)
	// publisheryear always shows full YYYY-MM-DD; other views use YYYY-MM.
	if displayType != PubTablePubSeries {
		precision := 1
		if displayType == PubTablePublisher {
			precision = 2
		}
		fmt.Fprintf(w, "<td>%s</td>\n", pubDateDisplay(p.PubYear.String, precision))
	}

	// Author/Editor
	isEdited := p.PubCType.String == "ANTHOLOGY" ||
		p.PubCType.String == "MAGAZINE" ||
		p.PubCType.String == "FANZINE"
	authors := pubAuthors[p.PubID]
	authorLinks := make([]string, len(authors))
	for i, a := range authors {
		authorLinks[i] = BuildAuthorLink(a.AuthorID, a.Canonical)
	}
	if isEdited {
		fmt.Fprintf(w, "<td>ed. %s</td>\n", strings.Join(authorLinks, ", "))
	} else {
		fmt.Fprintf(w, "<td>%s</td>\n", strings.Join(authorLinks, ", "))
	}

	// Publisher / Pub series column
	publisherCell := formatPublisherCell(p, publisherNames)
	pubSeriesCell := formatPubSeriesCell(p, pubSeriesNames)

	switch displayType {
	case PubTablePublisher:
		if pubSeriesCell != "" {
			fmt.Fprintf(w, "<td dir=\"ltr\">%s</td>\n", pubSeriesCell)
		} else {
			fmt.Fprintln(w, "<td>&nbsp;</td>")
		}
	case PubTablePubSeries:
		if publisherCell != "" {
			fmt.Fprintf(w, "<td dir=\"ltr\">%s</td>\n", publisherCell)
		} else {
			fmt.Fprintln(w, "<td>&nbsp;</td>")
		}
	default:
		var out string
		switch {
		case publisherCell != "" && pubSeriesCell != "":
			out = publisherCell + " (" + pubSeriesCell + ")"
		case publisherCell != "":
			out = publisherCell
		case pubSeriesCell != "":
			out = pubSeriesCell
		}
		if out != "" {
			fmt.Fprintf(w, "<td dir=\"ltr\">%s</td>\n", out)
		} else {
			fmt.Fprintln(w, "<td>&nbsp;</td>")
		}
	}

	// ISBN / Catalog ID
	printISBNCatalogCell(w, p)

	// Price
	if p.PubPrice.Valid && p.PubPrice.String != "" {
		fmt.Fprintf(w, "<td dir=\"ltr\">%s</td>\n", ISFDBText(p.PubPrice.String))
	} else {
		fmt.Fprintln(w, "<td>&nbsp;</td>")
	}

	// Pages (may contain '+' separators)
	if p.PubPages.Valid && p.PubPages.String != "" {
		parts := strings.Split(p.PubPages.String, "+")
		fmt.Fprint(w, "<td>")
		for i, pg := range parts {
			if i > 0 {
				fmt.Fprint(w, "+<br>")
			}
			fmt.Fprint(w, ISFDBText(pg))
		}
		fmt.Fprintln(w, "</td>")
	} else {
		fmt.Fprintln(w, "<td>&nbsp;</td>")
	}

	// Format
	if p.PubPType.Valid && p.PubPType.String != "" {
		fmt.Fprintf(w, "<td>%s</td>\n", ISFDBPubFormat(p.PubPType.String))
	} else {
		fmt.Fprintln(w, "<td>&nbsp;</td>")
	}

	// Type
	if p.PubCType.Valid && p.PubCType.String != "" {
		label, ok := pubTypeShortNames[p.PubCType.String]
		if !ok {
			label = p.PubCType.String
		}
		fmt.Fprintf(w, "<td>%s</td>\n", label)
	} else {
		fmt.Fprintln(w, "<td>&nbsp;</td>")
	}

	// Cover artists
	if artists := coverArtists[p.PubID]; len(artists) > 0 {
		links := make([]string, len(artists))
		for i, a := range artists {
			links[i] = BuildAuthorLink(a.AuthorID, a.Canonical)
		}
		fmt.Fprintf(w, "<td>%s</td>\n", strings.Join(links, ", "))
	} else {
		fmt.Fprintln(w, "<td>&nbsp;</td>")
	}

	fmt.Fprintln(w, "</tr>")
}

// formatPublisherCell returns an HTML link to the publisher page, or "".
func formatPublisherCell(p *Pub, names map[int32]string) string {
	if !p.PublisherID.Valid {
		return ""
	}
	name := names[p.PublisherID.Int32]
	if name == "" {
		return ""
	}
	return fmt.Sprintf("<a href=\"/publisher.cgi?%d\">%s</a>",
		p.PublisherID.Int32, ISFDBText(name))
}

// formatPubSeriesCell returns an HTML link to the pub series page (with series
// number appended if present), or "".
func formatPubSeriesCell(p *Pub, names map[int32]string) string {
	if !p.PubSeriesID.Valid {
		return ""
	}
	name := names[p.PubSeriesID.Int32]
	if name == "" {
		return ""
	}
	out := fmt.Sprintf("<a href=\"/pubseries.cgi?%d\">%s</a>",
		p.PubSeriesID.Int32, ISFDBText(name))
	if p.PubSeriesNum.Valid && p.PubSeriesNum.String != "" {
		out += " " + ISFDBText(p.PubSeriesNum.String)
	}
	return out
}

// printISBNCatalogCell writes the ISBN/Catalog ID table cell.
func printISBNCatalogCell(w io.Writer, p *Pub) {
	isbn := ""
	if p.PubISBN.Valid && p.PubISBN.String != "" {
		isbn = ISFDBText(p.PubISBN.String)
	}
	catalog := ""
	if p.PubCatalog.Valid && p.PubCatalog.String != "" {
		catalog = ISFDBText(p.PubCatalog.String)
	}
	var value string
	switch {
	case isbn != "" && catalog != "":
		value = isbn + " / " + catalog
	case isbn != "":
		value = isbn
	case catalog != "":
		value = catalog
	default:
		value = "&nbsp;"
	}
	fmt.Fprintf(w, "<td dir=\"ltr\">%s</td>\n", value)
}
