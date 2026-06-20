// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import "database/sql"

// RecognizedDomain holds a row from the recognized_domains table.
type RecognizedDomain struct {
	DomainID            int
	DomainName          string
	SiteName            string
	SiteURL             sql.NullString
	LinkingAllowed      sql.NullInt32
	RequiredSegment     sql.NullString
	ExplicitLinkRequired sql.NullInt32
}

// SQLLoadRecognizedDomains returns all rows from the recognized_domains table.
func SQLLoadRecognizedDomains(db *sql.DB) ([]RecognizedDomain, error) {
	rows, err := db.Query("SELECT domain_id, domain_name, site_name, site_url, linking_allowed, required_segment, explicit_link_required FROM recognized_domains")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []RecognizedDomain
	for rows.Next() {
		var d RecognizedDomain
		if err := rows.Scan(&d.DomainID, &d.DomainName, &d.SiteName, &d.SiteURL,
			&d.LinkingAllowed, &d.RequiredSegment, &d.ExplicitLinkRequired); err != nil {
			return nil, err
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

// This file serves as the interface to the database. It defines:
// - The Webpage struct, which holds data from the webpages table
// - SQL functions to load webpage URLs for various record types

type Webpage struct {
	WebpageID   int
	AuthorID    sql.NullInt32
	PublisherID sql.NullInt32
	URL         sql.NullString
	PubSeriesID sql.NullInt32
	TitleID     sql.NullInt32
	AwardTypeID sql.NullInt32
	SeriesID    sql.NullInt32
	AwardCatID  sql.NullInt32
	PubID       sql.NullInt32
}

// sqlLoadWebpageURLs is an internal helper that queries webpages by a given
// column name and ID value, returning a slice of URL strings.
func sqlLoadWebpageURLs(db *sql.DB, column string, id int) ([]string, error) {
	rows, err := db.Query("SELECT url FROM webpages WHERE "+column+"=?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []string
	for rows.Next() {
		var url sql.NullString
		if err := rows.Scan(&url); err != nil {
			return nil, err
		}
		if url.Valid && url.String != "" {
			urls = append(urls, url.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

// SQLloadWebpages returns all webpage URLs for a given author.
func SQLloadWebpages(db *sql.DB, authorID int) ([]string, error) {
	return sqlLoadWebpageURLs(db, "author_id", authorID)
}

// SQLloadPublisherWebpages returns all webpage URLs for a given publisher.
func SQLloadPublisherWebpages(db *sql.DB, publisherID int) ([]string, error) {
	return sqlLoadWebpageURLs(db, "publisher_id", publisherID)
}

// SQLloadPubSeriesWebpages returns all webpage URLs for a given publication series.
func SQLloadPubSeriesWebpages(db *sql.DB, pubSeriesID int) ([]string, error) {
	return sqlLoadWebpageURLs(db, "pub_series_id", pubSeriesID)
}

// SQLloadTitleWebpages returns all webpage URLs for a given title.
func SQLloadTitleWebpages(db *sql.DB, titleID int) ([]string, error) {
	return sqlLoadWebpageURLs(db, "title_id", titleID)
}

// SQLloadPubWebpages returns all webpage URLs for a given publication.
func SQLloadPubWebpages(db *sql.DB, pubID int) ([]string, error) {
	return sqlLoadWebpageURLs(db, "pub_id", pubID)
}

// SQLloadSeriesWebpages returns all webpage URLs for a given title series.
func SQLloadSeriesWebpages(db *sql.DB, seriesID int) ([]string, error) {
	return sqlLoadWebpageURLs(db, "series_id", seriesID)
}

// SQLloadAwardCatWebpages returns all webpage URLs for an award category.
func SQLloadAwardCatWebpages(db *sql.DB, catID int) ([]string, error) {
	return sqlLoadWebpageURLs(db, "award_cat_id", catID)
}

// SQLloadAwardTypeWebpages returns all webpage URLs for an award type.
func SQLloadAwardTypeWebpages(db *sql.DB, typeID int) ([]string, error) {
	return sqlLoadWebpageURLs(db, "award_type_id", typeID)
}
