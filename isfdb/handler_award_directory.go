// Copyright 2026 Al von Ruff. All rights reserved.
// Use of this source code is governed by the MIT No Attribution
// license that can be found in the LICENSE file.

package isfdb

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"unicode/utf8"
)


// AwardDirectoryHandler serves /award_directory.cgi — a full listing of all
// award types, ordered by short name.
func AwardDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	awards, err := SQLSearchAwardTypes(DB, "")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("text/html; charset=%s", UNICODE))
	HTMLheader(w, "Award Directory")
	PrintNavbar(w, "directory", "", "")

	// Sort to match MySQL utf8_general_ci behaviour: non-ASCII short names first
	// (Cyrillic, CJK, etc.), then ASCII names alphabetically within each group.
	sort.SliceStable(awards, func(i, j int) bool {
		ni := awards[i].AwardTypeShortName.String
		nj := awards[j].AwardTypeShortName.String
		ri, _ := utf8.DecodeRuneInString(ni)
		rj, _ := utf8.DecodeRuneInString(nj)
		iNonASCII := ri > 127
		jNonASCII := rj > 127
		if iNonASCII != jNonASCII {
			return iNonASCII // non-ASCII sorts first
		}
		return ni < nj
	})

	printSearchAwardTypeTable(w, awards)

	fmt.Fprintln(w, `</div>`) // main
	HTMLtrailer(w)
}
