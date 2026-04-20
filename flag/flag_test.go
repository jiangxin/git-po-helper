package flag

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

// setenvWithCleanup sets key for the duration of the test and restores the previous value.
// Go 1.16 has no testing.T.Setenv (added in Go 1.17); use os.Setenv/os.Unsetenv and t.Cleanup instead.
func setenvWithCleanup(t *testing.T, key, val string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	t.Cleanup(func() {
		if !had {
			_ = os.Unsetenv(key)
		} else {
			_ = os.Setenv(key, prev)
		}
	})
	if val == "" {
		_ = os.Unsetenv(key)
		return
	}
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("Setenv %s: %v", key, err)
	}
}

func TestReportTypos_githubActionsExplicitError(t *testing.T) {
	setenvWithCleanup(t, "GITHUB_ACTIONS", "true")
	k := "check-po--report-typos"
	before := viper.GetString(k)
	t.Cleanup(func() { viper.Set(k, before) })
	viper.Set(k, "error")

	if g := ReportTypos(); g != ReportIssueError {
		t.Fatalf("ReportTypos() = %d, want ReportIssueError (explicit flag must beat CI default)", g)
	}
}

func TestReportFileLocations_githubActionsExplicitNone(t *testing.T) {
	setenvWithCleanup(t, "GITHUB_ACTIONS", "true")
	k := "check-po--report-file-locations"
	before := viper.GetString(k)
	t.Cleanup(func() { viper.Set(k, before) })
	viper.Set(k, "none")

	if g := ReportFileLocations(); g != ReportIssueNone {
		t.Fatalf("ReportFileLocations() = %d, want ReportIssueNone (explicit flag must beat CI default)", g)
	}
}

func TestReportTypos_githubActionsUnsetDefaultsToWarn(t *testing.T) {
	setenvWithCleanup(t, "GITHUB_ACTIONS", "true")
	keys := []string{"check--report-typos", "check-po--report-typos", "check-commits--report-typos"}
	before := make([]string, len(keys))
	for i, k := range keys {
		before[i] = viper.GetString(k)
	}
	t.Cleanup(func() {
		for i, k := range keys {
			viper.Set(k, before[i])
		}
	})
	for _, k := range keys {
		viper.Set(k, "")
	}

	if g := ReportTypos(); g != ReportIssueWarn {
		t.Fatalf("ReportTypos() = %d, want ReportIssueWarn when unset under GitHub Actions", g)
	}
}

func TestFirstNonEmptyViperString_order(t *testing.T) {
	k1, k2 := "check--report-typos", "check-po--report-typos"
	b1, b2 := viper.GetString(k1), viper.GetString(k2)
	t.Cleanup(func() {
		viper.Set(k1, b1)
		viper.Set(k2, b2)
	})
	viper.Set(k1, "warn")
	viper.Set(k2, "error")
	if g := firstNonEmptyViperString(k1, k2); g != "warn" {
		t.Fatalf("first key should win, got %q", g)
	}
}
