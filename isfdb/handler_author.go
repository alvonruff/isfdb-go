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

func AuthorHandler(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a, err := SQLloadAuthorData(DB, id)
	if err != nil {
		http.Error(w, "Author not found", http.StatusNotFound)
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	RecordHistory("Author", a.AuthorCanonical.String, r.URL.RequestURI())
	HTMLheader(w, a.AuthorCanonical.String)
	PrintNavbar(w, "author", "", "")

	printAuthorMetadataBox(w, a, id)

	// ── Bibliography ContentBox (bottom section) ──────────────────────────
	bd, err := LoadBibliographyData(DB, id)
	if err != nil {
		log.Println(err)
	} else {
		authorLangID := 0
		if a.AuthorLanguage.Valid {
			authorLangID = int(a.AuthorLanguage.Int32)
		}
		PrintBibliography(w, bd, id, authorLangID)
	}

	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// printAuthorMetadataBox renders the author ContentBox (top section) used by
// author.cgi, author_alpha.cgi, and other author bibliography pages.
func printAuthorMetadataBox(w http.ResponseWriter, a *Author, id int) {
	fmt.Fprintln(w, `<div class="ContentBox">`)

	// Left/right layout table only when an author image is present.
	hasImage := a.AuthorImage.Valid && a.AuthorImage.String != ""
	if hasImage {
		image := ISFDBHostCorrection(a.AuthorImage.String, "")
		if idx := strings.Index(image, "|"); idx >= 0 {
			image = image[:idx]
		}
		fmt.Fprintln(w, `<table>`)
		fmt.Fprintln(w, `<tr align="left">`)
		fmt.Fprintln(w, `<td>`)
		fmt.Fprintf(w, "<img src=\"%s\" width=\"150\" alt=\"Author Picture\">\n", image)
		fmt.Fprintln(w, `</td>`)
		fmt.Fprintln(w, `<td class="authorimage">`)
	}

	fmt.Fprintln(w, `<ul>`)
	fmt.Fprintf(w, "<li><b>Author:</b> %s\n", ISFDBText(a.AuthorCanonical.String))

	if a.AuthorLegalName.Valid && a.AuthorLegalName.String != "" {
		fmt.Fprintf(w, "<li><b>Legal Name:</b> %s\n", ISFDBText(a.AuthorLegalName.String))
	}

	if a.AuthorBirthPlace.Valid && a.AuthorBirthPlace.String != "" {
		fmt.Fprintf(w, "<li><b>Birthplace:</b> %s\n", ISFDBText(a.AuthorBirthPlace.String))
	}

	if a.AuthorBirthDate.Valid && a.AuthorBirthDate.String != "" {
		fmt.Fprintf(w, "<li><b>Birthdate:</b> %s\n", ConvertAuthorDate(a.AuthorBirthDate.String))
	}

	if a.AuthorDeathDate.Valid && a.AuthorDeathDate.String != "" {
		fmt.Fprintf(w, "<li><b>Deathdate:</b> %s\n", ConvertAuthorDate(a.AuthorDeathDate.String))
	}

	if a.AuthorLanguage.Valid {
		if langName, ok := Languages[int(a.AuthorLanguage.Int32)]; ok {
			fmt.Fprintf(w, "<li><b>Language:</b> %s\n", ISFDBText(langName))
		}
	}

	// Webpages
	webpages, err := SQLloadWebpages(DB, id)
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

	// Pseudonym relationships
	actualAuthors, err := SQLgetBriefActualFromPseudo(DB, id)
	if err != nil {
		log.Println(err)
	} else if len(actualAuthors) > 0 {
		fmt.Fprintln(w, "<li><b>Used As Alternate Name By:</b>")
		printAuthorList(w, actualAuthors)
	}

	pseudoAuthors, err := SQLgetBriefPseudoFromActual(DB, id)
	if err != nil {
		log.Println(err)
	} else if len(pseudoAuthors) > 0 {
		fmt.Fprintln(w, "<li><b>Used These Alternate Names:</b>")
		printAuthorList(w, pseudoAuthors)
	}

	// Note
	if a.NoteID.Valid {
		note, err := SQLgetNotes(DB, int(a.NoteID.Int32))
		if err != nil {
			log.Println(err)
		} else if note != "" {
			fmt.Fprintln(w, "<li>")
			fmt.Fprintln(w, FormatNote(note, "Note", "short", id, "Author", false))
		}
	}

	fmt.Fprintln(w, `</ul>`)

	if hasImage {
		fmt.Fprintln(w, `</td>`)
		fmt.Fprintln(w, `</table>`)

		rawImage := a.AuthorImage.String
		if strings.Contains(rawImage, "amazon.com") {
			image := ISFDBHostCorrection(rawImage, "")
			if idx := strings.Index(image, "|"); idx >= 0 {
				image = image[:idx]
			}
			domains, err := SQLLoadRecognizedDomains(DB)
			if err != nil {
				log.Println(err)
			} else {
				_, credit, homePage, linkedPage := BuildDisplayedURL(image, domains)
				fmt.Fprintf(w, "Image supplied by <a href=\"%s\" target=\"_blank\">%s</a>",
					homePage, ISFDBText(credit))
				if linkedPage != "" {
					fmt.Fprintf(w, " on <a href=\"%s\" target=\"_blank\">this Web page</a>", linkedPage)
				}
			}
		} else {
			fmt.Fprintf(w, "Image supplied by <a href=\"https://www.isfdb.org\" target=\"_blank\">ISFDB</a>")
			filename := rawImage
			if idx := strings.LastIndex(rawImage, "/"); idx >= 0 {
				filename = rawImage[idx+1:]
			}
			if idx := strings.Index(filename, "|"); idx >= 0 {
				filename = filename[:idx]
			}
			if filename != "" {
				wikiPage := "https://www.isfdb.org/wiki/index.php/Image:" + filename
				fmt.Fprintf(w, " on <a href=\"%s\" target=\"_blank\">this Web page</a>", wikiPage)
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, `</div>`)
}

// printAuthorList renders a comma-separated list of linked author names.
func printAuthorList(w interface{ Write([]byte) (int, error) }, authors []AuthorRef) {
	links := make([]string, len(authors))
	for i, a := range authors {
		links[i] = BuildAuthorLink(a.AuthorID, a.Canonical)
	}
	fmt.Fprintln(w, strings.Join(links, ", "))
}
