// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import "database/sql"

// Tag holds one row from the tags table.
type Tag struct {
	TagID     int
	TagName   sql.NullString
	TagStatus int // 0 = public, non-zero = private
}

// SQLSearchTags searches for tags by name.
func SQLSearchTags(db *sql.DB, target string) ([]*Tag, error) {
	rows, err := db.Query(
		"SELECT tag_id, tag_name, tag_status FROM tags WHERE tag_name LIKE ? ORDER BY tag_name",
		"%"+target+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []*Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.TagID, &t.TagName, &t.TagStatus); err != nil {
			return nil, err
		}
		result = append(result, &t)
	}
	return result, rows.Err()
}
