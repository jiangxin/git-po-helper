package util

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/git-l10n/git-po-helper/flag"
	"github.com/git-l10n/git-po-helper/repository"
)

// getGitFilterAttribute resolves the Git `filter` attribute for relPath (repository-relative, for
// git check-attr). If injectedFilter is non-empty after trimming, it is used instead of running git.
// attrSourceCommit is passed as git check-attr --source when non-empty. When git reports unspecified,
// unset, or empty, guidance messages are appended and the effective filter is gettext-no-location.
// For any other unsupported driver, errs receives a warning and the effective value falls back to
// gettext-no-location.
// declaredFilter is always the attribute value from git or injection before unsupported fallback
// (e.g. still "rot13" when effective becomes gettext-no-location); use it for filter.<name>.clean
// lookup. On git check-attr failure or unparseable output, effectiveFilter and declaredFilter are
// empty and errs has a single line; treat that as fatal and skip further filter checks.
func getGitFilterAttribute(relPath, attrSourceCommit, injectedFilter string) (effectiveFilter string, declaredFilter string, errs []string) {
	attrSourceCommit = strings.TrimSpace(attrSourceCommit)
	filterValue := strings.TrimSpace(injectedFilter)
	if filterValue == "" {
		var checkAttrArgs []string
		if repository.IsBare() {
			checkAttrArgs = []string{"check-attr"}
		} else {
			workDir := repository.WorkDir()
			checkAttrArgs = []string{"-C", workDir, "check-attr"}
		}
		if attrSourceCommit != "" {
			checkAttrArgs = append(checkAttrArgs, "--source="+attrSourceCommit)
		}
		checkAttrArgs = append(checkAttrArgs, "filter", relPath)
		cmd := exec.Command("git", checkAttrArgs...)
		cmd.Stderr = nil
		out, err := cmd.Output()
		if err != nil {
			return "", "", []string{fmt.Sprintf("git check-attr failed for %s: %s", relPath, err)}
		}
		line := strings.TrimSpace(string(out))
		parts := strings.SplitN(line, ": ", 3)
		if len(parts) < 3 {
			return "", "", []string{fmt.Sprintf("unexpected git check-attr output: %s", line)}
		}
		filterValue = strings.TrimSpace(parts[2])
	}
	declaredFilter = filterValue

	missingFilter := filterValue == "unspecified" || filterValue == "unset" || filterValue == ""
	if missingFilter {
		errs = append(errs,
			"No Git `filter` attribute is set for *.po files on this path.",
			"",
			"The filter attribute describes how Git should normalize #: location comments on each",
			"PO entry when you commit. Those comments change often as source files move; committing",
			"their churn produces noisy diffs and inflates the repository.",
			"",
			"Setting filter=gettext-no-location or filter=gettext-no-line-number in .gitattributes",
			"tells git-po-helper which location style you intend, so it can flag bad #: lines in",
			"the PO (for example references that still include line numbers).",
			"",
			"Please configure the filter for XX.po, for example:",
			"",
			"    .gitattributes: *.po filter=gettext-no-location",
			"",
			"See:",
			"",
			"    https://lore.kernel.org/git/20220504124121.12683-1-worldhello.net@gmail.com/",
		)
		effectiveFilter = "gettext-no-location"
		return effectiveFilter, declaredFilter, errs
	}

	effectiveFilter = filterValue
	if filterValue != "gettext-no-location" && filterValue != "gettext-no-line-number" {
		if len(errs) > 0 {
			errs = append(errs, "")
		}
		errs = append(errs, fmt.Sprintf(
			"Unsupported filter attribute %q for %s; "+
				`using "gettext-no-location" rules as fallback (PO must not contain #: location comments). `+
				`Prefer filter=gettext-no-location or filter=gettext-no-line-number in .gitattributes.`,
			filterValue, relPath))
		effectiveFilter = "gettext-no-location"
	}
	return effectiveFilter, declaredFilter, errs
}

// poFilterAttrRelPath maps materialized contentPath to the repository-relative path used for
// git check-attr and user-facing displayPath (repo-relative fr.File when set and not absolute).
func poFilterAttrRelPath(fr *FileRevision, contentPath string) (displayPath, relPath string, err error) {
	if fr.File != "" && !filepath.IsAbs(fr.File) {
		displayPath = fr.File
		relPath, err = GetRepoRelPath(fr.File)
		if err != nil {
			if errors.Is(err, ErrOutsideWorktree) {
				return displayPath, "", fmt.Errorf("filter attr path escapes repository: %s", fr.File)
			}
			return displayPath, "", err
		}
		return displayPath, relPath, nil
	}
	displayPath = contentPath
	relPath, err = GetRepoRelPath(contentPath)
	if err != nil {
		if errors.Is(err, ErrOutsideWorktree) {
			return displayPath, "", fmt.Errorf("file %s is not under repository root", contentPath)
		}
		return displayPath, "", err
	}
	return displayPath, relPath, nil
}

// checkPoFilterFormat checks git attributes for the filter driver and matches PO #: comments
// to that policy: gettext-no-line-number allows #: lines but refs must be file-only (no line
// numbers; see checkPoLocationCommentsNoLineNumbers); gettext-no-location (and unsupported
// drivers, which fall back to gettext-no-location) require no #: lines (checkPoLocationCommentsAbsent).
// If no filter is set, this function appends the missing-attribute guidance but does not return
// early so the caller's "Location comments (#:)" section can still run.
// It does not run msgcat or compare normalized bytes, and does not read filter.<driver>.clean.
// fr identifies the PO and attribute context: content is read from fr.GetFile(); when fr.File is
// non-empty and not an absolute path, it is treated as repo-relative for git check-attr (and for
// display in messages). When fr.File is empty or absolute, relPath for check-attr is derived from
// the materialized content path under the worktree (fails when that path is outside the repo).
// fr.Revision (when non-empty) is passed as git check-attr --source so attributes resolve at that
// commit (important for bare partial clones).
//
// filterAttribute, when non-empty after trimming, is used as the Git filter driver name
// instead of running "git check-attr filter <path>" (for example in tests without
// .gitattributes). When empty, the filter is read from git check-attr as usual.
// Callers that honor --no-check-filter must not invoke this function when that flag is set
// (see check-po.go).
func checkPoFilterFormat(fr *FileRevision, filterAttribute string) ([]string, bool) {
	var errs []string

	if fr == nil {
		return []string{"internal error: nil FileRevision for filter check"}, false
	}

	contentPath, err := fr.GetFile()
	if err != nil {
		return []string{fmt.Sprintf("cannot materialize PO for filter check: %s", err)}, false
	}

	if !Exist(contentPath) {
		errs = append(errs, fmt.Sprintf("cannot open %s: file does not exist", contentPath))
		return errs, false
	}

	if !repository.Opened() {
		return []string{"Not in a git repository. Skipping filter attribute check for file locations."}, true
	}

	displayPath, relPath, pathErr := poFilterAttrRelPath(fr, contentPath)
	if pathErr != nil {
		errs = append(errs, pathErr.Error())
		return errs, false
	}

	effectiveFilter, _, filterErrs := getGitFilterAttribute(relPath, fr.Revision, filterAttribute)
	if effectiveFilter == "" {
		return append(errs, filterErrs...), false
	}
	errs = append(errs, filterErrs...)

	poData, err := os.ReadFile(contentPath)
	if err != nil {
		if len(errs) > 0 {
			errs = append(errs, "")
		}
		errs = append(errs, fmt.Sprintf("cannot read %s: %s", displayPath, err))
		return errs, false
	}
	po, err := ParsePoEntries(poData)
	if err != nil {
		if len(errs) > 0 {
			errs = append(errs, "")
		}
		errs = append(errs, fmt.Sprintf("cannot parse %s: %s", displayPath, err))
		return errs, false
	}
	if effectiveFilter == "gettext-no-line-number" {
		locErrs, locOk := checkPoLocationCommentsNoLineNumbers(po)
		if !locOk {
			if len(errs) > 0 {
				errs = append(errs, "")
			}
			errs = append(errs, locErrs...)
		}
	} else {
		locErrs, locOk := checkPoLocationCommentsAbsent(po)
		if !locOk {
			if len(errs) > 0 {
				errs = append(errs, "")
			}
			errs = append(errs, locErrs...)
		}
	}

	hasErrs := len(errs) > 0
	filterOk := !hasErrs
	if hasErrs && flag.ReportFileLocations() == flag.ReportIssueWarn {
		filterOk = true
	}
	return errs, filterOk
}

// checkPoFilterContent runs filter.<driver>.clean from git config when set; otherwise for
// gettext-no-location and gettext-no-line-number it uses the same msgcat defaults as Git's
// recommended smudge/clean setup. It compares filter stdout to the on-disk PO; any mismatch
// yields warning lines only (never fails check-po). Skipped when flag.GitHubActionEvent() is
// non-empty (GitHub Actions / --github-action-event), when no filter attribute applies, or when
// an unknown driver has no filter.<name>.clean configured.
func checkPoFilterContent(fr *FileRevision, filterAttribute string) []string {
	if strings.TrimSpace(flag.GitHubActionEvent()) != "" {
		return nil
	}
	if !repository.Opened() {
		return nil
	}
	if fr == nil {
		return []string{"internal error: nil FileRevision for filter clean comparison"}
	}

	contentPath, err := fr.GetFile()
	if err != nil {
		return []string{fmt.Sprintf("cannot materialize PO for filter clean comparison: %s", err)}
	}
	if !Exist(contentPath) {
		return []string{fmt.Sprintf("cannot open %s: file does not exist", contentPath)}
	}

	displayPath, relPath, pathErr := poFilterAttrRelPath(fr, contentPath)
	if pathErr != nil {
		return append([]string{"PO filter clean comparison could not run:"}, pathErr.Error())
	}

	effective, declared, policyErrs := getGitFilterAttribute(relPath, fr.Revision, filterAttribute)
	if effective == "" {
		return append([]string{"PO filter clean comparison could not run:"}, policyErrs...)
	}

	missingFilter := declared == "unspecified" || declared == "unset" || declared == ""
	if missingFilter {
		return nil
	}

	filterValue := declared
	var cmdArgs []string
	cleanKey := "filter." + filterValue + ".clean"
	var cleanCmd *exec.Cmd
	if repository.IsBare() {
		cleanCmd = exec.Command("git", "config", "--get", cleanKey)
	} else {
		cleanCmd = exec.Command("git", "-C", repository.WorkDir(), "config", "--get", cleanKey)
	}
	cleanOut, err := cleanCmd.Output()
	if err == nil && len(bytes.TrimSpace(cleanOut)) > 0 {
		cmdArgs = strings.Fields(string(bytes.TrimSpace(cleanOut)))
	}
	if len(cmdArgs) == 0 {
		switch filterValue {
		case "gettext-no-location":
			cmdArgs = []string{"msgcat", "--no-location", "-"}
		case "gettext-no-line-number":
			cmdArgs = []string{"msgcat", "--add-location=file", "-"}
		default:
			return nil
		}
	}

	exe, err := exec.LookPath(cmdArgs[0])
	if err != nil {
		return []string{fmt.Sprintf("cannot run filter clean for %s: %q not in PATH", displayPath, cmdArgs[0])}
	}

	original, err := os.ReadFile(contentPath)
	if err != nil {
		return []string{fmt.Sprintf("cannot read %s for filter clean comparison: %s", displayPath, err)}
	}

	filterExe := exec.Command(exe, cmdArgs[1:]...)
	filterExe.Stdin = bytes.NewReader(original)
	filterExe.Stderr = nil
	formatted, err := filterExe.Output()
	if err != nil {
		return []string{fmt.Sprintf("filter %q clean command failed for %s: %v", filterValue, displayPath, err)}
	}

	if bytes.Equal(original, formatted) {
		return nil
	}

	origTmp, err := os.CreateTemp("", "git-po-helper-orig-*.po")
	if err != nil {
		return []string{
			fmt.Sprintf("cannot create temp file for diff: %s", err),
			fmt.Sprintf("PO on disk differs from filter clean output for %s (warning only).", displayPath),
		}
	}
	_, _ = origTmp.Write(original)
	_ = origTmp.Close()
	defer os.Remove(origTmp.Name())

	formTmp, err := os.CreateTemp("", "git-po-helper-fmt-*.po")
	if err != nil {
		return []string{
			fmt.Sprintf("cannot create temp file for diff: %s", err),
			fmt.Sprintf("PO on disk differs from filter clean output for %s (warning only).", displayPath),
		}
	}
	_, _ = formTmp.Write(formatted)
	_ = formTmp.Close()
	defer os.Remove(formTmp.Name())

	diffCmd := exec.Command("diff", "-u", origTmp.Name(), formTmp.Name())
	diffOut, _ := diffCmd.Output()

	filterCmdLine := strings.Join(cmdArgs, " ")
	warns := []string{
		fmt.Sprintf("PO on disk does not match filter %q clean output for %s (warning only, not a failing check).", filterValue, displayPath),
		fmt.Sprintf("Filter clean command: %s", filterCmdLine),
		"Run this filter on the file and commit the normalized result.",
		"",
		"Diff (disk vs filtered):",
		"",
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(diffOut), "\n"), "\n") {
		if line == "" {
			warns = append(warns, "")
			continue
		}
		warns = append(warns, "    "+line)
	}
	return warns
}
