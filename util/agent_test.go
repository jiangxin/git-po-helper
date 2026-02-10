package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountPotEntries(t *testing.T) {
	tests := []struct {
		name        string
		potContent  string
		expected    int
		expectError bool
	}{
		{
			name: "normal POT file with multiple entries",
			potContent: `# SOME DESCRIPTIVE TITLE.
# Copyright (C) YEAR THE PACKAGE'S COPYRIGHT HOLDER
# This file is distributed under the same license as the PACKAGE package.
# FIRST AUTHOR <EMAIL@ADDRESS>, YEAR.
#
msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "First string"
msgstr ""

msgid "Second string"
msgstr ""

msgid "Third string"
msgstr ""
`,
			expected:    3,
			expectError: false,
		},
		{
			name: "POT file with only header",
			potContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"
`,
			expected:    0,
			expectError: false,
		},
		{
			name: "POT file with multi-line msgid",
			potContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "First line"
"Second line"
msgstr ""

msgid "Another string"
msgstr ""
`,
			expected:    2,
			expectError: false,
		},
		{
			name:        "empty file",
			potContent:  "",
			expected:    0,
			expectError: false,
		},
		{
			name: "POT file with commented entries",
			potContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

#~ msgid "Obsolete entry"
#~ msgstr ""

msgid "Active entry"
msgstr ""
`,
			expected:    1,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			potFile := filepath.Join(tmpDir, "test.pot")
			err := os.WriteFile(potFile, []byte(tt.potContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test POT file: %v", err)
			}

			// Test CountPotEntries
			count, err := CountPotEntries(potFile)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if count != tt.expected {
				t.Errorf("Expected count %d, got %d", tt.expected, count)
			}
		})
	}
}

func TestCountPotEntries_InvalidFile(t *testing.T) {
	// Test with non-existent file
	_, err := CountPotEntries("/nonexistent/file.pot")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestCountPotEntries_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	potFile := filepath.Join(tmpDir, "empty.pot")

	// Create empty file
	file, err := os.Create(potFile)
	if err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}
	file.Close()

	count, err := CountPotEntries(potFile)
	if err != nil {
		t.Errorf("Unexpected error for empty file: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 for empty file, got %d", count)
	}
}
