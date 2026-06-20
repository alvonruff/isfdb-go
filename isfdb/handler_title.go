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

func TitleHandler(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	t, err := SQLloadTitleData(DB, id)
	if err != nil {
		http.Error(w, "Title not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	RecordHistory("Title", t.TitleTitle.String, r.URL.RequestURI())
	HTMLheader(w, t.TitleTitle.String)
	PrintNavbar(w, "title", "", "")

	// ── Title ContentBox ─────────────────────────────────────────────────
	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintf(w, "<b>Title:</b> %s\n", ISFDBText(t.TitleTitle.String))

	authors, err := SQLTitleBriefAuthorRecords(DB, id)
	if err != nil {
		log.Println(err)
		authors = []AuthorRef{}
	}
	fmt.Fprint(w, "<br>")
	switch t.TitleTType.String {
	case "ANTHOLOGY", "EDITOR":
		DisplayPersonLabel(w, "Editor", authors)
	case "REVIEW":
		DisplayPersonLabel(w, "Reviewer", authors)
	case "INTERVIEW":
		DisplayPersonLabel(w, "Interviewer", authors)
	default:
		DisplayPersonLabel(w, "Author", authors)
	}
	DisplayPersons(w, authors)
	fmt.Fprintln(w, "<br>")

	fmt.Fprintf(w, "<b>Date:</b> %s<br>\n", ISFDBText(t.TitleCopyright.String))
	fmt.Fprintf(w, "<b>Type:</b> %s\n", ISFDBText(t.TitleTType.String))

	webpages, err := SQLloadTitleWebpages(DB, id)
	if err != nil {
		log.Println(err)
	} else if len(webpages) > 0 {
		domains, err := SQLLoadRecognizedDomains(DB)
		if err != nil {
			log.Println(err)
		} else {
			PrintWebPages(w, webpages, "<br>", domains)
		}
	}
	if t.TitleLanguage.Valid {
		if langName, ok := Languages[int(t.TitleLanguage.Int32)]; ok {
			fmt.Fprintf(w, "<br><b>Language:</b> %s\n", ISFDBText(langName))
		}
	}

	brRequired := true
	if t.NoteID.Valid {
		note, err := SQLgetNotes(DB, int(t.NoteID.Int32))
		if err != nil {
			log.Println(err)
		} else if note != "" {
			fmt.Fprintln(w, FormatNote(note, "Note", "short", id, "Title", true))
			brRequired = false
		}
	}
	if t.TitleSynopsis.Valid {
		synopsis, err := SQLgetNotes(DB, int(t.TitleSynopsis.Int32))
		if err != nil {
			log.Println(err)
		} else if synopsis != "" {
			fmt.Fprintln(w, FormatNote(synopsis, "Synopsis", "short", id, "Synopsis", true))
			brRequired = false
		}
	}
	if brRequired {
		fmt.Fprintln(w, "<br>")
	}
	fmt.Fprintln(w, `</div>`)

	// ── Awards ContentBox ─────────────────────────────────────────────────
	awardsList, err := SQLTitleAwards(DB, id)
	if err != nil {
		log.Println(err)
	} else if len(awardsList) > 0 {
		fmt.Fprintln(w, `<div class="ContentBox">`)
		fmt.Fprintln(w, `<h3 class="contentheader">Awards</h3>`)
		PrintAwardTable(w, awardsList, false, false)
		fmt.Fprintln(w, `</div>`)
	}

	// ── Publications ContentBox ───────────────────────────────────────────
	pubs, err := SQLGetPubsByTitle(DB, id)
	if err != nil {
		log.Println(err)
	} else {
		fmt.Fprintln(w, `<div class="ContentBox">`)
		fmt.Fprintf(w, "<h3 class=\"contentheader\">Publications</h3>\n")
		fmt.Fprintln(w, `<p>`)
		fmt.Fprintln(w, `<table class="publications">`)
		fmt.Fprintln(w, `<tr class="table2">`)
		fmt.Fprintln(w, `<th class="publication_title">Title</th>`)
		fmt.Fprintln(w, `<th class="publication_date">Date</th>`)
		fmt.Fprintln(w, `<th class="publication_author_editor">Author/Editor</th>`)
		fmt.Fprintln(w, `<th class="publication_publisher">Publisher/Pub. Series</th>`)
		fmt.Fprintln(w, `<th class="publication_isbn_catalog">ISBN/Catalog ID</th>`)
		fmt.Fprintln(w, `<th class="publication_price">Price</th>`)
		fmt.Fprintln(w, `<th class="publication_pages">Pages</th>`)
		fmt.Fprintln(w, `<th class="publication_format">Format</th>`)
		fmt.Fprintln(w, `<th class="publication_type">Type</th>`)
		fmt.Fprintln(w, `<th class="publication_cover_artist">Cover Artist</th>`)
		fmt.Fprintln(w, `</tr>`)

		pubIDs := make([]int, len(pubs))
		for i, p := range pubs {
			pubIDs[i] = p.PubID
		}

		publisherIDlist := make([]int, 0, len(pubs))
		seenPublisherIDs := make(map[int32]struct{})
		for _, p := range pubs {
			if p.PublisherID.Valid {
				if _, ok := seenPublisherIDs[p.PublisherID.Int32]; !ok {
					publisherIDlist = append(publisherIDlist, int(p.PublisherID.Int32))
					seenPublisherIDs[p.PublisherID.Int32] = struct{}{}
				}
			}
		}
		publisherNames, err := SQLgetPublisherNamesBatch(DB, publisherIDlist)
		if err != nil {
			log.Println(err)
			publisherNames = map[int32]string{}
		}

		pubAuthorsCache, err := SQLPubAuthorsBatch(DB, pubIDs)
		if err != nil {
			log.Println(err)
			pubAuthorsCache = map[int][]AuthorRef{}
		}

		coverArtists, err := SQLGetCoverAuthorsForPubs(DB, pubIDs)
		if err != nil {
			log.Println(err)
			coverArtists = map[int][]AuthorRef{}
		}

		for i, p := range pubs {
			rowClass := "table1"
			if i%2 == 1 {
				rowClass = "table0"
			}
			fmt.Fprintf(w, "<tr class=\"%s\">\n", rowClass)
			fmt.Fprintf(w, "<td><a href=\"/pub.cgi?%d\">%s</a></td>\n", p.PubID, ISFDBText(p.PubTitle.String))
			pubYear := ISFDBText(p.PubYear.String)
			if pubYear == "0000-00-00" || pubYear == "" {
				pubYear = "date unknown"
			}
			fmt.Fprintf(w, "<td>%s</td>\n", pubYear)
			authors := pubAuthorsCache[p.PubID]
			authorLinks := make([]string, len(authors))
			for i, a := range authors {
				authorLinks[i] = fmt.Sprintf("<a href=\"/author.cgi?%d\">%s</a>", a.AuthorID, ISFDBText(a.Canonical))
			}
			authorCell := strings.Join(authorLinks, ", ")
			if p.PubCType.String == "MAGAZINE" || p.PubCType.String == "ANTHOLOGY" {
				authorCell = "ed. " + authorCell
			}
			fmt.Fprintf(w, "<td>%s</td>\n", authorCell)
			publisherCell := ""
			if p.PublisherID.Valid {
				if pubName, ok := publisherNames[p.PublisherID.Int32]; ok {
					publisherCell = fmt.Sprintf("<a href=\"/publisher.cgi?%d\">%s</a>", p.PublisherID.Int32, ISFDBText(pubName))
				}
			}
			fmt.Fprintf(w, "<td>%s</td>\n", publisherCell)
			PrintISBNCatalog(w, p)
			fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(p.PubPrice.String))
			PrintPages(w, p)
			fmt.Fprintf(w, "<td>%s</td>\n", ISFDBText(p.PubPType.String))
			PrintPubType(w, p)
			fmt.Fprintln(w, "<td>")
			if artists, ok := coverArtists[p.PubID]; ok {
				artistLinks := make([]string, len(artists))
				for i, a := range artists {
					artistLinks[i] = fmt.Sprintf("<a href=\"/author.cgi?%d\">%s</a>", a.AuthorID, ISFDBText(a.Canonical))
				}
				fmt.Fprint(w, strings.Join(artistLinks, ", "))
			} else {
				fmt.Fprint(w, "&nbsp;")
			}
			fmt.Fprintln(w, "</td>")
			fmt.Fprintln(w, "</tr>")
		}
		fmt.Fprintln(w, `</table>`)
		fmt.Fprintln(w, `</div>`)
	}

	// ── Reviews ContentBox ────────────────────────────────────────────────
	reviews, err := SQLloadAllTitleReviews(DB, id)
	if err != nil {
		log.Println(err)
	} else if len(reviews) > 0 {
		PrintReviews(w, reviews, t.TitleLanguage)
	}

	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}
