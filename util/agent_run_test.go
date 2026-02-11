package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePoEntries(t *testing.T) {
	tests := []struct {
		name           string
		poContent      string
		expectedHeader []string
		expectedCount  int
		validateEntry  func(t *testing.T, entries []*PoEntry)
	}{
		{
			name: "simple PO file with header and entries",
			poContent: `# SOME DESCRIPTIVE TITLE.
# Copyright (C) YEAR THE PACKAGE'S COPYRIGHT HOLDER
# This file is distributed under the same license as the PACKAGE package.
# FIRST AUTHOR <EMAIL@ADDRESS>, YEAR.
#
msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"
"Content-Transfer-Encoding: 8bit\n"

msgid "Hello"
msgstr "你好"

msgid ""
"World"
msgstr ""
"世界"
`,
			expectedHeader: []string{
				`# SOME DESCRIPTIVE TITLE.`,
				`# Copyright (C) YEAR THE PACKAGE'S COPYRIGHT HOLDER`,
				`# This file is distributed under the same license as the PACKAGE package.`,
				`# FIRST AUTHOR <EMAIL@ADDRESS>, YEAR.`,
				`#`,
				`msgid ""`,
				`msgstr ""`,
				`"Content-Type: text/plain; charset=UTF-8\n"`,
				`"Content-Transfer-Encoding: 8bit\n"`,
			},
			expectedCount: 2,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(entries))
				}
				if entries[0].MsgID != "Hello" {
					t.Errorf("expected first entry MsgID 'Hello', got '%s'", entries[0].MsgID)
				}
				if entries[0].MsgStr != "你好" {
					t.Errorf("expected first entry MsgStr '你好', got '%s'", entries[0].MsgStr)
				}
				if entries[1].MsgID != "World" {
					t.Errorf("expected second entry MsgID 'World', got '%s'", entries[1].MsgID)
				}
				if entries[1].MsgStr != "世界" {
					t.Errorf("expected second entry MsgStr '世界', got '%s'", entries[1].MsgStr)
				}
			},
		},
		{
			name: "PO file with multi-line msgid and msgstr",
			poContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid ""
"First line"
"Second line"
msgstr ""
"第一行"
"第二行"

msgid "Single line"
msgstr "单行"
`,
			expectedHeader: []string{
				`msgid ""`,
				`msgstr ""`,
				`"Content-Type: text/plain; charset=UTF-8\n"`,
			},
			expectedCount: 2,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(entries))
				}
				expectedMsgID := "First lineSecond line"
				if entries[0].MsgID != expectedMsgID {
					t.Errorf("expected first entry MsgID '%s', got '%s'", expectedMsgID, entries[0].MsgID)
				}
				expectedMsgStr := "第一行第二行"
				if entries[0].MsgStr != expectedMsgStr {
					t.Errorf("expected first entry MsgStr '%s', got '%s'", expectedMsgStr, entries[0].MsgStr)
				}
				if entries[1].MsgID != "Single line" {
					t.Errorf("expected second entry MsgID 'Single line', got '%s'", entries[1].MsgID)
				}
			},
		},
		{
			name: "PO file with plural forms",
			poContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "One item"
msgid_plural "Many items"
msgstr[0] "一个项目"
msgstr[1] "多个项目"

msgid "File"
msgid_plural "Files"
msgstr[0] "文件"
msgstr[1] "文件"
`,
			expectedHeader: []string{
				`msgid ""`,
				`msgstr ""`,
				`"Content-Type: text/plain; charset=UTF-8\n"`,
			},
			expectedCount: 2,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(entries))
				}
				if entries[0].MsgID != "One item" {
					t.Errorf("expected first entry MsgID 'One item', got '%s'", entries[0].MsgID)
				}
				if entries[0].MsgIDPlural != "Many items" {
					t.Errorf("expected first entry MsgIDPlural 'Many items', got '%s'", entries[0].MsgIDPlural)
				}
				if len(entries[0].MsgStrPlural) != 2 {
					t.Fatalf("expected 2 plural forms, got %d", len(entries[0].MsgStrPlural))
				}
				if entries[0].MsgStrPlural[0] != "一个项目" {
					t.Errorf("expected first plural form '一个项目', got '%s'", entries[0].MsgStrPlural[0])
				}
				if entries[0].MsgStrPlural[1] != "多个项目" {
					t.Errorf("expected second plural form '多个项目', got '%s'", entries[0].MsgStrPlural[1])
				}
			},
		},
		{
			name: "PO file with plural forms with multiple lines",
			poContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid ""
"One "
"item"
msgid_plural ""
"Many "
"items"
msgstr[0] ""
"一个"
"项目"
msgstr[1] ""
"多个"
"项目"

msgid ""
"File"
msgid_plural ""
"Files"
msgstr[0] "文件"
msgstr[1] ""
"文件"
`,
			expectedHeader: []string{
				`msgid ""`,
				`msgstr ""`,
				`"Content-Type: text/plain; charset=UTF-8\n"`,
			},
			expectedCount: 2,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(entries))
				}
				if entries[0].MsgID != "One item" {
					t.Errorf("expected first entry MsgID 'One item', got '%s'", entries[0].MsgID)
				}
				if entries[0].MsgIDPlural != "Many items" {
					t.Errorf("expected first entry MsgIDPlural 'Many items', got '%s'", entries[0].MsgIDPlural)
				}
				if len(entries[0].MsgStrPlural) != 2 {
					t.Fatalf("expected 2 plural forms, got %d", len(entries[0].MsgStrPlural))
				}
				if entries[0].MsgStrPlural[0] != "一个项目" {
					t.Errorf("expected first plural form '一个项目', got '%s'", entries[0].MsgStrPlural[0])
				}
				if entries[0].MsgStrPlural[1] != "多个项目" {
					t.Errorf("expected second plural form '多个项目', got '%s'", entries[0].MsgStrPlural[1])
				}
			},
		},
		{
			name: "PO file with comments",
			poContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

# Translator comment
#. Automatic comment
#: file.c:123
msgid "String with comments"
msgstr "带注释的字符串"

msgid "Simple string"
msgstr "简单字符串"
`,
			expectedHeader: []string{
				`msgid ""`,
				`msgstr ""`,
				`"Content-Type: text/plain; charset=UTF-8\n"`,
			},
			expectedCount: 2,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(entries))
				}
				// Verify the entry exists and has correct msgid
				if entries[0].MsgID != "String with comments" {
					t.Errorf("expected first entry MsgID 'String with comments', got '%s'", entries[0].MsgID)
				}
				if entries[0].MsgStr != "带注释的字符串" {
					t.Errorf("expected first entry MsgStr '带注释的字符串', got '%s'", entries[0].MsgStr)
				}
				// Verify comments are preserved
				expectedComments := []string{
					"# Translator comment",
					"#. Automatic comment",
					"#: file.c:123",
				}
				if len(entries[0].Comments) != len(expectedComments) {
					t.Errorf("expected %d comments, got %d", len(expectedComments), len(entries[0].Comments))
				} else {
					for i, expectedComment := range expectedComments {
						if entries[0].Comments[i] != expectedComment {
							t.Errorf("comment %d mismatch: expected '%s', got '%s'", i, expectedComment, entries[0].Comments[i])
						}
					}
				}
				// Verify second entry has no comments
				if len(entries[1].Comments) != 0 {
					t.Errorf("expected second entry to have no comments, got %d comments", len(entries[1].Comments))
				}
			},
		},
		{
			name: "PO file with only header",
			poContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"
"Language: zh_CN\n"
`,
			expectedHeader: []string{
				`msgid ""`,
				`msgstr ""`,
				`"Content-Type: text/plain; charset=UTF-8\n"`,
				`"Language: zh_CN\n"`,
			},
			expectedCount: 0,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 0 {
					t.Errorf("expected 0 entries, got %d", len(entries))
				}
			},
		},
		{
			name: "PO file with empty msgstr (untranslated)",
			poContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Untranslated"
msgstr ""

msgid "Translated"
msgstr "已翻译"
`,
			expectedHeader: []string{
				`msgid ""`,
				`msgstr ""`,
				`"Content-Type: text/plain; charset=UTF-8\n"`,
			},
			expectedCount: 2,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(entries))
				}
				if entries[0].MsgID != "Untranslated" {
					t.Errorf("expected first entry MsgID 'Untranslated', got '%s'", entries[0].MsgID)
				}
				if entries[0].MsgStr != "" {
					t.Errorf("expected first entry MsgStr to be empty, got '%s'", entries[0].MsgStr)
				}
				if entries[1].MsgStr != "已翻译" {
					t.Errorf("expected second entry MsgStr '已翻译', got '%s'", entries[1].MsgStr)
				}
			},
		},
		{
			name:           "empty file",
			poContent:      "",
			expectedHeader: []string{}, // Empty file may return empty header or single empty line
			expectedCount:  0,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 0 {
					t.Errorf("expected 0 entries, got %d", len(entries))
				}
			},
		},
		{
			name: "PO file with fuzzy entry",
			poContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

#, fuzzy
msgid "Fuzzy string"
msgstr "模糊字符串"

msgid "Normal string"
msgstr "正常字符串"
`,
			expectedHeader: []string{
				`msgid ""`,
				`msgstr ""`,
				`"Content-Type: text/plain; charset=UTF-8\n"`,
			},
			expectedCount: 2,
			validateEntry: func(t *testing.T, entries []*PoEntry) {
				if len(entries) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(entries))
				}
				// Verify entries exist and have correct msgid
				if entries[0].MsgID != "Fuzzy string" {
					t.Errorf("expected first entry MsgID 'Fuzzy string', got '%s'", entries[0].MsgID)
				}
				if entries[1].MsgID != "Normal string" {
					t.Errorf("expected second entry MsgID 'Normal string', got '%s'", entries[1].MsgID)
				}
				// Verify fuzzy comment is preserved
				if len(entries[0].Comments) != 1 {
					t.Errorf("expected 1 comment for fuzzy entry, got %d", len(entries[0].Comments))
				} else if entries[0].Comments[0] != "#, fuzzy" {
					t.Errorf("expected comment '#, fuzzy', got '%s'", entries[0].Comments[0])
				}
				// Verify second entry has no comments
				if len(entries[1].Comments) != 0 {
					t.Errorf("expected second entry to have no comments, got %d comments", len(entries[1].Comments))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the PO content
			entries, header, err := parsePoEntries([]byte(tt.poContent))
			if err != nil {
				t.Fatalf("parsePoEntries failed: %v", err)
			}

			// Validate header (allow for minor differences, especially for empty file)
			if tt.name == "empty file" {
				// Empty file may return empty header or single empty line - both are acceptable
				if len(header) > 1 {
					t.Errorf("empty file header should be empty or single line, got %d lines", len(header))
				}
			} else {
				if len(header) != len(tt.expectedHeader) {
					t.Errorf("header length mismatch: expected %d, got %d", len(tt.expectedHeader), len(header))
					t.Logf("Expected header: %v", tt.expectedHeader)
					t.Logf("Got header: %v", header)
				} else {
					for i, expectedLine := range tt.expectedHeader {
						if i < len(header) && header[i] != expectedLine {
							t.Errorf("header line %d mismatch: expected '%s', got '%s'", i, expectedLine, header[i])
						}
					}
				}
			}

			// Validate entry count
			if len(entries) != tt.expectedCount {
				t.Errorf("entry count mismatch: expected %d, got %d", tt.expectedCount, len(entries))
			}

			// Run custom validation if provided
			if tt.validateEntry != nil {
				tt.validateEntry(t, entries)
			}

			// Test writeReviewInputPo and compare with original
			if tt.poContent != "" && len(entries) > 0 {
				tmpDir := t.TempDir()
				outputPath := filepath.Join(tmpDir, "test-output.po")

				err = writeReviewInputPo(outputPath, header, entries)
				if err != nil {
					t.Fatalf("writeReviewInputPo failed: %v", err)
				}

				// Read the written file
				writtenData, err := os.ReadFile(outputPath)
				if err != nil {
					t.Fatalf("failed to read written file: %v", err)
				}

				// Parse the written file again
				writtenEntries, writtenHeader, err := parsePoEntries(writtenData)
				if err != nil {
					t.Fatalf("failed to parse written file: %v", err)
				}

				// Compare headers (allow for minor differences in formatting)
				if len(writtenHeader) != len(header) {
					t.Errorf("written header length mismatch: expected %d, got %d", len(header), len(writtenHeader))
				}

				// Compare entry count
				if len(writtenEntries) != len(entries) {
					t.Errorf("written entry count mismatch: expected %d, got %d", len(entries), len(writtenEntries))
				}

				// Compare entries
				for i, originalEntry := range entries {
					if i >= len(writtenEntries) {
						t.Errorf("missing entry %d in written file", i)
						continue
					}
					writtenEntry := writtenEntries[i]

					if originalEntry.MsgID != writtenEntry.MsgID {
						t.Errorf("entry %d MsgID mismatch: expected '%s', got '%s'", i, originalEntry.MsgID, writtenEntry.MsgID)
					}
					if originalEntry.MsgStr != writtenEntry.MsgStr {
						t.Errorf("entry %d MsgStr mismatch: expected '%s', got '%s'", i, originalEntry.MsgStr, writtenEntry.MsgStr)
					}
					if originalEntry.MsgIDPlural != writtenEntry.MsgIDPlural {
						t.Errorf("entry %d MsgIDPlural mismatch: expected '%s', got '%s'", i, originalEntry.MsgIDPlural, writtenEntry.MsgIDPlural)
					}
					if len(originalEntry.MsgStrPlural) != len(writtenEntry.MsgStrPlural) {
						t.Errorf("entry %d MsgStrPlural length mismatch: expected %d, got %d", i, len(originalEntry.MsgStrPlural), len(writtenEntry.MsgStrPlural))
					} else {
						for j, expectedPlural := range originalEntry.MsgStrPlural {
							if writtenEntry.MsgStrPlural[j] != expectedPlural {
								t.Errorf("entry %d MsgStrPlural[%d] mismatch: expected '%s', got '%s'", i, j, expectedPlural, writtenEntry.MsgStrPlural[j])
							}
						}
					}
					// Verify comments are preserved
					if len(originalEntry.Comments) != len(writtenEntry.Comments) {
						t.Errorf("entry %d comments count mismatch: expected %d, got %d", i, len(originalEntry.Comments), len(writtenEntry.Comments))
					} else {
						for j, expectedComment := range originalEntry.Comments {
							if writtenEntry.Comments[j] != expectedComment {
								t.Errorf("entry %d comment %d mismatch: expected '%s', got '%s'", i, j, expectedComment, writtenEntry.Comments[j])
							}
						}
					}
				}
			}
		})
	}
}

// TestParsePoEntriesRoundTrip tests reading a PO file, writing it, and reading again
// to verify that the round-trip preserves entry count, structure, and header.
func TestParsePoEntriesRoundTrip(t *testing.T) {
	// Get the PO file path from environment variable
	poFilePath := os.Getenv("TEST_PO_FILE")
	if poFilePath == "" {
		t.Skip("TEST_PO_FILE environment variable not set, skipping test")
	}

	// Read the PO file content
	testPoContent, err := os.ReadFile(poFilePath)
	if err != nil {
		t.Fatalf("failed to read PO file %s: %v", poFilePath, err)
	}

	tmpDir := t.TempDir()
	originalPoPath := filepath.Join(tmpDir, "original.po")
	writtenPoPath := filepath.Join(tmpDir, "written.po")

	// Write the original PO file
	err = os.WriteFile(originalPoPath, testPoContent, 0644)
	if err != nil {
		t.Fatalf("failed to write original PO file: %v", err)
	}

	// Read the original PO file
	originalData, err := os.ReadFile(originalPoPath)
	if err != nil {
		t.Fatalf("failed to read original PO file: %v", err)
	}

	originalEntries, originalHeader, err := parsePoEntries(originalData)
	if err != nil {
		t.Fatalf("failed to parse original PO file: %v", err)
	}

	// Write the PO file using writeReviewInputPo
	err = writeReviewInputPo(writtenPoPath, originalHeader, originalEntries)
	if err != nil {
		t.Fatalf("failed to write PO file: %v", err)
	}

	// Read the written PO file
	writtenData, err := os.ReadFile(writtenPoPath)
	if err != nil {
		t.Fatalf("failed to read written PO file: %v", err)
	}

	writtenEntries, writtenHeader, err := parsePoEntries(writtenData)
	if err != nil {
		t.Fatalf("failed to parse written PO file: %v", err)
	}

	// Compare header
	if len(originalHeader) != len(writtenHeader) {
		t.Errorf("header length mismatch: original has %d lines, written has %d lines", len(originalHeader), len(writtenHeader))
		t.Logf("Original header: %v", originalHeader)
		t.Logf("Written header: %v", writtenHeader)
	} else {
		for i, originalLine := range originalHeader {
			if i < len(writtenHeader) && originalLine != writtenHeader[i] {
				t.Errorf("header line %d mismatch: original '%s', written '%s'", i, originalLine, writtenHeader[i])
			}
		}
	}

	// Compare entry count
	if len(originalEntries) != len(writtenEntries) {
		t.Errorf("entry count mismatch: original has %d entries, written has %d entries", len(originalEntries), len(writtenEntries))
	} else {
		t.Logf("Entry count matches: %d entries", len(originalEntries))
	}

	// Compare each entry structure
	for i, originalEntry := range originalEntries {
		if i >= len(writtenEntries) {
			t.Errorf("missing entry %d in written file", i)
			continue
		}
		writtenEntry := writtenEntries[i]

		// Compare MsgID
		if originalEntry.MsgID != writtenEntry.MsgID {
			t.Errorf("entry %d MsgID mismatch: original '%s', written '%s'", i, originalEntry.MsgID, writtenEntry.MsgID)
		}

		// Compare MsgStr
		if originalEntry.MsgStr != writtenEntry.MsgStr {
			t.Errorf("entry %d MsgStr mismatch: original '%s', written '%s'", i, originalEntry.MsgStr, writtenEntry.MsgStr)
		}

		// Compare MsgIDPlural
		if originalEntry.MsgIDPlural != writtenEntry.MsgIDPlural {
			t.Errorf("entry %d MsgIDPlural mismatch: original '%s', written '%s'", i, originalEntry.MsgIDPlural, writtenEntry.MsgIDPlural)
		}

		// Compare MsgStrPlural
		if len(originalEntry.MsgStrPlural) != len(writtenEntry.MsgStrPlural) {
			t.Errorf("entry %d MsgStrPlural length mismatch: original has %d, written has %d", i, len(originalEntry.MsgStrPlural), len(writtenEntry.MsgStrPlural))
		} else {
			for j, originalPlural := range originalEntry.MsgStrPlural {
				if writtenEntry.MsgStrPlural[j] != originalPlural {
					t.Errorf("entry %d MsgStrPlural[%d] mismatch: original '%s', written '%s'", i, j, originalPlural, writtenEntry.MsgStrPlural[j])
				}
			}
		}

		// Compare Comments
		if len(originalEntry.Comments) != len(writtenEntry.Comments) {
			t.Errorf("entry %d comments count mismatch: original has %d, written has %d", i, len(originalEntry.Comments), len(writtenEntry.Comments))
			t.Logf("Original entry %d comments: %v", i, originalEntry.Comments)
			t.Logf("Written entry %d comments: %v", i, writtenEntry.Comments)
		} else {
			for j, originalComment := range originalEntry.Comments {
				if writtenEntry.Comments[j] != originalComment {
					t.Errorf("entry %d comment %d mismatch: original '%s', written '%s'", i, j, originalComment, writtenEntry.Comments[j])
				}
			}
		}
	}

	// Now test double round-trip: write again and read to verify consistency
	secondWrittenPoPath := filepath.Join(tmpDir, "written2.po")
	err = writeReviewInputPo(secondWrittenPoPath, writtenHeader, writtenEntries)
	if err != nil {
		t.Fatalf("failed to write PO file second time: %v", err)
	}

	secondReadData, err := os.ReadFile(secondWrittenPoPath)
	if err != nil {
		t.Fatalf("failed to read second written PO file: %v", err)
	}

	secondReadEntries, secondReadHeader, err := parsePoEntries(secondReadData)
	if err != nil {
		t.Fatalf("failed to parse second written PO file: %v", err)
	}

	// Compare second round-trip with original file
	if len(originalHeader) != len(secondReadHeader) {
		t.Errorf("second round-trip header length mismatch: original has %d lines, second written has %d lines", len(originalHeader), len(secondReadHeader))
		t.Logf("Original header: %v", originalHeader)
		t.Logf("Second written header: %v", secondReadHeader)
	} else {
		for i, originalLine := range originalHeader {
			if i < len(secondReadHeader) && originalLine != secondReadHeader[i] {
				t.Errorf("second round-trip header line %d mismatch: original '%s', second written '%s'", i, originalLine, secondReadHeader[i])
			}
		}
	}

	if len(originalEntries) != len(secondReadEntries) {
		t.Errorf("second round-trip entry count mismatch: original has %d entries, second written has %d entries", len(originalEntries), len(secondReadEntries))
	} else {
		t.Logf("Entry count matches for second round-trip: %d entries", len(secondReadEntries))
	}

	// Compare each entry structure in second round-trip
	for i, originalEntry := range originalEntries {
		if i >= len(secondReadEntries) {
			t.Errorf("missing entry %d in second written file", i)
			continue
		}
		secondReadEntry := secondReadEntries[i]

		// Compare MsgID
		if originalEntry.MsgID != secondReadEntry.MsgID {
			t.Errorf("second round-trip entry %d MsgID mismatch: original '%s', second written '%s'", i, originalEntry.MsgID, secondReadEntry.MsgID)
		}

		// Compare MsgStr
		if originalEntry.MsgStr != secondReadEntry.MsgStr {
			t.Errorf("second round-trip entry %d MsgStr mismatch: original '%s', second written '%s'", i, originalEntry.MsgStr, secondReadEntry.MsgStr)
		}

		// Compare MsgIDPlural
		if originalEntry.MsgIDPlural != secondReadEntry.MsgIDPlural {
			t.Errorf("second round-trip entry %d MsgIDPlural mismatch: original '%s', second written '%s'", i, originalEntry.MsgIDPlural, secondReadEntry.MsgIDPlural)
		}

		// Compare MsgStrPlural
		if len(originalEntry.MsgStrPlural) != len(secondReadEntry.MsgStrPlural) {
			t.Errorf("second round-trip entry %d MsgStrPlural length mismatch: original has %d, second written has %d", i, len(originalEntry.MsgStrPlural), len(secondReadEntry.MsgStrPlural))
		} else {
			for j, originalPlural := range originalEntry.MsgStrPlural {
				if secondReadEntry.MsgStrPlural[j] != originalPlural {
					t.Errorf("second round-trip entry %d MsgStrPlural[%d] mismatch: original '%s', second written '%s'", i, j, originalPlural, secondReadEntry.MsgStrPlural[j])
				}
			}
		}

		// Compare Comments
		if len(originalEntry.Comments) != len(secondReadEntry.Comments) {
			t.Errorf("second round-trip entry %d comments count mismatch: original has %d, second written has %d", i, len(originalEntry.Comments), len(secondReadEntry.Comments))
			t.Logf("Original entry %d comments: %v", i, originalEntry.Comments)
			t.Logf("Second written entry %d comments: %v", i, secondReadEntry.Comments)
		} else {
			for j, originalComment := range originalEntry.Comments {
				if secondReadEntry.Comments[j] != originalComment {
					t.Errorf("second round-trip entry %d comment %d mismatch: original '%s', second written '%s'", i, j, originalComment, secondReadEntry.Comments[j])
				}
			}
		}
	}
}
