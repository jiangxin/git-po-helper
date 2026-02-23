package util

import (
	"fmt"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

// DiffStat holds the diff statistics between two PO files.
type DiffStat struct {
	Added   int // Entries in dest but not in src
	Changed int // Same msgid but different content
	Deleted int // Entries in src but not in dest
}

// EntriesEqual checks if two PO entries are equal (same msgid and msgstr).
func EntriesEqual(e1, e2 *PoEntry) bool {
	if e1.IsFuzzy != e2.IsFuzzy {
		return false
	}
	if e1.MsgID != e2.MsgID {
		return false
	}
	if e1.MsgStr != e2.MsgStr {
		return false
	}
	if e1.MsgIDPlural != e2.MsgIDPlural {
		return false
	}
	if len(e1.MsgStrPlural) != len(e2.MsgStrPlural) {
		return false
	}
	for i := range e1.MsgStrPlural {
		if e1.MsgStrPlural[i] != e2.MsgStrPlural[i] {
			return false
		}
	}
	return true
}

// PoCompare compares src and dest PO file content. Returns DiffStat, the generated
// review-input data (newHeader + reviewEntries), and error. reviewEntries are entries
// that are new or changed in dest compared to src.
func PoCompare(src, dest []byte) (DiffStat, []byte, error) {
	origEntries, _, err := ParsePoEntries(src)
	if err != nil {
		return DiffStat{}, nil, fmt.Errorf("failed to parse src file: %w", err)
	}
	newEntries, newHeader, err := ParsePoEntries(dest)
	if err != nil {
		return DiffStat{}, nil, fmt.Errorf("failed to parse dest file: %w", err)
	}

	// Sort entries by MsgID for consistent ordering
	sort.Slice(origEntries, func(i, j int) bool {
		return origEntries[i].MsgID < origEntries[j].MsgID
	})
	sort.Slice(newEntries, func(i, j int) bool {
		return newEntries[i].MsgID < newEntries[j].MsgID
	})

	if len(src) == 0 {
		log.Debugf("src file is empty, all entries in dest will be included in review-input")
	}

	// Two-pointer merge of sorted origEntries and newEntries
	var stat DiffStat
	var reviewEntries []*PoEntry
	i, j := 0, 0
	for i < len(origEntries) && j < len(newEntries) {
		cmp := strings.Compare(origEntries[i].MsgID, newEntries[j].MsgID)
		if cmp < 0 {
			stat.Deleted++
			i++
		} else if cmp > 0 {
			stat.Added++
			reviewEntries = append(reviewEntries, newEntries[j])
			j++
		} else {
			if !EntriesEqual(origEntries[i], newEntries[j]) {
				stat.Changed++
				reviewEntries = append(reviewEntries, newEntries[j])
			}
			i++
			j++
		}
	}
	for i < len(origEntries) {
		stat.Deleted++
		i++
	}
	for j < len(newEntries) {
		stat.Added++
		reviewEntries = append(reviewEntries, newEntries[j])
		j++
	}

	log.Debugf("review stats: deleted=%d, added=%d, changed=%d", stat.Deleted, stat.Added, stat.Changed)

	data := BuildPoContent(newHeader, reviewEntries)
	return stat, data, nil
}
