// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"database/sql"
	"fmt"
	"strings"
)

// isbnRange holds one row from the isbn_ranges table.
type isbnRange struct {
	startValue      int
	endValue        int
	prefixLength    int
	publisherLength int
}

// isbnRanges is loaded once at startup.
var isbnRanges []isbnRange

// loadISBNRanges loads all ISBN ranges from the database into memory.
func loadISBNRanges(db *sql.DB) ([]isbnRange, error) {
	rows, err := db.Query("SELECT start_value, end_value, prefix_length, publisher_length FROM isbn_ranges")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ranges []isbnRange
	for rows.Next() {
		var r isbnRange
		if err := rows.Scan(&r.startValue, &r.endValue, &r.prefixLength, &r.publisherLength); err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	return ranges, rows.Err()
}

// sqlFindISBNformat looks up prefix and publisher lengths from the in-memory cache.
func sqlFindISBNformat(db *sql.DB, value int) (int, int, bool, error) {
	for _, r := range isbnRanges {
		if value >= r.startValue && value <= r.endValue {
			return r.prefixLength, r.publisherLength, true, nil
		}
	}
	return 0, 0, false, nil
}

// ISBNValidFormat returns true if the ISBN string has a recognisable format
// (10 or 13 digits after stripping hyphens and spaces, with an optional
// trailing 'X' allowed for ISBN-10).
func ISBNValidFormat(isbn string) bool {
	isbn = strings.ReplaceAll(strings.ToUpper(isbn), "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	n := len(isbn)
	if n != 10 && n != 13 {
		return false
	}
	for i, ch := range isbn {
		if ch >= '0' && ch <= '9' {
			continue
		}
		// 'X' is only valid as the final character of an ISBN-10
		if ch == 'X' && n == 10 && i == 9 {
			continue
		}
		return false
	}
	return true
}

// ToISBN13 converts a compact (no hyphens) ISBN-10 to an ISBN-13 string
// (without hyphens). Returns the original string if conversion is not possible.
func ToISBN13(isbn10 string) string {
	if len(isbn10) != 10 {
		return isbn10
	}
	core := "978" + isbn10[:9]
	sum := 0
	for i, ch := range core {
		d := int(ch - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	check := (10 - sum%10) % 10
	return fmt.Sprintf("%s%d", core, check)
}

// ToISBN10 converts a compact (no hyphens) ISBN-13 that starts with "978" to
// an ISBN-10 string (without hyphens). Returns the original string otherwise.
func ToISBN10(isbn13 string) string {
	if len(isbn13) != 13 || !strings.HasPrefix(isbn13, "978") {
		return isbn13
	}
	core := isbn13[3:12]
	sum := 0
	for i, ch := range core {
		d := int(ch - '0')
		sum += (i + 1) * d
	}
	remainder := sum % 11
	var check string
	if remainder == 10 {
		check = "X"
	} else {
		check = fmt.Sprintf("%d", remainder)
	}
	return core + check
}

// ValidISBN13 returns true if the given string is a valid ISBN-13.
func ValidISBN13(isbn string) bool {
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	if len(isbn) != 13 {
		return false
	}
	if !strings.HasPrefix(isbn, "978") && !strings.HasPrefix(isbn, "979") {
		return false
	}
	sum1, sum2 := 0, 0
	for i := 0; i < 12; i += 2 {
		d := int(isbn[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		sum1 += d
	}
	for i := 1; i < 12; i += 2 {
		d := int(isbn[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		sum2 += d
	}
	checksum := sum1 + sum2*3
	remainder := checksum % 10
	if remainder != 0 {
		remainder = 10 - remainder
	}
	return int(isbn[12]-'0') == remainder
}

// ValidISBN returns true if the given string is a valid ISBN-10 or ISBN-13.
func ValidISBN(isbn string) bool {
	isbn = strings.ReplaceAll(strings.ToUpper(isbn), "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	if len(isbn) == 13 {
		return ValidISBN13(isbn)
	}
	if len(isbn) != 10 {
		return false
	}
	localSum := 0
	for i := 0; i < 9; i++ {
		d := int(isbn[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		localSum += (i + 1) * d
	}
	remain := localSum % 11
	if remain == 10 {
		return isbn[9] == 'X'
	}
	return int(isbn[9]-'0') == remain
}

// ConvertISBN formats an ISBN string with standard hyphen separators.
// Requires a database connection to look up the ISBN range formatting rules.
func ConvertISBN(db *sql.DB, isbn string) (string, error) {
	if !ValidISBN(isbn) {
		return isbn, nil
	}

	stripped := strings.ReplaceAll(isbn, "-", "")
	stripped = strings.ReplaceAll(stripped, " ", "")
	checksum := string(stripped[len(stripped)-1])

	var coreISBN string
	if len(stripped) == 13 {
		coreISBN = stripped[:11]
	} else {
		coreISBN = "978" + stripped[:8]
	}

	// Special case for certain ISBN-10 prefixes (Tor, M.E. Sharpe, etc.)
	var prefixLength, publisherLength int
	if len(stripped) == 10 {
		prefix5 := stripped[:5]
		if prefix5 == "07653" || prefix5 == "07656" || prefix5 == "08123" || prefix5 == "08125" {
			prefixLength = 4
			publisherLength = 3
		}
	}

	if prefixLength == 0 {
		// Parse coreISBN as integer for range lookup
		coreInt := 0
		for _, ch := range coreISBN {
			coreInt = coreInt*10 + int(ch-'0')
		}
		pl, publ, found, err := sqlFindISBNformat(db, coreInt)
		if err != nil {
			return isbn, err
		}
		if !found {
			// Not in a recognized range — add one hyphen before checksum
			return fmt.Sprintf("%s-%s", stripped[:len(stripped)-1], checksum), nil
		}
		prefixLength = pl
		publisherLength = publ
	}

	// Format with hyphens
	if len(stripped) == 13 {
		return fmt.Sprintf("%s-%s-%s-%s-%s",
			stripped[:3],
			stripped[3:prefixLength],
			stripped[prefixLength:prefixLength+publisherLength],
			stripped[prefixLength+publisherLength:12],
			checksum), nil
	}

	// ISBN-10: adjust prefix_length (which was calculated for ISBN-13 with 978 prefix)
	prefixLength -= 3
	return fmt.Sprintf("%s-%s-%s-%s",
		stripped[:prefixLength],
		stripped[prefixLength:prefixLength+publisherLength],
		stripped[prefixLength+publisherLength:9],
		checksum), nil
}
