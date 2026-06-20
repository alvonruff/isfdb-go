// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

// advSearchField is one searchable field for a given record type.
type advSearchField struct {
	Field string // internal field key, used as USE_N value
	Label string // human-readable label shown in the dropdown
}

// advOp is one comparison operator.
type advOp struct {
	Key   string
	Label string
}

// advSearchType bundles all metadata for one searchable record type.
type advSearchType struct {
	URLKey   string           // the ?key used in adv_search_selection.cgi URLs
	TypeName string           // the TYPE= value used in form submissions
	Fields   []advSearchField // searchable fields
	SortBy   []advSearchField // available sort fields
	Messages []string         // notes shown at top of the form
}

// advSearchOperators is the ordered list of comparison operators.
var advSearchOperators = []advOp{
	{"exact", "is exactly"},
	{"notexact", "is not exactly"},
	{"contains", "contains"},
	{"notcontains", "does not contain"},
	{"starts_with", "starts with"},
	{"ends_with", "ends with"},
}

// advMaxTerms is the number of term rows shown on the selection form.
const advMaxTerms = 10

// advSearchTypes is the ordered list of searchable record types.
var advSearchTypes = []advSearchType{
	{
		URLKey:   "author",
		TypeName: "Author",
		Fields: []advSearchField{
			{"author_canonical", "Canonical Name"},
			{"author_trans_name", "Transliterated Name"},
			{"author_lastname", "Directory Entry"},
			{"author_legalname", "Legal Name"},
			{"author_trans_legalname", "Transliterated Legal Name"},
			{"author_birthplace", "Birth Place"},
			{"author_birthdate", "Birthdate"},
			{"author_deathdate", "Deathdate"},
			{"author_language", "Working Language (list)"},
			{"author_language_free", "Working Language (free form)"},
			{"author_image", "Author Image"},
			{"author_webpage", "Webpage"},
			{"author_email", "E-mail"},
			{"author_pseudos", "Alternate Name"},
			{"author_note", "Note"},
		},
		SortBy: []advSearchField{
			{"author_canonical", "Canonical Name"},
			{"author_lastname", "Directory Entry"},
			{"author_legalname", "Legal Name"},
			{"author_birthplace", "Birth Place"},
			{"author_birthdate", "Birthdate"},
			{"author_deathdate", "Deathdate"},
		},
	},
	{
		URLKey:   "title",
		TypeName: "Title",
		Fields: []advSearchField{
			{"title_title", "Title"},
			{"title_trans_title", "Transliterated Title"},
			{"author_canonical", "Author's Name"},
			{"author_birthplace", "Author's Birth Place"},
			{"author_birthdate", "Author's Birthdate"},
			{"author_deathdate", "Author's Deathdate"},
			{"author_webpage", "Author's Webpage"},
			{"reviewee", "Reviewed Author"},
			{"interviewee", "Interviewed Author"},
			{"title_copyright", "Title Year"},
			{"month", "Title Month"},
			{"title_storylen", "Length"},
			{"title_content", "Content (omnibus only)"},
			{"title_ttype", "Title Type"},
			{"title_note", "Notes"},
			{"title_synopsis", "Synopsis"},
			{"single_vote", "Single User Vote"},
			{"series", "Series"},
			{"title_language", "Title Language (list)"},
			{"title_language_free", "Title Language (free form)"},
			{"title_webpage", "Title Webpage"},
			{"tag", "Tag"},
			{"title_jvn", "Juvenile"},
			{"title_nvz", "Novelization"},
			{"title_non_genre", "Non-Genre"},
			{"title_graphic", "Graphic Format"},
		},
		SortBy: []advSearchField{
			{"title_title", "Title"},
			{"title_copyright", "Date"},
			{"title_ttype", "Title Type"},
		},
		Messages: []string{
			"When specifying multiple authors and/or multiple tags, OR is supported but AND is not",
		},
	},
	{
		URLKey:   "series",
		TypeName: "Series",
		Fields: []advSearchField{
			{"series_title", "Series Name"},
			{"trans_series_name", "Transliterated Series Name"},
			{"parent_series_name", "Parent Series Name"},
			{"parent_series_position", "Position within Parent Series"},
			{"series_note", "Notes"},
			{"series_webpage", "Webpage"},
		},
		SortBy: []advSearchField{
			{"series_title", "Series Name"},
		},
	},
	{
		URLKey:   "pub",
		TypeName: "Publication",
		Fields: []advSearchField{
			{"pub_title", "Title"},
			{"pub_trans_title", "Transliterated Title"},
			{"pub_ctype", "Publication Type"},
			{"author_canonical", "Author's Name"},
			{"author_birthplace", "Author's Birth Place"},
			{"author_birthdate", "Author's Birthdate"},
			{"author_deathdate", "Author's Deathdate"},
			{"author_webpage", "Author's Webpage"},
			{"pub_year", "Publication Year"},
			{"pub_month", "Publication Month"},
			{"pub_publisher", "Publisher"},
			{"trans_publisher", "Transliterated Publisher"},
			{"pub_series", "Publication Series"},
			{"trans_pub_series", "Transliterated Publication Series"},
			{"title_language", "Language of an Included Title (list)"},
			{"title_language_free", "Language of an Included Title (free form)"},
			{"pub_isbn", "ISBN"},
			{"pub_catalog", "Catalog ID"},
			{"pub_price", "Price"},
			{"pub_pages", "Page Count"},
			{"pub_coverart", "Cover Artist"},
			{"pub_ptype", "Format"},
			{"pub_ver_date", "Primary Verification Date"},
			{"secondary_ver_source", "Secondary Verification Source"},
			{"pub_webpage", "Publication Webpage"},
			{"pub_note", "Notes"},
			{"pub_frontimage", "Image URL"},
		},
		SortBy: []advSearchField{
			{"pub_title", "Title"},
			{"pub_ctype", "Publication Type"},
			{"pub_year", "Date"},
			{"pub_isbn", "ISBN"},
			{"pub_catalog", "Catalog ID"},
			{"pub_price", "Price"},
			{"pub_pages", "Page Count"},
			{"pub_ptype", "Format"},
			{"pub_frontimage", "Image URL"},
		},
		Messages: []string{
			"ISBN searches ignore dashes and search for both ISBN-10 and ISBN-13",
			"When specifying multiple Authors, Cover Artists, Publication Webpages, Primary Verifiers or Secondary Verification Sources, OR is supported but AND is not",
		},
	},
	{
		URLKey:   "publisher",
		TypeName: "Publisher",
		Fields: []advSearchField{
			{"publisher_name", "Publisher Name"},
			{"trans_publisher_name", "Transliterated Publisher Name"},
			{"publisher_note", "Notes"},
			{"publisher_webpage", "Webpage"},
		},
		SortBy: []advSearchField{
			{"publisher_name", "Publisher Name"},
		},
	},
	{
		URLKey:   "pub_series",
		TypeName: "Publication Series",
		Fields: []advSearchField{
			{"pub_series_name", "Publication Series Name"},
			{"trans_pub_series_name", "Transliterated Publication Series Name"},
			{"pub_series_note", "Notes"},
			{"pub_series_webpage", "Webpage"},
		},
		SortBy: []advSearchField{
			{"pub_series_name", "Publication Series Name"},
		},
	},
	{
		URLKey:   "award_type",
		TypeName: "Award Type",
		Fields: []advSearchField{
			{"award_type_short_name", "Award Type Name (Short)"},
			{"award_type_name", "Award Type Name (Full)"},
			{"award_type_for", "Awarded For"},
			{"award_type_by", "Awarded By"},
			{"award_type_poll", "Poll"},
			{"award_type_non_genre", "Non-Genre"},
			{"note", "Notes"},
			{"webpage", "Webpage"},
		},
		SortBy: []advSearchField{
			{"award_type_short_name", "Short Award Type Name"},
			{"award_type_name", "Full Award Type Name"},
			{"award_type_for", "Awarded For"},
			{"award_type_by", "Awarded By"},
			{"award_type_poll", "Poll"},
			{"award_type_non_genre", "Non-Genre"},
		},
	},
	{
		URLKey:   "award_cat",
		TypeName: "Award Category",
		Fields: []advSearchField{
			{"award_cat_name", "Award Category Name"},
			{"award_type_short_name", "Parent Award Type Short Name"},
			{"award_type_full_name", "Parent Award Type Full Name"},
			{"award_cat_order", "Award Category Order"},
			{"note", "Notes"},
			{"webpage", "Webpage"},
		},
		SortBy: []advSearchField{
			{"award_cat_name", "Award Category Name"},
			{"award_cat_order", "Award Category Order"},
		},
	},
	{
		URLKey:   "award",
		TypeName: "Award",
		Fields: []advSearchField{
			{"award_year", "Award Year"},
			{"award_level", "Award Level"},
			{"title_title", "Title (for title-based awards)"},
			{"title_ttype", "Title Type"},
			{"award_cat_name", "Award Category"},
			{"award_type_short_name", "Award Type Short Name"},
			{"award_type_full_name", "Award Type Full Name"},
			{"note", "Notes"},
		},
		SortBy: []advSearchField{
			{"award_year", "Award Year"},
			{"award_level", "Award Level"},
		},
	},
}

// advSearchTypeByURLKey returns the advSearchType for the given URL key (e.g. "author").
func advSearchTypeByURLKey(key string) *advSearchType {
	for i := range advSearchTypes {
		if advSearchTypes[i].URLKey == key {
			return &advSearchTypes[i]
		}
	}
	return nil
}

// advSearchTypeByTypeName returns the advSearchType for the given TYPE= value (e.g. "Author").
func advSearchTypeByTypeName(name string) *advSearchType {
	for i := range advSearchTypes {
		if advSearchTypes[i].TypeName == name {
			return &advSearchTypes[i]
		}
	}
	return nil
}
