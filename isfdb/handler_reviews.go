// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
)

// ISFDBconvertYear converts a 4-digit year string, returning "unknown" for "0000".
func ISFDBconvertYear(year string) string {
	if year == "0000" || year == "" {
		return "unknown"
	}
	return year
}

var fullMonthNames = [13]string{
	"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// ConvertAuthorDate formats a "YYYY-MM-DD" birth/death date as "D Month YYYY",
// matching the Python Bibliography.ConvertDate() method.
// Unknown components are omitted; year "0000" renders as "unknown".
func ConvertAuthorDate(date string) string {
	parts := strings.SplitN(date, "-", 3)
	if len(parts) != 3 {
		return date
	}
	year, month, day := parts[0], parts[1], parts[2]

	// Day: strip leading zero; omit if "00"
	dayStr := ""
	if day != "00" {
		if len(day) == 2 && day[0] == '0' {
			dayStr = string(day[1]) + " "
		} else {
			dayStr = day + " "
		}
	}

	// Month: convert to name; omit if "00"
	monthStr := ""
	if month != "00" {
		m := 0
		for i := 0; i < len(month); i++ {
			m = m*10 + int(month[i]-'0')
		}
		if m >= 1 && m <= 12 {
			monthStr = fullMonthNames[m] + " "
		}
	}

	// Year
	yearStr := year
	if year == "0000" {
		yearStr = "unknown"
	}

	return dayStr + monthStr + yearStr
}

// ISFDBconvertDate formats a "YYYY-MM-DD" date string for display.
// precision 0 = year only, 1 = year-month (drops "-00" day), 2 = full date.
// Returns "" for unknown/empty dates (year "0000" or "").
func ISFDBconvertDate(date string, precision int) string {
	if date == "" || date == "0000-00-00" {
		return ""
	}
	if len(date) < 4 {
		return date
	}
	year := date[:4]
	if year == "0000" {
		return ""
	}
	if precision == 0 || len(date) < 7 {
		return year
	}
	month := date[5:7]
	if month == "00" {
		return year
	}
	if precision == 1 || len(date) < 10 {
		return year + "-" + month
	}
	day := date[8:10]
	if day == "00" {
		return year + "-" + month
	}
	return year + "-" + month + "-" + day
}

// sortableDate converts a date string for sort purposes:
// - "0000-00-00" sorts last (replaced with "9999-99-99")
// - month "00" sorts after real months (replaced with "13")
func sortableDate(date string) string {
	if date == "0000-00-00" || date == "" {
		return "9999-99-99"
	}
	if len(date) >= 7 && date[5:7] == "00" {
		return date[:4] + "-13-" + date[7:]
	}
	return date
}

// displayDate reverses sortableDate for display purposes.
func displayDate(sortDate string) string {
	if sortDate == "9999-99-99" {
		return "0000-00-00"
	}
	return sortDate
}

// reviewKey is used to build the multi-level sort structure.
type reviewKey struct {
	sortDate   string // canonical review sort date
	sortID     int    // canonical review ID (parent or self)
	pubDate    string // pub sort date
	pubID      int
	reviewID   int
}

// PrintReviews outputs the Reviews ContentBox section.
func PrintReviews(w io.Writer, reviews []*TitleReview, titleLanguage sql.NullInt32) {
	if len(reviews) == 0 {
		return
	}

	// Build a sorted list of reviewKeys with their associated TitleReview data.
	type keyedReview struct {
		key  reviewKey
		data *TitleReview
	}

	var keyed []keyedReview
	for _, r := range reviews {
		sortID := r.ReviewID
		if r.ReviewParentID.Valid && r.ReviewParentID.Int32 != 0 {
			sortID = int(r.ReviewParentID.Int32)
		}

		reviewDate := r.ReviewCopyright.String
		sortDate := r.ParentCopyright.String
		if !r.ReviewParentID.Valid || r.ReviewParentID.Int32 == 0 {
			sortDate = reviewDate
		}

		keyed = append(keyed, keyedReview{
			key: reviewKey{
				sortDate: sortableDate(sortDate),
				sortID:   sortID,
				pubDate:  sortableDate(r.PubYear.String),
				pubID:    r.PubID,
				reviewID: r.ReviewID,
			},
			data: r,
		})
	}

	// Sort by sortDate, sortID, pubDate, pubID, reviewID
	sort.Slice(keyed, func(i, j int) bool {
		a, b := keyed[i].key, keyed[j].key
		if a.sortDate != b.sortDate {
			return a.sortDate < b.sortDate
		}
		if a.sortID != b.sortID {
			return a.sortID < b.sortID
		}
		if a.pubDate != b.pubDate {
			return a.pubDate < b.pubDate
		}
		if a.pubID != b.pubID {
			return a.pubID < b.pubID
		}
		return a.reviewID < b.reviewID
	})

	// Pre-fetch all authors for all review title IDs in one batch
	titleIDset := make(map[int]struct{})
	for _, kr := range keyed {
		titleIDset[kr.data.ReviewID] = struct{}{}
		if kr.data.ReviewParentID.Valid && kr.data.ReviewParentID.Int32 != 0 {
			titleIDset[int(kr.data.ReviewParentID.Int32)] = struct{}{}
		}
	}
	titleIDlist := make([]int, 0, len(titleIDset))
	for id := range titleIDset {
		titleIDlist = append(titleIDlist, id)
	}
	authorCache, err := SQLTitleAuthorsBatch(DB, titleIDlist)
	if err != nil {
		log.Println(err)
		authorCache = map[int][]AuthorRef{}
	}

	fmt.Fprintln(w, `<div class="ContentBox">`)
	fmt.Fprintln(w, `<h3 class="contentheader">Reviews</h3>`)
	fmt.Fprintln(w, `<ul class="noindent">`)

	// Track pub_counter per (sortDate, sortID) group
	type groupKey struct {
		sortDate string
		sortID   int
	}
	pubCounters := make(map[groupKey]int)

	for _, kr := range keyed {
		r := kr.data
		gk := groupKey{kr.key.sortDate, kr.key.sortID}
		pubCounters[gk]++
		pubCounter := pubCounters[gk]

		displayReviewDate := displayDate(kr.key.sortDate)
		displayPubDate := displayDate(kr.key.pubDate)

		parentReviewID := 0
		if r.ReviewParentID.Valid {
			parentReviewID = int(r.ReviewParentID.Int32)
		}

		// Look up authors from pre-fetched cache
		var parentAuthors, variantAuthors []AuthorRef
		if parentReviewID != 0 {
			parentAuthors = authorCache[parentReviewID]
			variantAuthors = authorCache[r.ReviewID]
		} else {
			parentAuthors = authorCache[r.ReviewID]
		}

		// Build language statement if review language differs from title language
		langStatement := ""
		if r.LanguageID.Valid && titleLanguage.Valid &&
			r.LanguageID.Int32 != titleLanguage.Int32 {
			if langName, ok := Languages[int(r.LanguageID.Int32)]; ok {
				langStatement = fmt.Sprintf(" [%s] ", ISFDBText(langName))
			}
		}

		if pubCounter == 1 {
			// First publication of this review
			fmt.Fprintf(w, "<li><a href=\"/title.cgi?%d\">Review</a>%s by ", r.ReviewID, langStatement)

			// Print authors
			DisplayPersons(w, parentAuthors)

			// Show variant authors if different
			if len(variantAuthors) > 0 && !authorIDsEqual(variantAuthors, parentAuthors) {
				displayVariantAuthors(w, variantAuthors)
			}

			fmt.Fprintf(w, " (%s)", ISFDBconvertYear(displayReviewDate[:4]))

			// Link to publication
			fmt.Fprintf(w, " in <a href=\"/pub.cgi?%d\">%s</a>",
				r.PubID, ISFDBText(r.PubTitle.String))

			// Show pub date if different from review date
			if displayReviewDate != displayPubDate {
				fmt.Fprintf(w, ", (%s)", ISFDBconvertYear(displayPubDate[:4]))
			}
			fmt.Fprintln(w)

		} else {
			// Subsequent publications — reprints
			if pubCounter == 2 {
				fmt.Fprintln(w, ", reprinted in:")
			}
			fmt.Fprintf(w, "<li>&ensp;&ensp;%s", langStatement)

			if parentReviewID != 0 && !authorIDsEqual(variantAuthors, parentAuthors) {
				displayVariantAuthors(w, variantAuthors)
			}

			fmt.Fprintf(w, "<a href=\"/pub.cgi?%d\">%s</a>",
				r.PubID, ISFDBText(r.PubTitle.String))
			fmt.Fprintf(w, " (%s)\n", ISFDBconvertYear(displayPubDate[:4]))
		}
	}

	fmt.Fprintln(w, `</ul>`)
	fmt.Fprintln(w, `</div>`)
}

// authorIDsEqual returns true if two AuthorRef slices contain the same set of author IDs.
func authorIDsEqual(a, b []AuthorRef) bool {
	if len(a) != len(b) {
		return false
	}
	ids := make(map[int]struct{}, len(a))
	for _, r := range a {
		ids[r.AuthorID] = struct{}{}
	}
	for _, r := range b {
		if _, ok := ids[r.AuthorID]; !ok {
			return false
		}
	}
	return true
}

// displayVariantAuthors prints variant review authors in parentheses.
func displayVariantAuthors(w io.Writer, authors []AuthorRef) {
	if len(authors) == 0 {
		return
	}
	links := make([]string, len(authors))
	for i, a := range authors {
		links[i] = BuildAuthorLink(a.AuthorID, a.Canonical)
	}
	fmt.Fprintf(w, " (as %s) ", strings.Join(links, " <b>and</b> "))
}
