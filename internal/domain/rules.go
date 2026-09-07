package domain

import (
	"path/filepath"
	"strconv"
	"strings"
)

// TrashDir is the server folder deletes move into; it is hidden from listings.
const TrashDir = "/.trash"

// FreeName appends " 2", " 3", … before the extension until exists says the name is unused.
func FreeName(p string, exists func(string) bool) string {
	ext := filepath.Ext(p)
	stem := strings.TrimSuffix(p, ext)
	for i := 2; exists(p); i++ {
		p = stem + " " + strconv.Itoa(i) + ext
	}
	return p
}
