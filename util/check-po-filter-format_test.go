package util

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/git-l10n/git-po-helper/repository"
	"github.com/spf13/viper"
)

// preparePoFilterTestRepo creates a repo with po/test.po and gettext-no-location in .gitattributes,
// commits, opens it as the current repository, and chdirs into the repo root. t.Cleanup restores cwd
// and re-opens the original repository.
func preparePoFilterTestRepo(t *testing.T, poBody string) (repoPo string) {
	t.Helper()
	tmpDir := t.TempDir()
	gitEnv := gitTestEnv()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = gitEnv
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
		}
	}
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWd)
		repository.OpenRepository(origWd)
	})
	poDir := filepath.Join(tmpDir, "po")
	if err := os.MkdirAll(poDir, 0755); err != nil {
		t.Fatal(err)
	}
	repoPo = filepath.Join(poDir, "test.po")
	if err := os.WriteFile(repoPo, []byte(poBody), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitattributes"), []byte("po/*.po filter=gettext-no-location\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("init")
	runGit("config", "user.email", "t@t.com")
	runGit("config", "user.name", "T")
	runGit("add", "po/test.po", ".gitattributes")
	runGit("commit", "--no-verify", "-m", "init")
	repository.OpenRepository(tmpDir)
	return repoPo
}

func TestCheckPoFilterFormat_repoAttrPathWithTempContent(t *testing.T) {
	tmpDir := t.TempDir()
	gitEnv := gitTestEnv()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = gitEnv
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
		}
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origWd)
		repository.OpenRepository(origWd)
	}()

	runGit("init")
	runGit("config", "user.email", "t@t.com")
	runGit("config", "user.name", "T")

	poDir := filepath.Join(tmpDir, "po")
	if err := os.MkdirAll(poDir, 0755); err != nil {
		t.Fatal(err)
	}
	poBody := `msgid ""
msgstr ""
"Project-Id-Version: Git\n"
"Content-Type: text/plain; charset=UTF-8\n"

#: main.c:42
msgid "Hello"
msgstr "Hi"
`
	repoPo := filepath.Join(poDir, "test.po")
	if err := os.WriteFile(repoPo, []byte(poBody), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "po/test.po")
	runGit("commit", "--no-verify", "-m", "init")

	repository.OpenRepository(tmpDir)
	viper.Set("check--report-file-locations", "error")
	defer viper.Set("check--report-file-locations", "")

	// gettext-no-location disallows any #: line in the PO (injected filter; no .gitattributes).
	fr := &FileRevision{File: repoPo, Revision: ""}
	defer fr.Cleanup()
	errs, ok := checkPoFilterFormat(fr, "gettext-no-location")
	if ok || len(errs) == 0 {
		t.Fatalf("expected failure for #: under gettext-no-location, ok=%v errs=%v", ok, errs)
	}
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, "gettext-no-location") || !strings.Contains(joined, "location comment not allowed") {
		t.Fatalf("unexpected messages: %s", joined)
	}
}

func TestCheckPoFilterFormat_outsideRepoWithoutAttrPathFails(t *testing.T) {
	tmpDir := t.TempDir()
	outside := filepath.Join(tmpDir, "orphan.po")
	if err := os.WriteFile(outside, []byte(`msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"
`), 0644); err != nil {
		t.Fatal(err)
	}

	origWd, _ := os.Getwd()
	repoRoot := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origWd)
		repository.OpenRepository(origWd)
	}()

	gitEnv := gitTestEnv()
	init := exec.Command("git", "init")
	init.Dir = repoRoot
	init.Env = gitEnv
	if err := init.Run(); err != nil {
		t.Fatal(err)
	}
	repository.OpenRepository(repoRoot)
	viper.Set("check--report-file-locations", "error")
	defer viper.Set("check--report-file-locations", "")

	fr := &FileRevision{File: outside, Revision: ""}
	defer fr.Cleanup()
	errs, ok := checkPoFilterFormat(fr, "")
	if ok || len(errs) == 0 {
		t.Fatalf("expected failure when path outside repo without repo-relative File, ok=%v errs=%v", ok, errs)
	}
	if !strings.Contains(strings.Join(errs, "\n"), "not under repository root") {
		t.Fatalf("expected outside-repo message, got: %v", errs)
	}
}

func TestCheckPoFilterFormat_invalidRepoAttrPath(t *testing.T) {
	tmpDir := t.TempDir()

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origWd)
		repository.OpenRepository(origWd)
	}()

	gitEnv := gitTestEnv()
	init := exec.Command("git", "init")
	init.Dir = tmpDir
	init.Env = gitEnv
	if err := init.Run(); err != nil {
		t.Fatal(err)
	}
	repository.OpenRepository(tmpDir)
	viper.Set("check--report-file-locations", "error")
	defer viper.Set("check--report-file-locations", "")

	escapePo := filepath.Join(filepath.Dir(tmpDir), "git-po-helper-filter-escape-"+t.Name()+".po")
	if err := os.WriteFile(escapePo, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(escapePo) })

	fr := &FileRevision{File: filepath.Join("..", filepath.Base(escapePo)), Revision: ""}
	defer fr.Cleanup()
	_, ok := checkPoFilterFormat(fr, "")
	if ok {
		t.Fatal("expected failure for attr path escaping repo")
	}
}

// TestCheckPoFilterFormat_attrSourceCommit checks git check-attr --source=<rev> so attribute
// resolution follows the given revision (needed for bare partial clones; see checkPoFilterFormat).
func TestCheckPoFilterFormat_attrSourceCommit(t *testing.T) {
	tmpDir := t.TempDir()
	gitEnv := gitTestEnv()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = gitEnv
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
		}
	}
	revParse := func() string {
		t.Helper()
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = tmpDir
		cmd.Env = gitEnv
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origWd)
		repository.OpenRepository(origWd)
	}()

	runGit("init")
	runGit("config", "user.email", "t@t.com")
	runGit("config", "user.name", "T")

	poDir := filepath.Join(tmpDir, "po")
	if err := os.MkdirAll(poDir, 0755); err != nil {
		t.Fatal(err)
	}
	poBody := `msgid ""
msgstr ""
"Project-Id-Version: Git\n"
"Content-Type: text/plain; charset=UTF-8\n"

#: main.c:42
msgid "Hello"
msgstr "Hi"
`
	repoPo := filepath.Join(poDir, "test.po")
	if err := os.WriteFile(repoPo, []byte(poBody), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitattributes"), []byte("po/*.po filter=gettext-no-location\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "po/test.po", ".gitattributes")
	runGit("commit", "--no-verify", "-m", "with filter")
	commitWithFilter := revParse()

	if err := os.Remove(filepath.Join(tmpDir, ".gitattributes")); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "--no-verify", "-m", "drop gitattributes")
	commitNoAttr := revParse()

	repository.OpenRepository(tmpDir)
	viper.Set("check--report-file-locations", "error")
	defer viper.Set("check--report-file-locations", "")

	frOld := &FileRevision{File: "po/test.po", Revision: commitWithFilter}
	defer frOld.Cleanup()
	errs, ok := checkPoFilterFormat(frOld, "")
	if ok || len(errs) == 0 {
		t.Fatalf("expected failure (#: under gettext-no-location at old rev), ok=%v errs=%v", ok, errs)
	}
	if !strings.Contains(strings.Join(errs, "\n"), "location comment not allowed") {
		t.Fatalf("expected no-location message: %v", errs)
	}

	frNew := &FileRevision{File: "po/test.po", Revision: commitNoAttr}
	defer frNew.Cleanup()
	errs, ok = checkPoFilterFormat(frNew, "")
	if ok || len(errs) == 0 {
		t.Fatalf("expected failure (no filter policy) at new rev, ok=%v errs=%v", ok, errs)
	}
	if !strings.Contains(strings.Join(errs, "\n"), "No Git `filter` attribute") {
		t.Fatalf("expected missing-filter message for rev without .gitattributes: %v", errs)
	}
}

func TestCheckPoFilterContent_normalizedMatchesNoWarn(t *testing.T) {
	poBody := `msgid ""
msgstr ""
"Project-Id-Version: Git\n"
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Hello"
msgstr "Hi"
`
	repoPo := preparePoFilterTestRepo(t, poBody)

	fr := &FileRevision{File: repoPo, Revision: ""}
	defer fr.Cleanup()
	if warns := checkPoFilterContent(fr, ""); len(warns) != 0 {
		t.Fatalf("expected no filter clean warnings, got: %q", strings.Join(warns, "\n"))
	}
}

func TestCheckPoFilterContent_mismatchProducesWarn(t *testing.T) {
	poBody := `msgid ""
msgstr ""
"Project-Id-Version: Git\n"
"Content-Type: text/plain; charset=UTF-8\n"

#: main.c:42
msgid "Hello"
msgstr "Hi"
`
	repoPo := preparePoFilterTestRepo(t, poBody)

	fr := &FileRevision{File: repoPo, Revision: ""}
	defer fr.Cleanup()
	warns := checkPoFilterContent(fr, "")
	joined := strings.Join(warns, "\n")
	if len(warns) == 0 || !strings.Contains(joined, "does not match") {
		t.Fatalf("expected mismatch warning, got len=%d: %s", len(warns), joined)
	}
}

func TestCheckPoFilterContent_skipsInGitHubActionsEnv(t *testing.T) {
	oldGA := os.Getenv("GITHUB_ACTIONS")
	defer func() {
		if oldGA == "" {
			_ = os.Unsetenv("GITHUB_ACTIONS")
		} else {
			_ = os.Setenv("GITHUB_ACTIONS", oldGA)
		}
	}()
	if err := os.Setenv("GITHUB_ACTIONS", "true"); err != nil {
		t.Fatal(err)
	}

	poBody := `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

#: x.c:1
msgid "Hello"
msgstr "Hi"
`
	repoPo := preparePoFilterTestRepo(t, poBody)

	fr := &FileRevision{File: repoPo, Revision: ""}
	defer fr.Cleanup()
	if w := checkPoFilterContent(fr, ""); len(w) != 0 {
		t.Fatalf("expected skip in GITHUB_ACTIONS, got: %q", strings.Join(w, "\n"))
	}
}

func TestCheckPoFilterContent_skipsWithGithubActionEventViper(t *testing.T) {
	viper.Set("github-action-event", "push")
	defer viper.Set("github-action-event", "")

	poBody := `msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

#: x.c:1
msgid "Hello"
msgstr "Hi"
`
	repoPo := preparePoFilterTestRepo(t, poBody)

	fr := &FileRevision{File: repoPo, Revision: ""}
	defer fr.Cleanup()
	if w := checkPoFilterContent(fr, ""); len(w) != 0 {
		t.Fatalf("expected skip with github-action-event, got: %q", strings.Join(w, "\n"))
	}
}
