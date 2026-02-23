package util

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-l10n/git-po-helper/repository"
)

func requireMsgcmp(t *testing.T) {
	if _, err := exec.LookPath("msgcmp"); err != nil {
		t.Skip("msgcmp not installed, skipping diff tests")
	}
}

// TestPoFileDiffStat tests PoFileDiffStat with two PO files.
func TestPoFileDiffStat(t *testing.T) {
	requireMsgcmp(t)

	tmpDir := t.TempDir()

	// src: one entry
	srcContent := `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Hello"
msgstr "你好"
`
	srcPath := filepath.Join(tmpDir, "src.po")
	if err := os.WriteFile(srcPath, []byte(srcContent), 0644); err != nil {
		t.Fatalf("failed to write src.po: %v", err)
	}

	// dest: two entries (one new)
	destContent := srcContent + "\nmsgid \"World\"\nmsgstr \"世界\"\n"
	destPath := filepath.Join(tmpDir, "dest.po")
	if err := os.WriteFile(destPath, []byte(destContent), 0644); err != nil {
		t.Fatalf("failed to write dest.po: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	err := PoFileDiffStat(srcPath, destPath)
	if err != nil {
		t.Errorf("PoFileDiffStat returned error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output == "" {
		t.Errorf("output should not be empty")
	}
	if !strings.Contains(output, "1 new") {
		t.Errorf("output should contain '1 new' (one new entry in dest), got: %s", output)
	}
}

// TestPoFileDiffStat_NoChange tests PoFileDiffStat when files are identical.
func TestPoFileDiffStat_NoChange(t *testing.T) {
	requireMsgcmp(t)

	tmpDir := t.TempDir()
	content := `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Hello"
msgstr "你好"
`
	path := filepath.Join(tmpDir, "test.po")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test.po: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	err := PoFileDiffStat(path, path)
	if err != nil {
		t.Errorf("PoFileDiffStat returned error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// When nothing changed, "Nothing changed." is printed to stderr; stdout is empty
	if output != "" {
		t.Errorf("output should be empty when files are identical, got: %s", output)
	}
}

// TestPoFileRevisionDiffStat tests PoFileRevisionDiffStat with a git repository.
func TestPoFileRevisionDiffStat(t *testing.T) {
	requireMsgcmp(t)

	tmpDir := t.TempDir()

	// checkoutTmpfile runs git show from current dir; chdir to tmpDir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

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

	poDir := filepath.Join(tmpDir, "po")
	if err := os.MkdirAll(poDir, 0755); err != nil {
		t.Fatalf("failed to create po dir: %v", err)
	}

	poV1 := `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Hello"
msgstr "你好"
`
	poV2 := poV1 + "\nmsgid \"World\"\nmsgstr \"世界\"\n"

	if err := os.WriteFile(filepath.Join(poDir, "zh_CN.po"), []byte(poV1), 0644); err != nil {
		t.Fatalf("failed to write zh_CN.po: %v", err)
	}
	runGit("add", "po/")
	runGit("commit", "-m", "v1")

	if err := os.WriteFile(filepath.Join(poDir, "zh_CN.po"), []byte(poV2), 0644); err != nil {
		t.Fatalf("failed to write zh_CN.po: %v", err)
	}
	runGit("add", "po/zh_CN.po")
	runGit("commit", "-m", "v2")

	repository.OpenRepository(tmpDir)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	src := FileRevision{Revision: "HEAD~1", File: "po/zh_CN.po"}
	dest := FileRevision{Revision: "HEAD", File: "po/zh_CN.po"}

	err = PoFileRevisionDiffStat(src, dest)
	if err != nil {
		t.Errorf("PoFileRevisionDiffStat returned error: %v", err)
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output == "" {
		t.Errorf("output should not be empty")
	}
	if !strings.Contains(output, "1 new") {
		t.Errorf("output should contain '1 new', got: %s", output)
	}
}
