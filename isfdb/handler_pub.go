// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

func PubHandler(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p, err := SQLloadPubData(DB, id)
	if err != nil {
		http.Error(w, "Publication not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	RecordHistory("Pub", p.PubTitle.String, r.URL.RequestURI())
	HTMLheader(w, p.PubTitle.String)
	PrintNavbar(w, "publication", "", "")

	// ── Publication ContentBox ────────────────────────────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)

	// Left/right layout table only when a cover image is present.
	hasImage := p.PubFrontImage.Valid && p.PubFrontImage.String != ""
	if hasImage {
		image := ISFDBHostCorrection(p.PubFrontImage.String, "")
		if idx := strings.Index(image, "|"); idx >= 0 {
			image = image[:idx]
		}
		fmt.Fprintln(w, `<table>`)
		fmt.Fprintln(w, `<tr class="scan">`)
		fmt.Fprintln(w, `<td>`)
		fmt.Fprintf(w, "<a href=\"%s\"><img src=\"%s\" alt=\"picture\" class=\"scan\"></a>\n",
			image, image)
		fmt.Fprintln(w, `</td>`)
		fmt.Fprintln(w, `<td class="pubheader">`)
	}

	// ── Metadata ─────────────────────────────────────────────────────────
	fmt.Fprintln(w, `<ul>`)
	fmt.Fprintf(w, "<li><b>Publication:</b> %s\n", ISFDBText(p.PubTitle.String))

	authors, err := SQLPubAuthors(DB, p.PubID)
	if err != nil {
		log.Println(err)
		authors = []AuthorRef{}
	}
	fmt.Fprint(w, "<li>")
	switch p.PubCType.String {
	case "ANTHOLOGY", "MAGAZINE", "FANZINE":
		DisplayPersonLabel(w, "Editor", authors)
	default:
		DisplayPersonLabel(w, "Author", authors)
	}
	fmt.Fprint(w, " ")
	DisplayPersons(w, authors)

	if p.PubISBN.Valid && p.PubISBN.String != "" {
		isbn := p.PubISBN.String
		compact := strings.ReplaceAll(isbn, "-", "")
		compact = strings.ReplaceAll(compact, " ", "")
		if !ISBNValidFormat(isbn) {
			fmt.Fprintf(w, "  <li id=\"badISBN\">ISBN: %s  (Bad format)\n", ISFDBText(isbn))
		} else if !ValidISBN(isbn) {
			fmt.Fprintf(w, "  <li id=\"badISBN\">ISBN: %s  (Bad Checksum)\n", ISFDBText(isbn))
		} else if len(compact) == 10 {
			formatted10, _ := ConvertISBN(DB, compact)
			formatted13, _ := ConvertISBN(DB, ToISBN13(compact))
			fmt.Fprintf(w, "  <li><b>ISBN:</b> %s [<small>%s</small>]\n", formatted10, formatted13)
		} else {
			// ISBN-13
			if strings.HasPrefix(compact, "978") {
				formatted13, _ := ConvertISBN(DB, compact)
				formatted10, _ := ConvertISBN(DB, ToISBN10(compact))
				fmt.Fprintf(w, "  <li><b>ISBN:</b> %s [<small>%s</small>]\n", formatted13, formatted10)
			} else {
				formatted13, _ := ConvertISBN(DB, compact)
				fmt.Fprintf(w, "  <li><b>ISBN:</b> %s\n", formatted13)
			}
		}
	}

	if p.PubYear.Valid && p.PubYear.String != "" {
		if d := ISFDBconvertDate(p.PubYear.String, 1); d != "" {
			fmt.Fprintf(w, "<li><b>Date:</b> %s\n", d)
		}
	}

	if p.PubCatalog.Valid && p.PubCatalog.String != "" {
		fmt.Fprintf(w, "  <li><b>Catalog ID:</b> %s\n", ISFDBText(p.PubCatalog.String))
	}

	if p.PublisherID.Valid {
		pubName, err := SQLgetPublisherName(DB, int(p.PublisherID.Int32))
		if err != nil {
			log.Println(err)
		} else {
			fmt.Fprintf(w, "<li>\n  <b>Publisher:</b> <a href=\"/publisher.cgi?%d\">%s</a>\n",
				p.PublisherID.Int32, ISFDBText(pubName))
		}
	}

	if p.PubSeriesID.Valid {
		seriesName, err := SQLgetPubSeriesName(DB, int(p.PubSeriesID.Int32))
		if err != nil {
			log.Println(err)
		} else {
			fmt.Fprintf(w, "<li>\n  <b>Pub. Series:</b> <a href=\"/pubseries.cgi?%d\">%s</a>\n",
				p.PubSeriesID.Int32, ISFDBText(seriesName))
		}
	}

	if p.PubSeriesNum.Valid && p.PubSeriesNum.String != "" {
		fmt.Fprintf(w, "<li>\n  <b>Pub. Series #:</b> %s\n", ISFDBText(p.PubSeriesNum.String))
	}

	if p.PubPrice.Valid && p.PubPrice.String != "" {
		fmt.Fprintf(w, "<li>\n  <b>Price:</b> %s\n", ISFDBText(p.PubPrice.String))
	}

	if p.PubPages.Valid && p.PubPages.String != "" {
		fmt.Fprintf(w, "<li>\n  <b>Pages:</b> %s\n", ISFDBText(p.PubPages.String))
	}

	if p.PubPType.Valid && p.PubPType.String != "" {
		fmt.Fprintf(w, "<li>\n  <b>Format:</b> %s\n", ISFDBPubFormat(p.PubPType.String))
	}

	if p.PubCType.Valid && p.PubCType.String != "" {
		fmt.Fprintf(w, "<li>\n  <b>Type:</b> %s\n", ISFDBText(p.PubCType.String))
	}

	// ── Cover art ─────────────────────────────────────────────────────────
	pubTitles, err := SQLGetPubTitles(DB, p.PubID)
	if err != nil {
		log.Println(err)
		pubTitles = nil
	}

	// Pre-fetch authors for all COVERART title IDs in one batch
	var coverTitles []*Title
	for _, t := range pubTitles {
		if t.TitleTType.String == "COVERART" {
			coverTitles = append(coverTitles, t)
		}
	}
	if len(coverTitles) > 0 {
		coverTitleIDs := make([]int, len(coverTitles))
		for i, t := range coverTitles {
			coverTitleIDs[i] = t.TitleID
		}
		coverAuthorCache, err := SQLTitleAuthorsBatch(DB, coverTitleIDs)
		if err != nil {
			log.Println(err)
			coverAuthorCache = map[int][]AuthorRef{}
		}

		for i, t := range coverTitles {
			coverIndicator := ""
			if i > 0 {
				coverIndicator = fmt.Sprintf("%d", i+1)
			}
			fmt.Fprintf(w, "<li><b>Cover%s:</b> <a href=\"/title.cgi?%d\">%s</a>",
				coverIndicator, t.TitleID, ISFDBText(t.TitleTitle.String))
			if artists := coverAuthorCache[t.TitleID]; len(artists) > 0 {
				fmt.Fprint(w, " by ")
				DisplayPersons(w, artists)
			}
			fmt.Fprintln(w)
		}
	}

	// ── Webpages ──────────────────────────────────────────────────────────
	webpages, err := SQLloadPubWebpages(DB, p.PubID)
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

	// ── Note ─────────────────────────────────────────────────────────────
	if p.NoteID.Valid {
		note, err := SQLgetNotes(DB, int(p.NoteID.Int32))
		if err != nil {
			log.Println(err)
		} else if note != "" {
			fmt.Fprintln(w, "<li>")
			fmt.Fprintln(w, FormatNote(note, "Notes", "short", p.PubID, "Publication", true))
		}
	}

	// ── External IDs ─────────────────────────────────────────────────────
	extIDs, err := SQLloadPubIdentifiers(DB, p.PubID)
	if err != nil {
		log.Println(err)
	} else if len(extIDs) > 0 {
		// Collect unique type IDs for a single batch site lookup
		typeIDset := make(map[int]struct{})
		for _, id := range extIDs {
			typeIDset[id.TypeID] = struct{}{}
		}
		typeIDs := make([]int, 0, len(typeIDset))
		for tid := range typeIDset {
			typeIDs = append(typeIDs, tid)
		}
		sites, err := SQLloadIdentifierSitesBatch(DB, typeIDs)
		if err != nil {
			log.Println(err)
			sites = map[int][]*IdentifierSite{}
		}
		fmt.Fprintln(w, "<li>")
		fmt.Fprintln(w, "  <b>External IDs:</b>")
		PrintExternalIDs(w, extIDs, sites)
	}

	fmt.Fprintln(w, `</ul>`)

	if hasImage {
		fmt.Fprintln(w, `</td>`)
		fmt.Fprintln(w, `</table>`)

		// Cover art credit
		rawImage := p.PubFrontImage.String
		if strings.Contains(rawImage, "amazon.com") {
			// Amazon: use BuildDisplayedURL for the credit link
			image := ISFDBHostCorrection(rawImage, "")
			if idx := strings.Index(image, "|"); idx >= 0 {
				image = image[:idx]
			}
			domains, err := SQLLoadRecognizedDomains(DB)
			if err != nil {
				log.Println(err)
			} else {
				_, credit, homePage, linkedPage := BuildDisplayedURL(image, domains)
				fmt.Fprintf(w, "Cover art supplied by <a href=\"%s\" target=\"_blank\">%s</a>",
					homePage, ISFDBText(credit))
				if linkedPage != "" {
					fmt.Fprintf(w, " on <a href=\"%s\" target=\"_blank\">this Web page</a>", linkedPage)
				}
			}
			if strings.Contains(rawImage, "/images/P/") {
				fmt.Fprintln(w, `<br>The displayed Amazon image is based on the publication's ISBN. It may no
					longer reflect the actual cover of this particular edition.`)
			} else if strings.Contains(rawImage, "/images/G/") {
				fmt.Fprintln(w, `<br>The displayed Amazon image is possibly unstable and may no
					longer reflect the actual cover of this particular edition.`)
			}
		} else {
			// Non-Amazon: credit ISFDB directly, link to the wiki Image: page
			fmt.Fprintf(w, "Cover art supplied by <a href=\"https://www.isfdb.org\" target=\"_blank\">ISFDB</a>")
			// Extract the filename from the image URL to build the wiki Image: link
			filename := rawImage
			if idx := strings.LastIndex(rawImage, "/"); idx >= 0 {
				filename = rawImage[idx+1:]
			}
			// Strip any pipe suffix
			if idx := strings.Index(filename, "|"); idx >= 0 {
				filename = filename[:idx]
			}
			if filename != "" {
				wikiPage := "https://www.isfdb.org/wiki/index.php/Image:" + filename
				fmt.Fprintf(w, " on <a href=\"%s\" target=\"_blank\">this Web page</a>", wikiPage)
			}
		}
	}

	fmt.Fprintln(w, `</div>`)

	// ── Contents ContentBox ───────────────────────────────────────────────
	PrintContents(w, pubTitles, p)

	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}
