package util

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEntryRange(t *testing.T) {
	tests := []struct {
		spec     string
		maxEntry int
		want     []int
		wantErr  bool
	}{
		{"1", 10, []int{1}, false},
		{"0", 10, []int{0}, false},
		{"1-3", 10, []int{1, 2, 3}, false},
		{"3,5,9-13", 20, []int{3, 5, 9, 10, 11, 12, 13}, false},
		{"1-3,5", 10, []int{1, 2, 3, 5}, false},
		{"0,2,4", 5, []int{0, 2, 4}, false},
		{"15", 10, []int{}, false},        // Out of range, silently skipped
		{"1-5", 3, []int{1, 2, 3}, false}, // Range clipped
		{"", 10, nil, true},
		{"abc", 10, nil, true},
		{"1-2", 10, []int{1, 2}, false},
		{"2-1", 10, nil, true},                  // Invalid: start > end
		{"-5", 10, []int{1, 2, 3, 4, 5}, false}, // -N: from 1 to N
		{"-3", 10, []int{1, 2, 3}, false},
		{"50-", 100, buildRange(50, 100), false}, // N-: from N to last
		{"8-", 10, []int{8, 9, 10}, false},
		{"-", 10, nil, true}, // Invalid: both empty
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			got, err := ParseEntryRange(tt.spec, tt.maxEntry)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEntryRange(%q, %d) error = %v, wantErr %v", tt.spec, tt.maxEntry, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !sliceEqual(got, tt.want) {
				t.Errorf("ParseEntryRange(%q, %d) = %v, want %v", tt.spec, tt.maxEntry, got, tt.want)
			}
		})
	}
}

func buildRange(start, end int) []int {
	var r []int
	for i := start; i <= end; i++ {
		r = append(r, i)
	}
	return r
}

func sliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMsgSelect(t *testing.T) {
	poContent := `# SOME DESCRIPTIVE TITLE.
msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "First"
msgstr "第一个"

msgid "Second"
msgstr "第二个"

msgid "Third"
msgstr "第三个"
`

	tmpDir := t.TempDir()
	poFile := filepath.Join(tmpDir, "test.po")
	if err := os.WriteFile(poFile, []byte(poContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var buf bytes.Buffer
	err := MsgSelect(poFile, "1,3", &buf)
	if err != nil {
		t.Fatalf("MsgSelect failed: %v", err)
	}

	output := buf.String()
	// Should contain header (entry 0) and entries 1 and 3
	if !strings.Contains(output, "First") {
		t.Errorf("output should contain 'First', got:\n%s", output)
	}
	if !strings.Contains(output, "Third") {
		t.Errorf("output should contain 'Third', got:\n%s", output)
	}
	if !strings.Contains(output, "Content-Type") {
		t.Errorf("output should contain header, got:\n%s", output)
	}
	if strings.Contains(output, "Second") {
		t.Errorf("output should not contain 'Second' (entry 2), got:\n%s", output)
	}
}

func TestMsgSelect_OpenEndedRange(t *testing.T) {
	poContent := `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "First"
msgstr "一"

msgid "Second"
msgstr "二"

msgid "Third"
msgstr "三"
`

	tmpDir := t.TempDir()
	poFile := filepath.Join(tmpDir, "test.po")
	if err := os.WriteFile(poFile, []byte(poContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Run("-2 means entries 1-2", func(t *testing.T) {
		var buf bytes.Buffer
		err := MsgSelect(poFile, "-2", &buf)
		if err != nil {
			t.Fatalf("MsgSelect failed: %v", err)
		}
		output := buf.String()
		if !strings.Contains(output, "First") || !strings.Contains(output, "Second") {
			t.Errorf("output should contain First and Second, got:\n%s", output)
		}
		if strings.Contains(output, "Third") {
			t.Errorf("output should not contain Third, got:\n%s", output)
		}
	})

	t.Run("2- means entries 2 to last", func(t *testing.T) {
		var buf bytes.Buffer
		err := MsgSelect(poFile, "2-", &buf)
		if err != nil {
			t.Fatalf("MsgSelect failed: %v", err)
		}
		output := buf.String()
		if !strings.Contains(output, "Second") || !strings.Contains(output, "Third") {
			t.Errorf("output should contain Second and Third, got:\n%s", output)
		}
		if strings.Contains(output, "First") {
			t.Errorf("output should not contain First (entry 1), got:\n%s", output)
		}
	})
}
