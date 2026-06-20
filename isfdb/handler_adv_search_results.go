// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// Parameter parsing helpers
// ─────────────────────────────────────────────────────────────────────────────

// advTerm holds one parsed search term from the query string.
type advTerm struct {
	Use      string // field key (e.g. "author_lastname")
	Operator string // operator key (e.g. "starts_with")
	Value    string // the search value
}

// advParams holds all parsed parameters from an adv_search_results request.
type advParams struct {
	SearchType  string    // e.g. "Author"
	OrderBy     string    // e.g. "author_lastname"
	Conjunction string    // "AND" or "OR"
	Action      string    // "query" or "count"
	Start       int       // pagination offset
	Terms       []advTerm // up to advMaxTerms search terms
}

func parseAdvParams(r *http.Request) (*advParams, string) {
	q := r.URL.Query()

	p := &advParams{
		SearchType:  q.Get("TYPE"),
		OrderBy:     q.Get("ORDERBY"),
		Conjunction: q.Get("C"),
		Action:      q.Get("ACTION"),
	}
	if p.Conjunction == "" {
		p.Conjunction = q.Get("CONJUNCTION_1")
	}
	if p.Conjunction == "" {
		p.Conjunction = "AND"
	}
	if p.Action == "" {
		p.Action = "query"
	}
	if p.Action != "query" && p.Action != "count" {
		return nil, "Invalid ACTION value"
	}
	if p.Conjunction != "AND" && p.Conjunction != "OR" {
		return nil, "Only AND and OR conjunction values are allowed"
	}

	startStr := q.Get("START")
	if startStr != "" {
		n, err := strconv.Atoi(startStr)
		if err != nil {
			return nil, "Invalid START value"
		}
		p.Start = n
	}
	if p.Start > 30000 {
		return nil, "Advanced Search Is Currently Limited to 300 pages or 30,000 records"
	}

	if p.SearchType == "" {
		return nil, "Missing TYPE parameter"
	}
	if p.OrderBy == "" {
		return nil, "Missing ORDERBY parameter"
	}

	// Collect terms.
	for n := 1; n <= advMaxTerms; n++ {
		termKey := fmt.Sprintf("TERM_%d", n)
		val := q.Get(termKey)
		if val == "" {
			continue
		}
		use := q.Get(fmt.Sprintf("USE_%d", n))
		op := q.Get(fmt.Sprintf("O_%d", n))
		if op == "" {
			op = q.Get(fmt.Sprintf("OPERATOR_%d", n))
		}
		if use == "" || op == "" {
			continue
		}
		// Replace * with % (additional wildcard support).
		val = strings.ReplaceAll(val, "*", "%")
		p.Terms = append(p.Terms, advTerm{Use: use, Operator: op, Value: val})
	}

	if len(p.Terms) == 0 {
		return nil, "No search data entered"
	}

	return p, ""
}

// ─────────────────────────────────────────────────────────────────────────────
// SQL clause building
// ─────────────────────────────────────────────────────────────────────────────

// advClause is the output of building one term's SQL.
type advClause struct {
	clause string   // WHERE sub-expression
	tables []string // additional FROM tables
	joins  []string // additional WHERE join conditions
}

// padEntry returns the SQL comparison expression (right-hand side) and appends
// the bind value(s) to args.  Returns (expression, updatedArgs, error).
func padEntry(operator, value string, args []any) (string, []any, string) {
	switch operator {
	case "exact":
		return "LIKE ?", append(args, value), ""
	case "notexact":
		return "NOT LIKE ?", append(args, value), ""
	case "contains":
		return "LIKE ?", append(args, "%"+value+"%"), ""
	case "notcontains":
		return "NOT LIKE ?", append(args, "%"+value+"%"), ""
	case "starts_with":
		return "LIKE ?", append(args, value+"%"), ""
	case "ends_with":
		return "LIKE ?", append(args, "%"+value), ""
	default:
		return "", args, "Invalid operator: " + operator
	}
}

func buildAuthorClause(t advTerm, args []any) (advClause, []any, string) {
	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "author_canonical":
		return advClause{clause: "authors.author_canonical " + cmp}, args, ""
	case "author_trans_name":
		return advClause{
			clause: "trans_authors.trans_author_name " + cmp,
			tables: []string{"trans_authors"},
			joins:  []string{"trans_authors.author_id=authors.author_id"},
		}, args, ""
	case "author_lastname":
		return advClause{clause: "authors.author_lastname " + cmp}, args, ""
	case "author_legalname":
		return advClause{clause: "authors.author_legalname " + cmp}, args, ""
	case "author_trans_legalname":
		return advClause{
			clause: "trans_legal_names.trans_legal_name " + cmp,
			tables: []string{"trans_legal_names"},
			joins:  []string{"trans_legal_names.author_id=authors.author_id"},
		}, args, ""
	case "author_birthplace":
		return advClause{clause: "authors.author_birthplace " + cmp}, args, ""
	case "author_birthdate":
		return advClause{clause: "authors.author_birthdate " + cmp}, args, ""
	case "author_deathdate":
		return advClause{clause: "authors.author_deathdate " + cmp}, args, ""
	case "author_language", "author_language_free":
		return advClause{
			clause: "languages.lang_name " + cmp,
			tables: []string{"languages"},
			joins:  []string{"languages.lang_id=authors.author_language"},
		}, args, ""
	case "author_image":
		return advClause{clause: "authors.author_image " + cmp}, args, ""
	case "author_webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"webpages"},
			joins:  []string{"webpages.author_id=authors.author_id"},
		}, args, ""
	case "author_email":
		return advClause{
			clause: "emails.email_address " + cmp,
			tables: []string{"emails"},
			joins:  []string{"emails.author_id=authors.author_id"},
		}, args, ""
	case "author_pseudos":
		return advClause{
			clause: "authors.author_id in (SELECT pseudonyms.author_id FROM pseudonyms, authors a1 WHERE pseudonym = a1.author_id AND a1.author_canonical " + cmp + ")",
		}, args, ""
	case "author_note":
		return advClause{clause: "authors.author_note " + cmp}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

func buildTitleClause(t advTerm, args []any) (advClause, []any, string) {
	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "title_title":
		return advClause{clause: "titles.title_title " + cmp}, args, ""
	case "title_trans_title":
		return advClause{
			clause: "trans_titles.trans_title_title " + cmp,
			tables: []string{"trans_titles"},
			joins:  []string{"trans_titles.title_id=titles.title_id"},
		}, args, ""
	case "title_copyright":
		return advClause{clause: "SUBSTR(titles.title_copyright,1,4) " + cmp}, args, ""
	case "month":
		return advClause{clause: "SUBSTR(titles.title_copyright,1,7) " + cmp}, args, ""
	case "title_ttype":
		return advClause{clause: "titles.title_ttype " + cmp}, args, ""
	case "title_storylen", "title_content", "title_graphic", "title_non_genre", "title_jvn", "title_nvz":
		return advClause{clause: "titles." + t.Use + " " + cmp}, args, ""
	case "author_canonical", "author_birthplace", "author_birthdate", "author_deathdate":
		return advClause{
			clause: "authors." + t.Use + " " + cmp + " AND canonical_author.ca_status=1",
			tables: []string{"authors", "canonical_author"},
			joins: []string{
				"authors.author_id=canonical_author.author_id",
				"titles.title_id=canonical_author.title_id",
			},
		}, args, ""
	case "author_webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"canonical_author", "webpages"},
			joins: []string{
				"canonical_author.title_id=titles.title_id",
				"canonical_author.author_id=webpages.author_id",
			},
		}, args, ""
	case "series":
		return advClause{
			clause: "series.series_title " + cmp,
			tables: []string{"series"},
			joins:  []string{"series.series_id=titles.series_id"},
		}, args, ""
	case "title_language", "title_language_free":
		return advClause{
			clause: "languages.lang_name " + cmp,
			tables: []string{"languages"},
			joins:  []string{"languages.lang_id=titles.title_language"},
		}, args, ""
	case "title_note":
		return advClause{
			clause: "n1.note_note " + cmp,
			tables: []string{"notes n1"},
			joins:  []string{"n1.note_id=titles.note_id"},
		}, args, ""
	case "title_synopsis":
		return advClause{
			clause: "n2.note_note " + cmp,
			tables: []string{"notes n2"},
			joins:  []string{"n2.note_id=titles.title_synopsis"},
		}, args, ""
	case "title_webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"webpages"},
			joins:  []string{"webpages.title_id=titles.title_id"},
		}, args, ""
	case "single_vote":
		return advClause{
			clause: "votes.rating " + cmp,
			tables: []string{"votes"},
			joins:  []string{"votes.title_id=titles.title_id"},
		}, args, ""
	case "tag":
		return advClause{
			clause: "tags.tag_name " + cmp,
			tables: []string{"tags", "tag_mapping"},
			joins: []string{
				"tag_mapping.title_id=titles.title_id",
				"tags.tag_id=tag_mapping.tag_id",
			},
		}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

func buildPubClause(t advTerm, args []any) (advClause, []any, string) {
	switch t.Use {
	case "pub_isbn":
		// Special case: build OR over all ISBN variations.
		variants := isbnVariations(t.Value)
		var parts []string
		var notOps = map[string]bool{"notexact": true, "notcontains": true}
		sep := " OR "
		if notOps[t.Operator] {
			sep = " AND "
		}
		for _, v := range variants {
			cmp, newArgs, errMsg := padEntry(t.Operator, v, args)
			if errMsg != "" {
				return advClause{}, args, errMsg
			}
			args = newArgs
			parts = append(parts, "pubs.pub_isbn "+cmp)
		}
		return advClause{clause: "(" + strings.Join(parts, sep) + ")"}, args, ""
	}

	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "pub_title":
		return advClause{clause: "pubs.pub_title " + cmp}, args, ""
	case "pub_trans_title":
		return advClause{
			clause: "trans_pubs.trans_pub_title " + cmp,
			tables: []string{"trans_pubs"},
			joins:  []string{"trans_pubs.pub_id=pubs.pub_id"},
		}, args, ""
	case "pub_year":
		return advClause{clause: "SUBSTR(pubs.pub_year,1,4) " + cmp}, args, ""
	case "pub_month":
		return advClause{clause: "SUBSTR(pubs.pub_year,1,7) " + cmp}, args, ""
	case "pub_publisher":
		return advClause{
			clause: "publishers.publisher_name " + cmp,
			tables: []string{"publishers"},
			joins:  []string{"pubs.publisher_id=publishers.publisher_id"},
		}, args, ""
	case "trans_publisher":
		return advClause{
			clause: "trans_publisher.trans_publisher_name " + cmp,
			tables: []string{"trans_publisher"},
			joins:  []string{"trans_publisher.publisher_id=pubs.publisher_id"},
		}, args, ""
	case "pub_series":
		return advClause{
			clause: "pub_series.pub_series_name " + cmp,
			tables: []string{"pub_series"},
			joins:  []string{"pubs.pub_series_id=pub_series.pub_series_id"},
		}, args, ""
	case "trans_pub_series":
		return advClause{
			clause: "trans_pub_series.trans_pub_series_name " + cmp,
			tables: []string{"trans_pub_series"},
			joins:  []string{"trans_pub_series.pub_series_id=pubs.pub_series_id"},
		}, args, ""
	case "pub_catalog":
		return advClause{clause: "pubs.pub_catalog " + cmp}, args, ""
	case "pub_ptype":
		return advClause{clause: "pubs.pub_ptype " + cmp}, args, ""
	case "pub_ctype":
		return advClause{clause: "pubs.pub_ctype " + cmp}, args, ""
	case "pub_price":
		return advClause{clause: "pubs.pub_price " + cmp}, args, ""
	case "pub_pages":
		return advClause{clause: "CAST(pubs.pub_pages AS INTEGER) " + cmp}, args, ""
	case "pub_frontimage":
		return advClause{clause: "pubs.pub_frontimage " + cmp}, args, ""
	case "pub_note":
		return advClause{
			clause: "notes.note_note " + cmp,
			tables: []string{"notes"},
			joins:  []string{"pubs.note_id=notes.note_id"},
		}, args, ""
	case "pub_webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"webpages"},
			joins:  []string{"webpages.pub_id=pubs.pub_id"},
		}, args, ""
	case "author_canonical", "author_birthplace", "author_birthdate", "author_deathdate":
		return advClause{
			clause: "authors." + t.Use + " " + cmp,
			tables: []string{"authors", "pub_authors"},
			joins: []string{
				"authors.author_id=pub_authors.author_id",
				"pubs.pub_id=pub_authors.pub_id",
			},
		}, args, ""
	case "author_webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"pub_authors", "webpages"},
			joins: []string{
				"pub_authors.pub_id=pubs.pub_id",
				"pub_authors.author_id=webpages.author_id",
			},
		}, args, ""
	case "title_language", "title_language_free":
		return advClause{
			clause: "languages.lang_name " + cmp,
			tables: []string{"pub_content", "titles", "languages"},
			joins: []string{
				"pubs.pub_id=pub_content.pub_id",
				"pub_content.title_id=titles.title_id",
				"languages.lang_id=titles.title_language",
			},
		}, args, ""
	case "pub_ver_date":
		return advClause{
			clause: "DATE(primary_verifications.ver_time) " + cmp,
			tables: []string{"primary_verifications"},
			joins:  []string{"pubs.pub_id=primary_verifications.pub_id"},
		}, args, ""
	case "secondary_ver_source":
		return advClause{
			clause: "reference.reference_label " + cmp + " AND verification.ver_status=1",
			tables: []string{"reference", "verification"},
			joins: []string{
				"pubs.pub_id=verification.pub_id",
				"verification.reference_id=reference.reference_id",
			},
		}, args, ""
	case "pub_coverart":
		return advClause{
			clause: "titles.title_ttype='COVERART' AND authors.author_canonical " + cmp,
			tables: []string{"pub_content", "canonical_author", "titles", "authors"},
			joins: []string{
				"pubs.pub_id=pub_content.pub_id",
				"pub_content.title_id=canonical_author.title_id",
				"canonical_author.title_id=titles.title_id",
				"canonical_author.author_id=authors.author_id",
			},
		}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

func buildPublisherClause(t advTerm, args []any) (advClause, []any, string) {
	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "publisher_name":
		return advClause{clause: "publishers.publisher_name " + cmp}, args, ""
	case "trans_publisher_name":
		return advClause{
			clause: "trans_publisher.trans_publisher_name " + cmp,
			tables: []string{"trans_publisher"},
			joins:  []string{"trans_publisher.publisher_id=publishers.publisher_id"},
		}, args, ""
	case "publisher_note":
		return advClause{
			clause: "notes.note_note " + cmp,
			tables: []string{"notes"},
			joins:  []string{"publishers.note_id=notes.note_id"},
		}, args, ""
	case "publisher_webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"webpages"},
			joins:  []string{"webpages.publisher_id=publishers.publisher_id"},
		}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

func buildPubSeriesClause(t advTerm, args []any) (advClause, []any, string) {
	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "pub_series_name":
		return advClause{clause: "pub_series.pub_series_name " + cmp}, args, ""
	case "trans_pub_series_name":
		return advClause{
			clause: "trans_pub_series.trans_pub_series_name " + cmp,
			tables: []string{"trans_pub_series"},
			joins:  []string{"trans_pub_series.pub_series_id=pub_series.pub_series_id"},
		}, args, ""
	case "pub_series_note":
		return advClause{
			clause: "notes.note_note " + cmp,
			tables: []string{"notes"},
			joins:  []string{"pub_series.pub_series_note_id=notes.note_id"},
		}, args, ""
	case "pub_series_webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"webpages"},
			joins:  []string{"webpages.pub_series_id=pub_series.pub_series_id"},
		}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

func buildSeriesClause(t advTerm, args []any) (advClause, []any, string) {
	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "series_title":
		return advClause{clause: "series.series_title " + cmp}, args, ""
	case "trans_series_name":
		return advClause{
			clause: "trans_series.trans_series_name " + cmp,
			tables: []string{"trans_series"},
			joins:  []string{"trans_series.series_id=series.series_id"},
		}, args, ""
	case "parent_series_name":
		return advClause{
			clause: "s2.series_title " + cmp,
			tables: []string{"series s2"},
			joins:  []string{"s2.series_id=series.series_parent"},
		}, args, ""
	case "parent_series_position":
		return advClause{clause: "series.series_parent_position " + cmp}, args, ""
	case "series_note":
		return advClause{
			clause: "notes.note_note " + cmp,
			tables: []string{"notes"},
			joins:  []string{"series.series_note_id=notes.note_id"},
		}, args, ""
	case "series_webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"webpages"},
			joins:  []string{"webpages.series_id=series.series_id"},
		}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

func buildAwardTypeClause(t advTerm, args []any) (advClause, []any, string) {
	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "award_type_short_name":
		return advClause{clause: "award_types.award_type_short_name " + cmp}, args, ""
	case "award_type_name":
		return advClause{clause: "award_types.award_type_name " + cmp}, args, ""
	case "award_type_for":
		return advClause{clause: "award_types.award_type_for " + cmp}, args, ""
	case "award_type_by":
		return advClause{clause: "award_types.award_type_by " + cmp}, args, ""
	case "award_type_poll":
		return advClause{clause: "award_types.award_type_poll " + cmp}, args, ""
	case "award_type_non_genre":
		return advClause{clause: "award_types.award_type_non_genre " + cmp}, args, ""
	case "note":
		return advClause{
			clause: "notes.note_note " + cmp,
			tables: []string{"notes"},
			joins:  []string{"award_types.award_type_note_id=notes.note_id"},
		}, args, ""
	case "webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"webpages"},
			joins:  []string{"webpages.award_type_id=award_types.award_type_id"},
		}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

func buildAwardCatClause(t advTerm, args []any) (advClause, []any, string) {
	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "award_cat_name":
		return advClause{clause: "award_cats.award_cat_name " + cmp}, args, ""
	case "award_type_short_name":
		return advClause{
			clause: "award_types.award_type_short_name " + cmp,
			tables: []string{"award_types"},
			joins:  []string{"award_cats.award_cat_type_id=award_types.award_type_id"},
		}, args, ""
	case "award_type_full_name":
		return advClause{
			clause: "award_types.award_type_name " + cmp,
			tables: []string{"award_types"},
			joins:  []string{"award_cats.award_cat_type_id=award_types.award_type_id"},
		}, args, ""
	case "award_cat_order":
		return advClause{clause: "award_cats.award_cat_order " + cmp}, args, ""
	case "note":
		return advClause{
			clause: "notes.note_note " + cmp,
			tables: []string{"notes"},
			joins:  []string{"award_cats.award_cat_note_id=notes.note_id"},
		}, args, ""
	case "webpage":
		return advClause{
			clause: "webpages.url " + cmp,
			tables: []string{"webpages"},
			joins:  []string{"webpages.award_cat_id=award_cats.award_cat_id"},
		}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

func buildAwardClause(t advTerm, args []any) (advClause, []any, string) {
	cmp, args, errMsg := padEntry(t.Operator, t.Value, args)
	if errMsg != "" {
		return advClause{}, args, errMsg
	}
	switch t.Use {
	case "award_year":
		return advClause{clause: "SUBSTR(awards.award_year,1,4) " + cmp}, args, ""
	case "award_level":
		// Strip trailing description if present (e.g. "1 (Win)" → "1").
		val := strings.Fields(t.Value)
		numStr := t.Value
		if len(val) > 0 {
			numStr = val[0]
		}
		if _, err := strconv.Atoi(numStr); err != nil {
			return advClause{}, args, "Invalid Award Level"
		}
		cmp2, args2, errMsg2 := padEntry(t.Operator, numStr, args[:len(args)-1]) // rebuild without already-appended value
		if errMsg2 != "" {
			return advClause{}, args, errMsg2
		}
		return advClause{clause: "awards.award_level " + cmp2}, args2, ""
	case "title_title":
		return advClause{
			clause: "titles.title_title " + cmp,
			tables: []string{"titles", "title_awards"},
			joins: []string{
				"awards.award_id=title_awards.award_id",
				"title_awards.title_id=titles.title_id",
			},
		}, args, ""
	case "title_ttype":
		return advClause{
			clause: "titles.title_ttype " + cmp,
			tables: []string{"titles", "title_awards"},
			joins: []string{
				"awards.award_id=title_awards.award_id",
				"title_awards.title_id=titles.title_id",
			},
		}, args, ""
	case "award_cat_name":
		return advClause{
			clause: "award_cats.award_cat_name " + cmp,
			tables: []string{"award_cats"},
			joins:  []string{"award_cats.award_cat_id=awards.award_cat_id"},
		}, args, ""
	case "award_type_short_name":
		return advClause{
			clause: "award_types.award_type_short_name " + cmp,
			tables: []string{"award_types", "award_cats"},
			joins:  []string{"awards.award_cat_id=award_cats.award_cat_id AND award_cats.award_cat_type_id=award_types.award_type_id"},
		}, args, ""
	case "award_type_full_name":
		return advClause{
			clause: "award_types.award_type_name " + cmp,
			tables: []string{"award_types", "award_cats"},
			joins:  []string{"awards.award_cat_id=award_cats.award_cat_id AND award_cats.award_cat_type_id=award_types.award_type_id"},
		}, args, ""
	case "note":
		return advClause{
			clause: "notes.note_note " + cmp,
			tables: []string{"notes"},
			joins:  []string{"awards.award_note_id=notes.note_id"},
		}, args, ""
	default:
		return advClause{}, args, "Unknown field: " + t.Use
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Query assembly
// ─────────────────────────────────────────────────────────────────────────────

// buildQuery assembles the full SQL query string and bound args for a given
// search type and set of parsed terms.
// mainTable is the primary table (e.g. "authors").
// expandSort translates sort field names to SQL expressions where needed.
func buildAdvQuery(
	p *advParams,
	st *advSearchType,
	clauseBuilder func(advTerm, []any) (advClause, []any, string),
) (string, []any, string) {

	mainTable := advMainTable(st.TypeName)
	if mainTable == "" {
		return "", nil, "Unsupported search type: " + st.TypeName
	}

	// Collect clauses and extra tables.
	extraTablesSet := map[string]bool{}
	joinsSet := map[string]bool{}
	var termClauses []string
	var args []any

	for _, t := range p.Terms {
		cl, newArgs, errMsg := clauseBuilder(t, args)
		if errMsg != "" {
			return "", nil, errMsg
		}
		args = newArgs
		termClauses = append(termClauses, "("+cl.clause+")")
		for _, tbl := range cl.tables {
			extraTablesSet[tbl] = true
		}
		for _, j := range cl.joins {
			joinsSet[j] = true
		}
	}

	// Build the FROM clause.
	from := mainTable
	for tbl := range extraTablesSet {
		from += ", " + tbl
	}

	// Combine term clauses.
	whereTerms := strings.Join(termClauses, " "+p.Conjunction+" ")
	joinClauses := make([]string, 0, len(joinsSet))
	for j := range joinsSet {
		joinClauses = append(joinClauses, j)
	}
	sort.Strings(joinClauses)

	where := "(" + whereTerms + ")"
	if len(joinClauses) > 0 {
		where += " AND " + strings.Join(joinClauses, " AND ")
	}

	// Build ORDER BY.
	orderBy := expandSortField(p.OrderBy, st.TypeName)

	// Build SELECT.
	var selectClause string
	if p.Action == "count" {
		selectClause = fmt.Sprintf("SELECT COUNT(DISTINCT %s.%s)", mainTable, advIDField(st.TypeName))
	} else {
		switch st.TypeName {
		case "Author", "Title", "Publication":
			selectClause = "SELECT DISTINCT *"
		default:
			selectClause = "SELECT DISTINCT " + mainTable + ".*"
		}
	}

	query := fmt.Sprintf("%s FROM %s WHERE %s ORDER BY %s LIMIT %d, 101",
		selectClause, from, where, orderBy, p.Start)

	return query, args, ""
}

func advMainTable(typeName string) string {
	switch typeName {
	case "Author":
		return "authors"
	case "Title":
		return "titles"
	case "Publication":
		return "pubs"
	case "Publisher":
		return "publishers"
	case "Publication Series":
		return "pub_series"
	case "Series":
		return "series"
	case "Award Type":
		return "award_types"
	case "Award Category":
		return "award_cats"
	case "Award":
		return "awards"
	}
	return ""
}

func advIDField(typeName string) string {
	switch typeName {
	case "Author":
		return "author_id"
	case "Title":
		return "title_id"
	case "Publication":
		return "pub_id"
	case "Publisher":
		return "publisher_id"
	case "Publication Series":
		return "pub_series_id"
	case "Series":
		return "series_id"
	case "Award Type":
		return "award_type_id"
	case "Award Category":
		return "award_cat_id"
	case "Award":
		return "award_id"
	}
	return "id"
}

func expandSortField(field, typeName string) string {
	switch field {
	case "pub_pages":
		field = "CAST(pub_pages AS INTEGER)"
	case "award_level":
		field = "CAST(award_level AS INTEGER)"
	}
	// Add secondary sorts.
	switch typeName {
	case "Author":
		if field != "author_canonical" {
			field += ", author_canonical"
		}
	case "Title":
		if field != "title_title" {
			field += ", title_title"
		}
		if field != "title_copyright" {
			field += ", title_copyright"
		}
	case "Publication":
		if field != "pub_title" {
			field += ", pub_title"
		}
		if field != "pub_year" {
			field += ", pub_year"
		}
	case "Award Type":
		if field != "award_type_short_name" {
			field += ", award_type_short_name"
		}
	case "Award Category":
		if field != "award_cat_name" {
			field += ", award_cat_name"
		}
	case "Award":
		if field != "award_year" {
			field += ", award_year"
		}
	}
	return field
}

// ─────────────────────────────────────────────────────────────────────────────
// Result scanning and rendering
// ─────────────────────────────────────────────────────────────────────────────

func executeAndRenderAdvSearch(w http.ResponseWriter, db *sql.DB, p *advParams, query string, args []any) {
	rows, err := db.Query(query, args...)
	if err != nil {
		fmt.Fprintf(w, "<h2>Database error: %s</h2>\n", ISFDBText(err.Error()))
		return
	}
	defer rows.Close()

	switch p.SearchType {
	case "Author":
		renderAdvAuthors(w, rows, p)
	case "Title":
		renderAdvTitles(w, rows, p, db)
	case "Publication":
		renderAdvPubs(w, rows, p)
	case "Publisher":
		renderAdvPublishers(w, rows, p)
	case "Publication Series":
		renderAdvPubSeries(w, rows, p)
	case "Series":
		renderAdvSeries(w, rows, p)
	case "Award Type":
		renderAdvAwardTypes(w, rows, p)
	case "Award Category":
		renderAdvAwardCats(w, rows, p)
	case "Award":
		renderAdvAwards(w, rows, p, db)
	}
}

func renderAdvAuthors(w http.ResponseWriter, rows *sql.Rows, p *advParams) {
	var authors []*Author
	for rows.Next() {
		var a Author
		if err := rows.Scan(
			&a.AuthorID, &a.AuthorCanonical, &a.AuthorLegalName,
			&a.AuthorBirthPlace, &a.AuthorBirthDate, &a.AuthorDeathDate,
			&a.NoteID, &a.AuthorWikipedia, &a.AuthorViews, &a.AuthorIMDB,
			&a.AuthorMarque, &a.AuthorImage, &a.AuthorAnnualViews,
			&a.AuthorLastName, &a.AuthorLanguage, &a.AuthorNote,
		); err != nil {
			continue
		}
		authors = append(authors, &a)
	}
	if len(authors) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d author(s) found</h3>\n", len(authors))
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr class="generic_table_header"><th>Author</th><th>Legal Name</th><th>Birthdate</th><th>Deathdate</th></tr>`)
	for i, a := range authors {
		if i == 100 {
			break
		}
		rowClass := "table1"
		if i%2 == 1 {
			rowClass = "table2"
		}
		fmt.Fprintf(w, "<tr class=\"%s\">", rowClass)
		fmt.Fprintf(w, "<td>%s</td>", ISFDBLink("author.cgi", a.AuthorID, a.AuthorCanonical.String))
		fmt.Fprintf(w, "<td>%s</td>", ISFDBText(a.AuthorLegalName.String))
		fmt.Fprintf(w, "<td>%s</td>", ISFDBText(a.AuthorBirthDate.String))
		fmt.Fprintf(w, "<td>%s</td>", ISFDBText(a.AuthorDeathDate.String))
		fmt.Fprintln(w, "</tr>")
	}
	fmt.Fprintln(w, "</table>")
	printAdvPageButtons(w, p, len(authors))
}

func renderAdvTitles(w http.ResponseWriter, rows *sql.Rows, p *advParams, db *sql.DB) {
	titles, err := scanTitles(rows)
	if err != nil || len(titles) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d title(s) found</h3>\n", len(titles))
	printSearchTitleTable(w, titles)
	printAdvPageButtons(w, p, len(titles))
}

func renderAdvPubs(w http.ResponseWriter, rows *sql.Rows, p *advParams) {
	pubs, err := scanPubs(rows)
	if err != nil || len(pubs) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d publication(s) found</h3>\n", len(pubs))
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr class="generic_table_header"><th>Title</th><th>Date</th><th>ISBN</th></tr>`)
	for i, pub := range pubs {
		if i == 100 {
			break
		}
		rowClass := "table1"
		if i%2 == 1 {
			rowClass = "table2"
		}
		isbn := pub.PubISBN.String
		fmt.Fprintf(w, "<tr class=\"%s\"><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			rowClass,
			ISFDBLink("pub.cgi", pub.PubID, pub.PubTitle.String),
			ISFDBText(pub.PubYear.String),
			ISFDBText(isbn))
	}
	fmt.Fprintln(w, "</table>")
	printAdvPageButtons(w, p, len(pubs))
}

func renderAdvPublishers(w http.ResponseWriter, rows *sql.Rows, p *advParams) {
	var pubs []*Publisher
	for rows.Next() {
		var pub Publisher
		if err := rows.Scan(&pub.PublisherID, &pub.PublisherName, &pub.PublisherWikipedia, &pub.NoteID); err != nil {
			continue
		}
		pubs = append(pubs, &pub)
	}
	if len(pubs) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d publisher(s) found</h3>\n", len(pubs))
	printSearchPublisherTable(w, pubs)
	printAdvPageButtons(w, p, len(pubs))
}

func renderAdvPubSeries(w http.ResponseWriter, rows *sql.Rows, p *advParams) {
	var series []*PubSeriesRecord
	for rows.Next() {
		var ps PubSeriesRecord
		if err := rows.Scan(&ps.PubSeriesID, &ps.PubSeriesName, &ps.Wikipedia, &ps.NoteID); err != nil {
			continue
		}
		series = append(series, &ps)
	}
	if len(series) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d publication series found</h3>\n", len(series))
	printSearchPubSeriesTable(w, series)
	printAdvPageButtons(w, p, len(series))
}

func renderAdvSeries(w http.ResponseWriter, rows *sql.Rows, p *advParams) {
	type seriesRow struct {
		ID       int
		Title    sql.NullString
		ParentID sql.NullInt32
		Type     sql.NullInt32
		ParentPos sql.NullInt32
		NoteID   sql.NullInt32
	}
	var results []seriesRow
	for rows.Next() {
		var s seriesRow
		if err := rows.Scan(&s.ID, &s.Title, &s.ParentID, &s.Type, &s.ParentPos, &s.NoteID); err != nil {
			continue
		}
		results = append(results, s)
	}
	if len(results) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d series found</h3>\n", len(results))
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr class="generic_table_header"><th>Series</th></tr>`)
	for i, s := range results {
		if i == 100 {
			break
		}
		rowClass := "table1"
		if i%2 == 1 {
			rowClass = "table2"
		}
		fmt.Fprintf(w, "<tr class=\"%s\"><td>%s</td></tr>\n",
			rowClass, ISFDBLink("pe.cgi", s.ID, s.Title.String))
	}
	fmt.Fprintln(w, "</table>")
	printAdvPageButtons(w, p, len(results))
}

func renderAdvAwardTypes(w http.ResponseWriter, rows *sql.Rows, p *advParams) {
	var types []*AwardType
	for rows.Next() {
		var at AwardType
		if err := rows.Scan(
			&at.AwardTypeID, &at.AwardTypeCode, &at.AwardTypeName,
			&at.AwardTypeWikipedia, &at.AwardTypeNoteID, &at.AwardTypeBy,
			&at.AwardTypeFor, &at.AwardTypeShortName, &at.AwardTypePoll,
			&at.AwardTypeNonGenre,
		); err != nil {
			continue
		}
		types = append(types, &at)
	}
	if len(types) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d award type(s) found</h3>\n", len(types))
	printSearchAwardTypeTable(w, types)
	printAdvPageButtons(w, p, len(types))
}

func renderAdvAwardCats(w http.ResponseWriter, rows *sql.Rows, p *advParams) {
	var cats []*AwardCat
	for rows.Next() {
		var ac AwardCat
		if err := rows.Scan(&ac.AwardCatID, &ac.AwardCatName, &ac.AwardCatTypeID, &ac.AwardCatOrder, &ac.AwardCatNoteID); err != nil {
			continue
		}
		cats = append(cats, &ac)
	}
	if len(cats) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d award category(ies) found</h3>\n", len(cats))
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr class="generic_table_header"><th>Category</th></tr>`)
	for i, ac := range cats {
		if i == 100 {
			break
		}
		rowClass := "table1"
		if i%2 == 1 {
			rowClass = "table2"
		}
		fmt.Fprintf(w, "<tr class=\"%s\"><td>%s</td></tr>\n",
			rowClass,
			ISFDBLink("award_category.cgi", ac.AwardCatID, ac.AwardCatName.String))
	}
	fmt.Fprintln(w, "</table>")
	printAdvPageButtons(w, p, len(cats))
}

func renderAdvAwards(w http.ResponseWriter, rows *sql.Rows, p *advParams, db *sql.DB) {
	var awards []*Award
	for rows.Next() {
		var a Award
		if err := rows.Scan(
			&a.AwardID, &a.AwardTitle, &a.AwardAuthor, &a.AwardYear, &a.AwardTType,
			&a.AwardAType, &a.AwardLevel, &a.AwardMovie, &a.AwardTypeID,
			&a.AwardCatID, &a.AwardNoteID,
		); err != nil {
			continue
		}
		awards = append(awards, &a)
	}
	if len(awards) == 0 {
		fmt.Fprintln(w, "<h2>No records found</h2>")
		return
	}
	fmt.Fprintf(w, "<h3>%d award(s) found</h3>\n", len(awards))
	fmt.Fprintln(w, `<table class="generic_table">`)
	fmt.Fprintln(w, `<tr class="generic_table_header"><th>Year</th><th>Level</th><th>Title/Author</th></tr>`)
	for i, a := range awards {
		if i == 100 {
			break
		}
		rowClass := "table1"
		if i%2 == 1 {
			rowClass = "table2"
		}
		fmt.Fprintf(w, "<tr class=\"%s\"><td>%s</td><td>%s</td><td>%s / %s</td></tr>\n",
			rowClass,
			ISFDBText(a.AwardYear.String),
			ISFDBText(a.AwardLevel.String),
			ISFDBText(a.AwardTitle.String),
			ISFDBText(a.AwardAuthor.String))
	}
	fmt.Fprintln(w, "</table>")
	printAdvPageButtons(w, p, len(awards))
}

// printAdvPageButtons renders Previous/Next page buttons when applicable.
func printAdvPageButtons(w http.ResponseWriter, p *advParams, count int) {
	q := ""
	if p.Start > 99 {
		prev := p.Start - 100
		q += fmt.Sprintf("<form method=\"GET\" action=\"%s://%s/adv_search_results.cgi\">",
			PROTOCOL, HTMLHOST)
		q += fmt.Sprintf("<input name=\"START\" value=\"%d\" type=\"hidden\">", prev)
		// Reconstruct the other params from the URL.
		q += advHiddenFormFields(p)
		q += fmt.Sprintf("<input type=\"submit\" value=\"Previous page (%d - %d)\">", p.Start-99, p.Start)
		q += "</form>"
	}
	if count > 100 {
		next := p.Start + 100
		q += fmt.Sprintf("<form method=\"GET\" action=\"%s://%s/adv_search_results.cgi\">",
			PROTOCOL, HTMLHOST)
		q += fmt.Sprintf("<input name=\"START\" value=\"%d\" type=\"hidden\">", next)
		q += advHiddenFormFields(p)
		q += fmt.Sprintf("<input type=\"submit\" value=\"Next page (%d - %d)\">", p.Start+101, p.Start+200)
		q += "</form>"
	}
	if q != "" {
		fmt.Fprintln(w, `<div class="button-container">`)
		fmt.Fprintln(w, q)
		fmt.Fprintln(w, `</div>`)
	}
}

// advHiddenFormFields builds hidden form inputs reconstructing the search
// parameters (TYPE, ORDERBY, C, ACTION, USE_N, O_N, TERM_N).
func advHiddenFormFields(p *advParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<input name=\"TYPE\" value=\"%s\" type=\"hidden\">", ISFDBText(p.SearchType))
	fmt.Fprintf(&b, "<input name=\"ORDERBY\" value=\"%s\" type=\"hidden\">", ISFDBText(p.OrderBy))
	fmt.Fprintf(&b, "<input name=\"C\" value=\"%s\" type=\"hidden\">", ISFDBText(p.Conjunction))
	fmt.Fprintf(&b, "<input name=\"ACTION\" value=\"%s\" type=\"hidden\">", ISFDBText(p.Action))
	for i, t := range p.Terms {
		n := i + 1
		fmt.Fprintf(&b, "<input name=\"USE_%d\" value=\"%s\" type=\"hidden\">", n, ISFDBText(t.Use))
		fmt.Fprintf(&b, "<input name=\"O_%d\" value=\"%s\" type=\"hidden\">", n, ISFDBText(t.Operator))
		fmt.Fprintf(&b, "<input name=\"TERM_%d\" value=\"%s\" type=\"hidden\">", n, ISFDBText(t.Value))
	}
	return b.String()
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP handler
// ─────────────────────────────────────────────────────────────────────────────

// AdvSearchResultsHandler serves /adv_search_results.cgi
func AdvSearchResultsHandler(w http.ResponseWriter, r *http.Request) {
	p, errMsg := parseAdvParams(r)
	if errMsg != "" {
		HTMLheader(w, "Advanced Search")
		PrintNavbar(w, "adv_search_results", "", "")
		fmt.Fprintf(w, "<h2>Error: %s</h2>\n", ISFDBText(errMsg))
		HTMLtrailer(w)
		return
	}

	st := advSearchTypeByTypeName(p.SearchType)
	if st == nil {
		HTMLheader(w, "Advanced Search")
		PrintNavbar(w, "adv_search_results", "", "")
		fmt.Fprintf(w, "<h2>Error: Unknown search type: %s</h2>\n", ISFDBText(p.SearchType))
		HTMLtrailer(w)
		return
	}

	// Validate ORDER BY against the type's declared sort fields.
	validSort := false
	for _, sf := range st.SortBy {
		if sf.Field == p.OrderBy {
			validSort = true
			break
		}
	}
	if !validSort {
		HTMLheader(w, "Advanced Search")
		PrintNavbar(w, "adv_search_results", "", "")
		fmt.Fprintf(w, "<h2>Error: Unknown sort field: %s</h2>\n", ISFDBText(p.OrderBy))
		HTMLtrailer(w)
		return
	}

	// Pick the clause builder for this type.
	clauseBuilders := map[string]func(advTerm, []any) (advClause, []any, string){
		"Author":             buildAuthorClause,
		"Title":              buildTitleClause,
		"Publication":        buildPubClause,
		"Publisher":          buildPublisherClause,
		"Publication Series": buildPubSeriesClause,
		"Series":             buildSeriesClause,
		"Award Type":         buildAwardTypeClause,
		"Award Category":     buildAwardCatClause,
		"Award":              buildAwardClause,
	}
	builder, ok := clauseBuilders[p.SearchType]
	if !ok {
		HTMLheader(w, "Advanced Search")
		PrintNavbar(w, "adv_search_results", "", "")
		fmt.Fprintf(w, "<h2>Error: Unsupported search type</h2>\n")
		HTMLtrailer(w)
		return
	}

	query, args, qErr := buildAdvQuery(p, st, builder)
	if qErr != "" {
		HTMLheader(w, "Advanced Search")
		PrintNavbar(w, "adv_search_results", "", "")
		fmt.Fprintf(w, "<h2>Error: %s</h2>\n", ISFDBText(qErr))
		HTMLtrailer(w)
		return
	}

	HTMLheader(w, "Advanced "+p.SearchType+" Search")
	PrintNavbar(w, "adv_search_results", "", "")

	// Show selection criteria summary.
	fmt.Fprintf(w, "<b>Selection Criteria (joined using %s):</b>\n", ISFDBText(p.Conjunction))
	for _, t := range p.Terms {
		// Resolve field label.
		fieldLabel := t.Use
		for _, f := range st.Fields {
			if f.Field == t.Use {
				fieldLabel = f.Label
				break
			}
		}
		// Resolve operator label.
		opLabel := t.Operator
		for _, op := range advSearchOperators {
			if op.Key == t.Operator {
				opLabel = op.Label
				break
			}
		}
		fmt.Fprintf(w, "<br>%s %s %s\n",
			ISFDBText(fieldLabel), ISFDBText(opLabel), ISFDBText(t.Value))
	}
	// Sort name label.
	sortLabel := p.OrderBy
	for _, sf := range st.SortBy {
		if sf.Field == p.OrderBy {
			sortLabel = sf.Label
			break
		}
	}
	fmt.Fprintf(w, "<br>Sort by %s\n", ISFDBText(sortLabel))

	db := DB
	if p.Action == "count" {
		rows, err := db.Query(query, args...)
		if err != nil {
			fmt.Fprintf(w, "<h2>Database error: %s</h2>\n", ISFDBText(err.Error()))
		} else {
			defer rows.Close()
			var count int
			if rows.Next() {
				rows.Scan(&count)
			}
			fmt.Fprintf(w, "<h2>Count of matching records: %d</h2>\n", count)
		}
	} else {
		executeAndRenderAdvSearch(w, db, p, query, args)
	}

	HTMLtrailer(w)
}
