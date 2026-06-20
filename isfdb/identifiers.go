// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"strings"
)

// Identifier holds one row from the identifiers table plus its type metadata.
type Identifier struct {
	IdentifierID     int
	TypeID           int
	Value            string
	TypeName         string // short name, e.g. "ASIN"
	TypeFullName     string // long name, e.g. "Amazon Standard Identification Number"
}

// IdentifierSite holds one row from the identifier_sites table.
type IdentifierSite struct {
	SiteID   int
	TypeID   int
	Position int
	SiteURL  string // contains %s placeholder for the identifier value
	SiteName string
}

// SQLloadPubIdentifiers returns all identifiers for a publication, including
// type metadata, ordered by type name then value.
func SQLloadPubIdentifiers(db *sql.DB, pubID int) ([]*Identifier, error) {
	rows, err := db.Query(
		"SELECT i.identifier_id, i.identifier_type_id, i.identifier_value, "+
			"t.identifier_type_name, t.identifier_type_full_name "+
			"FROM identifiers i "+
			"JOIN identifier_types t ON i.identifier_type_id = t.identifier_type_id "+
			"WHERE i.pub_id = ? "+
			"ORDER BY t.identifier_type_name, i.identifier_value",
		pubID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Identifier
	for rows.Next() {
		var id Identifier
		var value, typeName, typeFullName sql.NullString
		if err := rows.Scan(&id.IdentifierID, &id.TypeID, &value, &typeName, &typeFullName); err != nil {
			return nil, err
		}
		id.Value = value.String
		id.TypeName = typeName.String
		id.TypeFullName = typeFullName.String
		result = append(result, &id)
	}
	return result, rows.Err()
}

// SQLloadIdentifierSitesBatch returns a map of typeID -> []IdentifierSite
// for the given set of type IDs.
func SQLloadIdentifierSitesBatch(db *sql.DB, typeIDs []int) (map[int][]*IdentifierSite, error) {
	result := make(map[int][]*IdentifierSite)
	if len(typeIDs) == 0 {
		return result, nil
	}
	ph := make([]string, len(typeIDs))
	args := make([]any, len(typeIDs))
	for i, id := range typeIDs {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		"SELECT identifier_site_id, identifier_type_id, site_position, site_url, site_name "+
			"FROM identifier_sites WHERE identifier_type_id IN ("+strings.Join(ph, ",")+")" +
			" ORDER BY identifier_type_id, site_position",
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s IdentifierSite
		var siteURL, siteName sql.NullString
		if err := rows.Scan(&s.SiteID, &s.TypeID, &s.Position, &siteURL, &siteName); err != nil {
			return nil, err
		}
		s.SiteURL = siteURL.String
		s.SiteName = siteName.String
		result[s.TypeID] = append(result[s.TypeID], &s)
	}
	return result, rows.Err()
}

// formatExternalIDLink builds a single external ID hyperlink.
// Spaces in the value are stripped (matching Python str.replace(value,' ','')).
func formatExternalIDLink(siteURL, value, displayValue string) string {
	urlValue := strings.ReplaceAll(value, " ", "")
	href := fmt.Sprintf(siteURL, urlValue)
	return fmt.Sprintf(`<a href="%s" target="_blank">%s</a>`, href, ISFDBText(displayValue))
}

// formatExternalIDSite renders the value portion of one identifier entry.
func formatExternalIDSite(typeSites []*IdentifierSite, value string) string {
	count := len(typeSites)
	if count == 0 {
		return ISFDBText(value)
	}
	if count == 1 {
		return " " + formatExternalIDLink(typeSites[0].SiteURL, value, value)
	}
	// Multiple sites: plain value followed by named links
	result := " " + ISFDBText(value)
	lastURL := ""
	for _, site := range typeSites {
		result += " " + formatExternalIDLink(site.SiteURL, value, site.SiteName)
		lastURL = site.SiteURL
	}
	if strings.Contains(lastURL, ".amazon.") {
		result += " (US/UK earn commissions)"
	}
	return result
}

// PrintExternalIDs renders the External IDs section for a publication.
// Output matches the Python printExternalIDs(format='list') behaviour:
// one <li> per identifier type, values concatenated on the same line,
// type label as <abbr class="template" title="Full Name">ShortName</abbr>:
func PrintExternalIDs(w interface {
	Write([]byte) (int, error)
}, ids []*Identifier, sites map[int][]*IdentifierSite) {
	// Group by TypeName, preserving sorted order (ids already sorted by type_name).
	type group struct {
		typeName     string
		typeFullName string
		typeID       int
		values       []string
	}
	var groups []group
	groupIdx := make(map[string]int)

	for _, id := range ids {
		if i, ok := groupIdx[id.TypeName]; ok {
			groups[i].values = append(groups[i].values, id.Value)
		} else {
			groupIdx[id.TypeName] = len(groups)
			groups = append(groups, group{
				typeName:     id.TypeName,
				typeFullName: id.TypeFullName,
				typeID:       id.TypeID,
				values:       []string{id.Value},
			})
		}
	}

	fmt.Fprintln(w, `  <ul class="noindent">`)
	for _, g := range groups {
		// Type label: <abbr class="template" title="Full Name">ShortName</abbr>:
		label := fmt.Sprintf(`<abbr class="template" title="%s">%s</abbr>:`,
			ISFDBText(g.typeFullName), ISFDBText(g.typeName))
		typeSites := sites[g.typeID]
		line := label
		for _, val := range g.values {
			line += formatExternalIDSite(typeSites, val)
		}
		fmt.Fprintf(w, "<li> %s\n", line)
	}
	fmt.Fprintln(w, `  </ul>`)
}
