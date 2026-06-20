// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
)

// This file serves as the interface to the database. It defines:
// - The Template struct, which holds data from the templates table
// - SQL functions to load template data

type Template struct {
	TemplateID        int
	TemplateName      sql.NullString
	TemplateDisplay   sql.NullString
	TemplateType      sql.NullString
	TemplateURL       sql.NullString
	TemplateMouseover sql.NullString
}

// TemplateEntry is the in-memory representation of a template used for
// note formatting. Mirrors the Python tuple: (url, display, mouseover).
type TemplateEntry struct {
	URL       string
	Display   string
	Mouseover string
}

// Templates is a map of template_name to TemplateEntry, loaded once at startup.
var Templates map[string]TemplateEntry

// loadTemplates loads all templates from the database into a map keyed by template name.
// Internal URL templates are rewritten to point at the local server.
func loadTemplates(db *sql.DB) (map[string]TemplateEntry, error) {
	rows, err := db.Query("SELECT template_id, template_name, template_display, template_type, template_url, template_mouseover FROM templates")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make(map[string]TemplateEntry)
	for rows.Next() {
		var t Template
		if err := rows.Scan(
			&t.TemplateID, &t.TemplateName, &t.TemplateDisplay,
			&t.TemplateType, &t.TemplateURL, &t.TemplateMouseover,
		); err != nil {
			return nil, err
		}

		if !t.TemplateName.Valid {
			continue
		}

		urlStr := t.TemplateURL.String
		if t.TemplateType.Valid && t.TemplateType.String == "Internal URL" {
			urlStr = fmt.Sprintf("%s://%s/%s", PROTOCOL, HTMLHOST, urlStr)
		}

		entry := TemplateEntry{URL: urlStr}
		if t.TemplateDisplay.Valid {
			entry.Display = t.TemplateDisplay.String
		}
		if t.TemplateMouseover.Valid {
			entry.Mouseover = t.TemplateMouseover.String
		}

		templates[t.TemplateName.String] = entry
	}
	return templates, rows.Err()
}
