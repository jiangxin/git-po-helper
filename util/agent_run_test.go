package util

import (
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
			entries, header, err := ParsePoEntries([]byte(tt.poContent))
			if err != nil {
				t.Fatalf("ParsePoEntries failed: %v", err)
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

		})
	}
}
