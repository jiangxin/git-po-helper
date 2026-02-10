package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

func TestReplacePlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		template string
		prompt   string
		source   string
		commit   string
		expected string
	}{
		{
			name:     "all placeholders",
			template: "cmd -p {prompt} -s {source} -c {commit}",
			prompt:   "update pot",
			source:   "po/zh_CN.po",
			commit:   "HEAD",
			expected: "cmd -p update pot -s po/zh_CN.po -c HEAD",
		},
		{
			name:     "only prompt placeholder",
			template: "cmd -p {prompt}",
			prompt:   "update pot",
			source:   "",
			commit:   "",
			expected: "cmd -p update pot",
		},
		{
			name:     "multiple occurrences",
			template: "{prompt} {prompt} {prompt}",
			prompt:   "test",
			source:   "",
			commit:   "",
			expected: "test test test",
		},
		{
			name:     "empty values",
			template: "cmd -p {prompt} -s {source} -c {commit}",
			prompt:   "",
			source:   "",
			commit:   "",
			expected: "cmd -p  -s  -c ",
		},
		{
			name:     "no placeholders",
			template: "cmd -p test",
			prompt:   "update pot",
			source:   "po/zh_CN.po",
			commit:   "HEAD",
			expected: "cmd -p test",
		},
		{
			name:     "special characters in values",
			template: "cmd -p {prompt}",
			prompt:   "update 'pot' file",
			source:   "",
			commit:   "",
			expected: "cmd -p update 'pot' file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReplacePlaceholders(tt.template, tt.prompt, tt.source, tt.commit)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExecuteAgentCommand(t *testing.T) {
	// Test successful command execution
	t.Run("successful command", func(t *testing.T) {
		var cmd []string
		if runtime.GOOS == "windows" {
			cmd = []string{"cmd", "/c", "echo", "test output"}
		} else {
			cmd = []string{"sh", "-c", "echo 'test output'"}
		}

		stdout, stderr, err := ExecuteAgentCommand(cmd, "")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		output := strings.TrimSpace(string(stdout))
		if !strings.Contains(output, "test output") {
			t.Errorf("Expected stdout to contain 'test output', got %q", output)
		}

		if len(stderr) > 0 {
			t.Logf("stderr: %s", string(stderr))
		}
	})

	// Test command with stderr output
	t.Run("command with stderr", func(t *testing.T) {
		var cmd []string
		if runtime.GOOS == "windows" {
			cmd = []string{"cmd", "/c", "echo test >&2"}
		} else {
			cmd = []string{"sh", "-c", "echo 'test error' >&2"}
		}

		stdout, stderr, err := ExecuteAgentCommand(cmd, "")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// stderr should contain the error message
		if len(stderr) == 0 {
			t.Log("Note: stderr is empty (this may be expected on some systems)")
		}
		_ = stdout
	})

	// Test command failure (non-zero exit code)
	t.Run("command failure", func(t *testing.T) {
		var cmd []string
		if runtime.GOOS == "windows" {
			cmd = []string{"cmd", "/c", "exit 1"}
		} else {
			cmd = []string{"sh", "-c", "exit 1"}
		}

		stdout, stderr, err := ExecuteAgentCommand(cmd, "")
		if err == nil {
			t.Error("Expected error for failing command, got nil")
		}

		// Error should mention exit code
		if !strings.Contains(err.Error(), "exit code") && !strings.Contains(err.Error(), "failed") {
			t.Errorf("Error message should mention exit code or failure: %v", err)
		}

		_ = stdout
		_ = stderr
	})

	// Test empty command
	t.Run("empty command", func(t *testing.T) {
		_, _, err := ExecuteAgentCommand([]string{}, "")
		if err == nil {
			t.Error("Expected error for empty command, got nil")
		}
		if !strings.Contains(err.Error(), "cannot be empty") {
			t.Errorf("Error should mention 'cannot be empty': %v", err)
		}
	})

	// Test non-existent command
	t.Run("non-existent command", func(t *testing.T) {
		_, _, err := ExecuteAgentCommand([]string{"nonexistent-command-xyz123"}, "")
		if err == nil {
			t.Error("Expected error for non-existent command, got nil")
		}
	})

	// Test command with custom working directory
	t.Run("custom working directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		var cmd []string
		if runtime.GOOS == "windows" {
			cmd = []string{"cmd", "/c", "cd"}
		} else {
			cmd = []string{"pwd"}
		}

		stdout, stderr, err := ExecuteAgentCommand(cmd, tmpDir)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		output := strings.TrimSpace(string(stdout))
		// On Windows, the path format might be different, so just check it's not empty
		if len(output) == 0 {
			t.Error("Expected non-empty output from pwd/cd command")
		}

		_ = stderr
	})
}

func TestExecuteAgentCommand_PlaceholderReplacement(t *testing.T) {
	// This test verifies that placeholder replacement should be done
	// before calling ExecuteAgentCommand, not inside it.
	// ExecuteAgentCommand should execute the command as-is.

	t.Run("command with literal placeholders", func(t *testing.T) {
		// ExecuteAgentCommand should not replace placeholders
		// (that's the caller's responsibility)
		var cmd []string
		if runtime.GOOS == "windows" {
			cmd = []string{"cmd", "/c", "echo", "{prompt}"}
		} else {
			cmd = []string{"sh", "-c", "echo '{prompt}'"}
		}

		stdout, _, err := ExecuteAgentCommand(cmd, "")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		output := strings.TrimSpace(string(stdout))
		if !strings.Contains(output, "{prompt}") {
			t.Errorf("Expected literal {prompt} in output, got %q", output)
		}
	})
}

// Test helper: verify that ExecuteAgentCommand works with real commands
func TestExecuteAgentCommand_RealCommand(t *testing.T) {
	// Test with a command that should exist on all systems
	var cmd []string
	if runtime.GOOS == "windows" {
		// On Windows, use cmd /c echo
		cmd = []string{"cmd", "/c", "echo", "Hello World"}
	} else {
		// On Unix, use echo
		cmd = []string{"echo", "Hello World"}
	}

	// Check if command exists
	if _, err := exec.LookPath(cmd[0]); err != nil {
		t.Skipf("Command %s not found, skipping test", cmd[0])
	}

	stdout, stderr, err := ExecuteAgentCommand(cmd, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	output := strings.TrimSpace(string(stdout))
	if !strings.Contains(output, "Hello World") {
		t.Errorf("Expected 'Hello World' in output, got %q", output)
	}

	if len(stderr) > 0 {
		t.Logf("stderr (non-fatal): %s", string(stderr))
	}
}

func TestCountPoEntries(t *testing.T) {
	tests := []struct {
		name        string
		poContent   string
		expected    int
		expectError bool
	}{
		{
			name: "normal PO file with multiple entries",
			poContent: `# SOME DESCRIPTIVE TITLE.
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
			name: "PO file with only header",
			poContent: `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"
`,
			expected:    0,
			expectError: false,
		},
		{
			name: "PO file with multi-line msgid",
			poContent: `msgid ""
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
			poContent:   "",
			expected:    0,
			expectError: false,
		},
		{
			name: "PO file with commented entries",
			poContent: `msgid ""
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
			poFile := filepath.Join(tmpDir, "test.po")
			err := os.WriteFile(poFile, []byte(tt.poContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test PO file: %v", err)
			}

			// Test CountPoEntries
			count, err := CountPoEntries(poFile)
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

func TestCountPoEntries_InvalidFile(t *testing.T) {
	// Test with non-existent file
	_, err := CountPoEntries("/nonexistent/file.po")
	if err == nil {
		t.Error("Expected error for non-existent PO file, got nil")
	}
}

func TestCountPoEntries_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	poFile := filepath.Join(tmpDir, "empty.po")

	// Create empty file
	file, err := os.Create(poFile)
	if err != nil {
		t.Fatalf("Failed to create empty PO file: %v", err)
	}
	file.Close()

	count, err := CountPoEntries(poFile)
	if err != nil {
		t.Errorf("Unexpected error for empty PO file: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 for empty PO file, got %d", count)
	}
}

func TestValidatePoEntryCount_Disabled(t *testing.T) {
	// nil expectedCount
	var expectedNil *int
	if err := ValidatePoEntryCount("/nonexistent/file.po", expectedNil, "before update"); err != nil {
		t.Errorf("Expected no error when validation is disabled with nil expectedCount, got %v", err)
	}

	// zero expectedCount
	zero := 0
	if err := ValidatePoEntryCount("/nonexistent/file.po", &zero, "after update"); err != nil {
		t.Errorf("Expected no error when validation is disabled with zero expectedCount, got %v", err)
	}
}

func TestValidatePoEntryCount_BeforeUpdateMissingFile(t *testing.T) {
	expected := 1
	if err := ValidatePoEntryCount("/nonexistent/file.po", &expected, "before update"); err == nil {
		t.Errorf("Expected error when file is missing and expectedCount is non-zero in before update stage, got nil")
	}
}

func TestValidatePoEntryCount_AfterUpdateMissingFile(t *testing.T) {
	expected := 1
	if err := ValidatePoEntryCount("/nonexistent/file.po", &expected, "after update"); err == nil {
		t.Errorf("Expected error when file is missing in after update stage, got nil")
	}
}

func TestValidatePoEntryCount_MatchingAndNonMatching(t *testing.T) {
	// Prepare a temporary PO file with a single entry
	const poContent = `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "First string"
msgstr ""
`

	tmpDir := t.TempDir()
	poFile := filepath.Join(tmpDir, "test.po")
	if err := os.WriteFile(poFile, []byte(poContent), 0644); err != nil {
		t.Fatalf("Failed to create test PO file: %v", err)
	}

	// Matching expected count
	matching := 1
	if err := ValidatePoEntryCount(poFile, &matching, "before update"); err != nil {
		t.Errorf("Expected no error for matching entry count, got %v", err)
	}

	// Non-matching expected count
	nonMatching := 2
	if err := ValidatePoEntryCount(poFile, &nonMatching, "after update"); err == nil {
		t.Errorf("Expected error for non-matching entry count, got nil")
	}
}
