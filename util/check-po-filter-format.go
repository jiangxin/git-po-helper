package util

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/git-l10n/git-po-helper/flag"
	"github.com/git-l10n/git-po-helper/repository"
)

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
// the materialized content path under the worktree (skipped when that path is outside the repo).
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

	attrSourceCommit := strings.TrimSpace(fr.Revision)

	displayPath := contentPath

	var relPath string
	if fr.File != "" && !filepath.IsAbs(fr.File) {
		displayPath = fr.File
		rel, err := GetRepoRelPath(fr.File)
		if err != nil {
			if errors.Is(err, ErrOutsideWorktree) {
				errs = append(errs, fmt.Sprintf("filter attr path escapes repository: %s", fr.File))
				return errs, false
			}
			errs = append(errs, err.Error())
			return errs, false
		}
		relPath = rel
	} else {
		rel, err := GetRepoRelPath(contentPath)
		if err != nil {
			if errors.Is(err, ErrOutsideWorktree) {
				// File is outside repo and no logical repo-relative path; skip filter check
				return nil, true
			}
			errs = append(errs, err.Error())
			return errs, false
		}
		relPath = rel
	}

	filterValue := strings.TrimSpace(filterAttribute)
	if filterValue == "" {
		var checkAttrArgs []string
		if repository.IsBare() {
			// Bare repos have no work tree; -C workDir would be invalid (empty workDir).
			checkAttrArgs = []string{"check-attr"}
		} else {
			workDir := repository.WorkDir()
			checkAttrArgs = []string{"-C", workDir, "check-attr"}
		}
		// Query git check-attr [--source=<rev>] filter <path>
		if attrSourceCommit != "" {
			checkAttrArgs = append(checkAttrArgs, "--source="+attrSourceCommit)
		}
		checkAttrArgs = append(checkAttrArgs, "filter", relPath)
		cmd := exec.Command("git", checkAttrArgs...)
		cmd.Stderr = nil
		out, err := cmd.Output()
		if err != nil {
			errs = append(errs, fmt.Sprintf("git check-attr failed for %s: %s", displayPath, err))
			return errs, false
		}

		// Parse: "path: filter: value"
		line := strings.TrimSpace(string(out))
		parts := strings.SplitN(line, ": ", 3)
		if len(parts) < 3 {
			errs = append(errs, fmt.Sprintf("unexpected git check-attr output: %s", line))
			return errs, false
		}
		filterValue = strings.TrimSpace(parts[2])
	}

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
	}

	effectiveFilter := filterValue
	if !missingFilter && filterValue != "gettext-no-location" && filterValue != "gettext-no-line-number" {
		if len(errs) > 0 {
			errs = append(errs, "")
		}
		errs = append(errs, fmt.Sprintf(
			"Unsupported filter attribute %q for %s; "+
				`using "gettext-no-location" rules as fallback (PO must not contain #: location comments). `+
				`Prefer filter=gettext-no-location or filter=gettext-no-line-number in .gitattributes.`,
			filterValue, displayPath))
		effectiveFilter = "gettext-no-location"
	}

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
