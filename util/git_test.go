package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-l10n/git-po-helper/repository"
)

// TestGetChangedPoFiles tests GetChangedPoFiles with a temporary git repository.
// It creates a repo with po files, makes commits, and verifies the changed files list.
func TestGetChangedPoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repository
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create po directory and initial files
	poDir := filepath.Join(tmpDir, "po")
	if err := os.MkdirAll(poDir, 0755); err != nil {
		t.Fatalf("failed to create po dir: %v", err)
	}

	poContent := `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Hello"
msgstr "你好"
`

	for _, f := range []string{"zh_CN.po", "zh_TW.po"} {
		if err := os.WriteFile(filepath.Join(poDir, f), []byte(poContent), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", f, err)
		}
	}

	runGit("add", "po/")
	runGit("commit", "-m", "initial")

	// Modify only zh_CN.po
	modifiedContent := poContent + "\nmsgid \"World\"\nmsgstr \"世界\"\n"
	if err := os.WriteFile(filepath.Join(poDir, "zh_CN.po"), []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to modify zh_CN.po: %v", err)
	}

	// Open repository for testing (must be done before GetChangedPoFiles)
	repository.OpenRepository(tmpDir)

	t.Run("default mode (HEAD vs working tree)", func(t *testing.T) {
		files, err := GetChangedPoFiles("", "")
		if err != nil {
			t.Fatalf("GetChangedPoFiles failed: %v", err)
		}
		if len(files) != 1 {
			t.Errorf("expected 1 changed file, got %d: %v", len(files), files)
		}
		if len(files) > 0 && files[0] != "po/zh_CN.po" {
			t.Errorf("expected po/zh_CN.po, got %s", files[0])
		}
	})

	t.Run("excludes .pot files", func(t *testing.T) {
		// Add git.pot and modify it
		potContent := `# Copyright (C) 2024
msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "test"
msgstr ""
`
		if err := os.WriteFile(filepath.Join(poDir, "git.pot"), []byte(potContent), 0644); err != nil {
			t.Fatalf("failed to write git.pot: %v", err)
		}
		runGit("add", "po/git.pot")
		runGit("commit", "-m", "add pot")

		// Modify pot file
		if err := os.WriteFile(filepath.Join(poDir, "git.pot"), []byte(potContent+"\nmsgid \"extra\"\nmsgstr \"\"\n"), 0644); err != nil {
			t.Fatalf("failed to modify git.pot: %v", err)
		}

		files, err := GetChangedPoFiles("", "")
		if err != nil {
			t.Fatalf("GetChangedPoFiles failed: %v", err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, ".pot") {
				t.Errorf("GetChangedPoFiles should not return .pot files, got %s", f)
			}
		}
	})
}
