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

func PublisherAuthorsHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) < 2 {
		http.Error(w, "Usage: /publisher_authors.cgi?publisher_id+name|count", http.StatusBadRequest)
		return
	}

	publisherID, err := strconv.Atoi(params[0])
	if err != nil {
		http.Error(w, "Invalid publisher ID", http.StatusBadRequest)
		return
	}

	sortBy := params[1]
	if sortBy != "name" && sortBy != "count" {
		sortBy = "name"
	}

	publisherName, err := SQLgetPublisherName(DB, publisherID)
	if err != nil || publisherName == "" {
		http.Error(w, "Publisher not found", http.StatusNotFound)
		if err != nil {
			log.Println(err)
		}
		return
	}

	title := fmt.Sprintf("Authors for Publisher %s, Sorted by %s",
		publisherName, capitalize(sortBy))

	authors, err := SQLGetAllAuthorsForPublisher(DB, publisherID, sortBy)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, title)
	PrintNavbar(w, "publisher_authors", "", "")

	// Sort toggle + return link
	if sortBy == "name" {
		fmt.Fprintf(w, "<a href=\"/publisher_authors.cgi?%d+count\">Sort by Publication Count</a>", publisherID)
	} else {
		fmt.Fprintf(w, "<a href=\"/publisher_authors.cgi?%d+name\">Sort by Author Name</a>", publisherID)
	}
	fmt.Fprintf(w, " • <a href=\"/publisher.cgi?%d\">Return to the publisher page</a>\n", publisherID)

	fmt.Fprintln(w, `<p>Note that the statistics below count the number of publications associated`)
	fmt.Fprintln(w, `with publication-level authors and editors. They do not count the authors of`)
	fmt.Fprintln(w, `individual titles (stories, poems, etc.) contained in publications. Each edition`)
	fmt.Fprintln(w, `of a book increments its author's count. Different forms of an author's name, e.g.`)
	fmt.Fprintln(w, `'Mary Shelley' vs. 'Mary W. Shelley', are counted separately.`)

	// Author/count table
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr align="left" class="generic_table_header">`)
	fmt.Fprintln(w, `<th colspan="1">#</th>`)
	fmt.Fprintln(w, `<th colspan="1">Author/Editor</th>`)
	fmt.Fprintln(w, `<th colspan="1">Publication Count</th>`)
	fmt.Fprintln(w, `</tr>`)

	for i, a := range authors {
		rowCSS := "table1"
		if i%2 != 0 {
			rowCSS = "table2"
		}
		fmt.Fprintf(w, "<tr align=\"center\" class=\"%s\">\n", rowCSS)
		fmt.Fprintf(w, "<td>%d</td>\n", i+1)
		fmt.Fprintf(w, "<td>%s</td>\n", BuildAuthorLink(a.AuthorID, a.Canonical))
		fmt.Fprintf(w, "<td><a href=\"/publisher_one_author.cgi?%d+%d\">%d</a></td>\n",
			publisherID, a.AuthorID, a.Count)
		fmt.Fprintln(w, "</tr>")
	}

	fmt.Fprintln(w, `</table>`)
	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}

// capitalize returns s with its first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return string(s[0]-32) + s[1:]
}
