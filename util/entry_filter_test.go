package util

import (
	"testing"
)

func TestFilterPoEntries_Default(t *testing.T) {
	entries := []*PoEntry{
		{MsgID: "a", MsgStr: "A", IsObsolete: false},
		{MsgID: "b", MsgStr: "", IsObsolete: false},
		{MsgID: "c", MsgStr: "c", IsFuzzy: false, IsObsolete: false},
		{MsgID: "d", MsgStr: "", IsObsolete: true},
	}
	f := DefaultFilter()
	indices := FilterPoEntries(entries, f)
	if len(indices) != 4 {
		t.Errorf("default filter: expected 4 entries, got %d", len(indices))
	}
}

func TestFilterPoEntries_NoObsolete(t *testing.T) {
	entries := []*PoEntry{
		{MsgID: "a", MsgStr: "A", IsObsolete: false},
		{MsgID: "d", MsgStr: "", IsObsolete: true},
	}
	f := EntryStateFilter{NoObsolete: true}
	indices := FilterPoEntries(entries, f)
	if len(indices) != 1 {
		t.Errorf("no-obsolete: expected 1 entry, got %d", len(indices))
	}
	if indices[0] != 1 {
		t.Errorf("expected index 1, got %d", indices[0])
	}
}

func TestFilterPoEntries_OnlyTranslated(t *testing.T) {
	entries := []*PoEntry{
		{MsgID: "a", MsgStr: "A", IsObsolete: false},
		{MsgID: "b", MsgStr: "", IsObsolete: false},
		{MsgID: "c", MsgStr: "c", IsFuzzy: false, IsObsolete: false},
	}
	f := EntryStateFilter{Translated: true, WithObsolete: false}
	indices := FilterPoEntries(entries, f)
	// Translated: a (A), c (same). b is untranslated.
	if len(indices) != 2 {
		t.Errorf("translated only: expected 2 entries, got %d", len(indices))
	}
}

func TestFilterPoEntries_OnlyObsolete(t *testing.T) {
	entries := []*PoEntry{
		{MsgID: "a", MsgStr: "A", IsObsolete: false},
		{MsgID: "d", MsgStr: "x", IsObsolete: true},
	}
	f := EntryStateFilter{OnlyObsolete: true}
	indices := FilterPoEntries(entries, f)
	if len(indices) != 1 {
		t.Errorf("only-obsolete: expected 1 entry, got %d", len(indices))
	}
	if indices[0] != 2 {
		t.Errorf("expected index 2 (obsolete), got %d", indices[0])
	}
}

func TestFilterPoEntries_OnlySame(t *testing.T) {
	entries := []*PoEntry{
		{MsgID: "a", MsgStr: "A", IsObsolete: false},
		{MsgID: "b", MsgStr: "b", IsObsolete: false},
	}
	f := EntryStateFilter{OnlySame: true}
	indices := FilterPoEntries(entries, f)
	if len(indices) != 1 {
		t.Errorf("only-same: expected 1 entry, got %d", len(indices))
	}
	if indices[0] != 2 {
		t.Errorf("expected index 2 (same), got %d", indices[0])
	}
}
