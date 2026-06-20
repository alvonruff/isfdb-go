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

// NoteHandler serves /note.cgi?<record_type>+<record_id> — displays the full
// text of a note (or synopsis) for any record type, with a "Back to X" link.
// Matches Python's note.py.
func NoteHandler(w http.ResponseWriter, r *http.Request) {
	params := ParseRawParams(r)
	if len(params) != 2 {
		http.Error(w, "This page requires two parameters", http.StatusBadRequest)
		return
	}
	recordType := params[0]
	recordID, err := strconv.Atoi(params[1])
	if err != nil || recordID <= 0 {
		http.Error(w, "Invalid record ID", http.StatusBadRequest)
		return
	}

	noteType := "Note"   // overridden to "Synopsis" for Synopsis record type
	recordName := ""     // e.g. "Author", "Series"
	recordTitle := ""    // the name/title of the record
	noteBody := ""       // the resolved note text
	cgiScript := ""      // base CGI name for the "Back to" link (legacy names ok — they 301)

	switch recordType {
	case "Author":
		a, err := SQLloadAuthorData(DB, recordID)
		if err != nil {
			http.Error(w, "Author not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		cgiScript = "ea"
		recordTitle = a.AuthorCanonical.String
		noteBody = a.AuthorNote.String
		recordName = "Author"

	case "Title":
		t, err := SQLloadTitleData(DB, recordID)
		if err != nil {
			http.Error(w, "Title not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if t.NoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(t.NoteID.Int32))
		}
		cgiScript = "title"
		recordTitle = t.TitleTitle.String
		recordName = "Title"

	case "Synopsis":
		t, err := SQLloadTitleData(DB, recordID)
		if err != nil {
			http.Error(w, "Title not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if t.TitleSynopsis.Valid {
			noteBody, _ = SQLgetNotes(DB, int(t.TitleSynopsis.Int32))
		}
		noteType = "Synopsis"
		cgiScript = "title"
		recordTitle = t.TitleTitle.String
		recordName = "Title"

	case "Series":
		s, err := SQLLoadSeries(DB, recordID)
		if err != nil {
			http.Error(w, "Series not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if s.NoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(s.NoteID.Int32))
		}
		cgiScript = "pe"
		recordTitle = s.SeriesTitle
		recordName = "Series"

	case "SeriesGrid":
		s, err := SQLLoadSeries(DB, recordID)
		if err != nil {
			http.Error(w, "Series not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if s.NoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(s.NoteID.Int32))
		}
		cgiScript = "seriesgrid"
		recordTitle = s.SeriesTitle
		recordName = "Series grid"

	case "Award":
		a, err := SQLloadAwardData(DB, recordID)
		if err != nil {
			http.Error(w, "Award not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if a.AwardNoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(a.AwardNoteID.Int32))
		}
		cgiScript = "award_details"
		recordTitle = a.AwardTitle.String
		recordName = "Award"

	case "AwardCat":
		c, err := SQLGetAwardCatById(DB, recordID)
		if err != nil {
			http.Error(w, "Award category not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if c.AwardCatNoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(c.AwardCatNoteID.Int32))
		}
		cgiScript = "award_category"
		recordTitle = c.AwardCatName.String
		recordName = "Award Category"

	case "AwardType":
		at, err := SQLGetAwardTypeById(DB, recordID)
		if err != nil {
			http.Error(w, "Award type not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if at.AwardTypeNoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(at.AwardTypeNoteID.Int32))
		}
		cgiScript = "awardtype"
		recordTitle = at.AwardTypeName.String
		recordName = "Award Type"

	case "Publication":
		p, err := SQLloadPubData(DB, recordID)
		if err != nil {
			http.Error(w, "Publication not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if p.NoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(p.NoteID.Int32))
		}
		cgiScript = "pl"
		recordTitle = p.PubTitle.String
		recordName = "Publication"

	case "Publisher":
		pub, err := SQLloadPublisherData(DB, recordID)
		if err != nil {
			http.Error(w, "Publisher not found", http.StatusNotFound)
			log.Println(err)
			return
		}
		if pub.NoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(pub.NoteID.Int32))
		}
		cgiScript = "publisher"
		recordTitle = pub.PublisherName.String
		recordName = "Publisher"

	case "Pubseries":
		recs, err := SQLLoadPubSeriesBatch(DB, []int{recordID})
		if err != nil || len(recs) == 0 {
			http.Error(w, "Publication series not found", http.StatusNotFound)
			if err != nil {
				log.Println(err)
			}
			return
		}
		ps := recs[0]
		if ps.NoteID.Valid {
			noteBody, _ = SQLgetNotes(DB, int(ps.NoteID.Int32))
		}
		cgiScript = "pubseries"
		recordTitle = ps.PubSeriesName.String
		recordName = "Publication Series"

	default:
		http.Error(w, "Record type does not exist", http.StatusBadRequest)
		return
	}

	if noteBody == "" {
		http.Error(w, "Record does not exist or has no note", http.StatusNotFound)
		return
	}

	pageTitle := fmt.Sprintf("Full %s for %s: %s", noteType, recordName, recordTitle)
	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, pageTitle)
	PrintNavbar(w, "note", "", "")

	fmt.Fprintln(w, FormatNote(noteBody, noteType, "full", recordID, recordType, true))

	fmt.Fprintf(w,
		"<big>Back to <a href=\"/%s.cgi?%d\" class=\"inverted\">%s</a></big>\n",
		cgiScript, recordID, ISFDBText(recordTitle))

	fmt.Fprintln(w, `</div>`)
	HTMLtrailer(w)
}
