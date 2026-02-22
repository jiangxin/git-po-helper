package util

import (
	"os"
	"strings"
	"testing"
)

// TestResolvePoFile tests ResolvePoFile in non-interactive scenarios.
// Interactive mode (multiple files, TTY) is not tested here.
func TestResolvePoFile(t *testing.T) {
	tests := []struct {
		name           string
		poFileArg      string
		changedPoFiles []string
		want           string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "empty arg, no changed files",
			poFileArg:      "",
			changedPoFiles: []string{},
			wantErr:        true,
			errContains:    "no changed po files found",
		},
		{
			name:           "empty arg, one changed file",
			poFileArg:      "",
			changedPoFiles: []string{"po/zh_CN.po"},
			want:           "po/zh_CN.po",
			wantErr:        false,
		},
		{
			name:           "empty arg, multiple changed files (non-interactive)",
			poFileArg:      "",
			changedPoFiles: []string{"po/zh_CN.po", "po/zh_TW.po"},
			wantErr:        true,
			errContains:    "multiple changed po files found",
		},
		{
			name:           "specified arg, valid",
			poFileArg:      "po/zh_CN.po",
			changedPoFiles: []string{"po/zh_CN.po", "po/zh_TW.po"},
			want:           "po/zh_CN.po",
			wantErr:        false,
		},
		{
			name:           "specified arg with basename only",
			poFileArg:      "zh_CN.po",
			changedPoFiles: []string{"po/zh_CN.po"},
			want:           "zh_CN.po",
			wantErr:        false,
		},
		{
			name:           "specified arg, not in changed",
			poFileArg:      "po/ja.po",
			changedPoFiles: []string{"po/zh_CN.po", "po/zh_TW.po"},
			wantErr:        true,
			errContains:    "is not in the changed files",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For "multiple changed" test, ensure we're non-interactive (no TTY)
			if tt.wantErr && strings.Contains(tt.errContains, "multiple") {
				oldStdin := os.Stdin
				oldStdout := os.Stdout
				r, w, _ := os.Pipe()
				os.Stdin = r
				os.Stdout = w
				defer func() {
					os.Stdin = oldStdin
					os.Stdout = oldStdout
				}()
			}

			got, err := ResolvePoFile(tt.poFileArg, tt.changedPoFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolvePoFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ResolvePoFile() = %v, want %v", got, tt.want)
			}
			if tt.wantErr && tt.errContains != "" && err != nil && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("ResolvePoFile() error = %v, want contains %q", err, tt.errContains)
			}
		})
	}
}
