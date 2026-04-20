// Package flag provides viper flags.
package flag

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

const (
	ReportIssueNone = iota
	ReportIssueWarn
	ReportIssueError
)

// Verbose returns option "--verbose".
func Verbose() int {
	return viper.GetInt("verbose")
}

// Quiet returns option "--quiet".
func Quiet() int {
	return viper.GetInt("quiet")
}

// Force returns option "--force".
func Force() bool {
	return viper.GetBool("check--force") || viper.GetBool("check-commits--force")
}

// GitHubActionEvent returns option "--github-action-event".
// When not set, uses GITHUB_EVENT_NAME from GitHub Actions (e.g. pull_request, push).
// If that is unset but GITHUB_ACTIONS is true, returns "workflow" so CI defaults
// (e.g. --pot-file download, skipping local PO filter clean comparison) still apply.
func GitHubActionEvent() string {
	if v := strings.TrimSpace(viper.GetString("github-action-event")); v != "" {
		return v
	}
	if ev := strings.TrimSpace(os.Getenv("GITHUB_EVENT_NAME")); ev != "" {
		return ev
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GITHUB_ACTIONS")), "true") {
		return "workflow"
	}
	return ""
}

// firstNonEmptyViperString returns the first non-empty strings.TrimSpace(viper.GetString(k)) among keys.
func firstNonEmptyViperString(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(viper.GetString(k)); v != "" {
			return v
		}
	}
	return ""
}

// NoGPG returns option "--no-gpg".
// In GitHub Actions this is always true (no override): CI jobs typically have no usable signing keys.
func NoGPG() bool {
	return GitHubActionEvent() != "" || viper.GetBool("check--no-gpg") || viper.GetBool("check-commits--no-gpg")
}

// ReportTypos returns way to display typos (none, warn, error).
// In GitHub Actions, defaults to warn when no --report-typos was set; an explicit flag value always wins.
func ReportTypos() int {
	value := firstNonEmptyViperString("check--report-typos", "check-po--report-typos", "check-commits--report-typos")

	// In GitHub Actions, default to warn so typo noise does not fail the job unless configured.
	// Explicit --report-typos=... on check-po / check-commits (or check-- alias) always wins.
	if GitHubActionEvent() != "" && value == "" {
		return ReportIssueWarn
	}
	switch value {
	case "none":
		return ReportIssueNone
	case "warn":
		return ReportIssueWarn
	case "error":
		fallthrough
	default:
		return ReportIssueError
	}
}

// AllowObsoleteEntries returns true when obsolete entries should be allowed
// (e.g. after msgmerge in update flow, which creates obsolete entries by design).
func AllowObsoleteEntries() bool {
	return viper.GetBool("check--allow-obsolete")
}

// ReportFileLocations returns way to display file-location / filter issues (none, warn, error).
// In GitHub Actions, defaults to error when no --report-file-locations was set; an explicit flag wins.
func ReportFileLocations() int {
	value := firstNonEmptyViperString(
		"check--report-file-locations",
		"check-po--report-file-locations",
		"check-commits--report-file-locations",
	)
	if value == "" && GitHubActionEvent() != "" {
		return ReportIssueError
	}
	switch value {
	case "none":
		return ReportIssueNone
	case "warn":
		return ReportIssueWarn
	case "error":
		fallthrough
	default:
		return ReportIssueError
	}
}

// IsPotFileSet returns true when --pot-file was explicitly set by user.
func IsPotFileSet() bool {
	return viper.IsSet("pot-file")
}

// GetPotFileRaw returns the raw --pot-file value.
func GetPotFileRaw() string {
	return viper.GetString("pot-file")
}

// Core returns option "--core".
func Core() bool {
	return viper.GetBool("check--core") || viper.GetBool("check-po--core")
}

// NoCheckFilter returns true when --no-check-filter is set (check-po / check-commits),
// skipping PO .gitattributes filter, filter clean-output comparison, and related checks.
func NoCheckFilter() bool {
	return viper.GetBool("check--no-check-filter") ||
		viper.GetBool("check-po--no-check-filter") ||
		viper.GetBool("check-commits--no-check-filter")
}

// NoSpecialGettextVersions returns option "--no-special-gettext-versions".
func NoSpecialGettextVersions() bool {
	return viper.GetBool("no-special-gettext-versions")
}

// SetGettextUseMultipleVersions sets option "gettext-use-multiple-versions".
func SetGettextUseMultipleVersions(value bool) {
	viper.Set("gettext-use-multiple-versions", value)
}

// GettextUseMultipleVersions returns option "gettext-use-multiple-versions".
func GettextUseMultipleVersions() bool {
	return viper.GetBool("gettext-use-multiple-versions")
}

// GetConfigFilePath returns option "--config" (custom agent config file path).
// If non-empty, agent config is loaded only from this file.
func GetConfigFilePath() string {
	return viper.GetString("config")
}
