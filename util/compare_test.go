package util

import (
	"bytes"
	"testing"
)

const poHeader = `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

`

// TestPoCompare_Added tests PoCompare when dest has new entries.
func TestPoCompare_Added(t *testing.T) {
	srcContent := poHeader + `msgid "Hello"
msgstr "你好"
`
	destContent := srcContent + `msgid "World"
msgstr "世界"
`

	stat, data, err := PoCompare([]byte(srcContent), []byte(destContent))
	if err != nil {
		t.Fatalf("PoCompare returned error: %v", err)
	}
	if stat.Added != 1 {
		t.Errorf("expected Added=1, got %d", stat.Added)
	}
	if stat.Changed != 0 {
		t.Errorf("expected Changed=0, got %d", stat.Changed)
	}
	if stat.Deleted != 0 {
		t.Errorf("expected Deleted=0, got %d", stat.Deleted)
	}
	if len(data) == 0 {
		t.Errorf("expected non-empty review data")
	}
	if !bytes.Contains(data, []byte("World")) {
		t.Errorf("review data should contain new entry 'World', got: %s", data)
	}
}

// TestPoCompare_NoChange tests PoCompare when files are identical.
func TestPoCompare_NoChange(t *testing.T) {
	content := poHeader + `msgid "Hello"
msgstr "你好"
`

	stat, data, err := PoCompare([]byte(content), []byte(content))
	if err != nil {
		t.Fatalf("PoCompare returned error: %v", err)
	}
	if stat.Added != 0 || stat.Changed != 0 || stat.Deleted != 0 {
		t.Errorf("expected all zeros, got Added=%d Changed=%d Deleted=%d",
			stat.Added, stat.Changed, stat.Deleted)
	}
	if len(data) != 0 {
		t.Errorf("expected empty review data when no change, got %d bytes", len(data))
	}
}

// TestPoCompare_Deleted tests PoCompare when dest has fewer entries.
func TestPoCompare_Deleted(t *testing.T) {
	srcContent := poHeader + `msgid "Hello"
msgstr "你好"

msgid "World"
msgstr "世界"
`
	destContent := poHeader + `msgid "Hello"
msgstr "你好"
`

	stat, data, err := PoCompare([]byte(srcContent), []byte(destContent))
	if err != nil {
		t.Fatalf("PoCompare returned error: %v", err)
	}
	if stat.Added != 0 {
		t.Errorf("expected Added=0, got %d", stat.Added)
	}
	if stat.Changed != 0 {
		t.Errorf("expected Changed=0, got %d", stat.Changed)
	}
	if stat.Deleted != 1 {
		t.Errorf("expected Deleted=1, got %d", stat.Deleted)
	}
	if len(data) != 0 {
		t.Errorf("expected empty review data (no new/changed), got %d bytes", len(data))
	}
}

// TestPoCompare_Changed tests PoCompare when same msgid has different content.
func TestPoCompare_Changed(t *testing.T) {
	srcContent := poHeader + `msgid "Hello"
msgstr "你好"
`
	destContent := poHeader + `msgid "Hello"
msgstr "您好"
`

	stat, data, err := PoCompare([]byte(srcContent), []byte(destContent))
	if err != nil {
		t.Fatalf("PoCompare returned error: %v", err)
	}
	if stat.Added != 0 {
		t.Errorf("expected Added=0, got %d", stat.Added)
	}
	if stat.Changed != 1 {
		t.Errorf("expected Changed=1, got %d", stat.Changed)
	}
	if stat.Deleted != 0 {
		t.Errorf("expected Deleted=0, got %d", stat.Deleted)
	}
	if !bytes.Contains(data, []byte("您好")) {
		t.Errorf("review data should contain changed entry, got: %s", data)
	}
}

// TestPoCompare_EmptySrc tests PoCompare when src is empty (all dest entries are new).
func TestPoCompare_EmptySrc(t *testing.T) {
	destContent := poHeader + `msgid "Hello"
msgstr "你好"
`

	stat, data, err := PoCompare([]byte{}, []byte(destContent))
	if err != nil {
		t.Fatalf("PoCompare returned error: %v", err)
	}
	if stat.Added != 1 {
		t.Errorf("expected Added=1, got %d", stat.Added)
	}
	if stat.Deleted != 0 {
		t.Errorf("expected Deleted=0, got %d", stat.Deleted)
	}
	if !bytes.Contains(data, []byte("Hello")) {
		t.Errorf("review data should contain entry, got: %s", data)
	}
}

// TestPoCompare_OutputRoundTrip verifies the returned data can be parsed back.
func TestPoCompare_OutputRoundTrip(t *testing.T) {
	srcContent := poHeader + `msgid "Hello"
msgstr "你好"
`
	destContent := srcContent + `msgid "World"
msgstr "世界"
`

	_, data, err := PoCompare([]byte(srcContent), []byte(destContent))
	if err != nil {
		t.Fatalf("PoCompare returned error: %v", err)
	}
	if len(data) == 0 {
		t.Skip("no data to round-trip")
	}

	entries, header, err := ParsePoEntries(data)
	if err != nil {
		t.Fatalf("ParsePoEntries of output failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry in review output, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].MsgID != "World" {
		t.Errorf("expected MsgID 'World', got %q", entries[0].MsgID)
	}
	if len(header) == 0 {
		t.Errorf("expected non-empty header")
	}

	// Round-trip: BuildPoContent should produce same bytes
	rebuilt := BuildPoContent(header, entries)
	if !bytes.Equal(data, rebuilt) {
		t.Errorf("round-trip mismatch: BuildPoContent output differs from PoCompare output")
	}
}

// TestEntriesEqual tests EntriesEqual.
func TestEntriesEqual(t *testing.T) {
	tests := []struct {
		name string
		e1   *PoEntry
		e2   *PoEntry
		want bool
	}{
		{
			name: "identical",
			e1:   &PoEntry{MsgID: "a", MsgStr: "x"},
			e2:   &PoEntry{MsgID: "a", MsgStr: "x"},
			want: true,
		},
		{
			name: "different msgstr",
			e1:   &PoEntry{MsgID: "a", MsgStr: "x"},
			e2:   &PoEntry{MsgID: "a", MsgStr: "y"},
			want: false,
		},
		{
			name: "different msgid",
			e1:   &PoEntry{MsgID: "a", MsgStr: "x"},
			e2:   &PoEntry{MsgID: "b", MsgStr: "x"},
			want: false,
		},
		{
			name: "different IsFuzzy",
			e1:   &PoEntry{MsgID: "a", MsgStr: "x", IsFuzzy: false},
			e2:   &PoEntry{MsgID: "a", MsgStr: "x", IsFuzzy: true},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EntriesEqual(tt.e1, tt.e2)
			if got != tt.want {
				t.Errorf("EntriesEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
