// Package util provides review-related utilities for agent-run.
package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

// PrepareReviewData prepares data for review by creating orig.po, new.po, and review-input.po files.
// It gets the original file from git, sorts both files by msgid, and extracts differences.
func PrepareReviewData(poFile, commit, since, outputFile string) error {
	var (
		err        error
		workDir    = repository.WorkDir()
		poFileName = filepath.Base(poFile)
		langCode   = strings.TrimSuffix(poFileName, ".po")
	)
	if langCode == "" || langCode == poFileName {
		return fmt.Errorf("invalid PO file path: %s (expected format: po/XX.po)", poFile)
	}

	// Use temp files for orig and new; they are deleted when the function returns
	origFile, err := os.CreateTemp("", fmt.Sprintf("%s-orig-*.po", langCode))
	if err != nil {
		return fmt.Errorf("failed to create temp orig file: %w", err)
	}
	origPath := origFile.Name()
	origFile.Close()

	newFile, err := os.CreateTemp("", fmt.Sprintf("%s-new-*.po", langCode))
	if err != nil {
		os.Remove(origPath)
		return fmt.Errorf("failed to create temp new file: %w", err)
	}
	newPath := newFile.Name()
	newFile.Close()

	defer func() {
		os.Remove(origPath)
		os.Remove(newPath)
	}()

	log.Debugf("preparing review data: orig=%s, new=%s, review-input=%s", origPath, newPath, outputFile)

	// Determine the base commit for comparison and the new file source
	var baseCommit string
	var newFileSource string
	if commit != "" {
		// Commit mode: orig is parent commit, new is the specified commit
		cmd := exec.Command("git", "rev-parse", commit+"^")
		cmd.Dir = workDir
		output, err := cmd.Output()
		if err != nil {
			// If commit has no parent (root commit), use empty tree
			baseCommit = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // Empty tree
		} else {
			baseCommit = strings.TrimSpace(string(output))
		}
		newFileSource = commit
		log.Infof("commit mode: orig from %s, new from %s", baseCommit, commit)
	} else if since != "" {
		// Since mode: orig is since commit, new is current file
		baseCommit = since
		newFileSource = "" // Use current file
		log.Infof("since mode: orig from %s, new from current file", since)
	} else {
		// Default mode: orig is HEAD, new is current file
		baseCommit = "HEAD"
		newFileSource = "" // Use current file
		log.Infof("default mode: orig from HEAD, new from current file")
	}

	// Get original file from git
	log.Infof("getting original file from commit: %s", baseCommit)
	// Convert absolute path to relative path for git show command
	poFileRel, err := filepath.Rel(workDir, poFile)
	if err != nil {
		return fmt.Errorf("failed to convert PO file path to relative: %w", err)
	}
	// Normalize to use forward slashes (git uses forward slashes in paths)
	poFileRel = filepath.ToSlash(poFileRel)
	origFileRevision := FileRevision{
		Revision: baseCommit,
		File:     poFileRel,
	}
	if err := checkoutTmpfile(&origFileRevision); err != nil {
		// Check if error is because file doesn't exist in the commit
		if strings.Contains(err.Error(), "does not exist in") {
			// If file doesn't exist in that commit, create empty file
			log.Infof("file %s not found in commit %s, using empty file as original", poFileRel, baseCommit)
			if err := os.WriteFile(origPath, []byte{}, 0644); err != nil {
				return fmt.Errorf("failed to create empty orig file: %w", err)
			}
		} else {
			// For other errors, return them
			return fmt.Errorf("failed to get original file from commit %s: %w", baseCommit, err)
		}
	} else {
		// Copy tmpfile to orig.po
		origData, err := os.ReadFile(origFileRevision.Tmpfile)
		defer func() {
			os.Remove(origFileRevision.Tmpfile)
		}()
		if err != nil {
			return fmt.Errorf("failed to read orig tmpfile: %w", err)
		}
		if err := os.WriteFile(origPath, origData, 0644); err != nil {
			return fmt.Errorf("failed to write orig file: %w", err)
		}
	}

	// Get new file (either from git commit or current file)
	if newFileSource != "" {
		// Get file from specified commit
		log.Debugf("getting new file from commit: %s", newFileSource)
		// Use the same relative path for git show command
		newFileRevision := FileRevision{
			Revision: newFileSource,
			File:     poFileRel,
		}
		if err := checkoutTmpfile(&newFileRevision); err != nil {
			return fmt.Errorf("failed to get new file from commit %s: %w", newFileSource, err)
		}
		defer func() {
			os.Remove(newFileRevision.Tmpfile)
		}()
		newData, err := os.ReadFile(newFileRevision.Tmpfile)
		if err != nil {
			return fmt.Errorf("failed to read new tmpfile: %w", err)
		}
		if err := os.WriteFile(newPath, newData, 0644); err != nil {
			return fmt.Errorf("failed to write new file: %w", err)
		}
	} else {
		// Copy current file to new.po
		log.Debugf("copying current file to new.po")
		newData, err := os.ReadFile(poFile)
		if err != nil {
			return fmt.Errorf("failed to read current PO file: %w", err)
		}
		if err := os.WriteFile(newPath, newData, 0644); err != nil {
			return fmt.Errorf("failed to write new file: %w", err)
		}
	}

	// Extract differences: use msgcmp to find entries that are different or new
	// We'll use a simpler approach: extract entries from new that don't match orig
	log.Debugf("extracting differences to review-input.po")
	if err := extractReviewInput(origPath, newPath, outputFile); err != nil {
		return fmt.Errorf("failed to extract review input: %w", err)
	}

	log.Infof("review data prepared: review-input=%s", outputFile)
	return nil
}

// extractReviewInput extracts entries from new.po that are different or new compared to orig.po.
// It copies the header and first empty msgid entry from new.po, then adds all entries
// that are new or different in new.po.
func extractReviewInput(origPath, newPath, outputPath string) error {
	// Read both files
	origData, err := os.ReadFile(origPath)
	if err != nil {
		return fmt.Errorf("failed to read orig file: %w", err)
	}
	newData, err := os.ReadFile(newPath)
	if err != nil {
		return fmt.Errorf("failed to read new file: %w", err)
	}

	// Parse entries from both files
	origEntries, _, err := ParsePoEntries(origData)
	if err != nil {
		return fmt.Errorf("failed to parse orig file: %w", err)
	}
	newEntries, newHeader, err := ParsePoEntries(newData)
	if err != nil {
		return fmt.Errorf("failed to parse new file: %w", err)
	}

	// If orig file is empty, all entries in new file will be considered new
	// This handles the case where the file doesn't exist in HEAD
	if len(origData) == 0 {
		log.Debugf("orig file is empty, all entries in new file will be included in review-input")
	}

	// Create a map of orig entries by msgid for quick lookup
	origMap := make(map[string]*PoEntry)
	for _, entry := range origEntries {
		origMap[entry.MsgID] = entry
	}

	// Extract entries that are new or different
	var reviewEntries []*PoEntry
	for _, newEntry := range newEntries {
		origEntry, exists := origMap[newEntry.MsgID]
		if !exists {
			// New entry
			reviewEntries = append(reviewEntries, newEntry)
		} else if !entriesEqual(origEntry, newEntry) {
			// Different entry (msgid or msgstr changed)
			reviewEntries = append(reviewEntries, newEntry)
		}
		// If entry exists and is equal, skip it
	}

	// Write review-input.po with header and review entries
	return writeReviewInputPo(outputPath, newHeader, reviewEntries)
}

// entriesEqual checks if two PO entries are equal (same msgid and msgstr).
func entriesEqual(e1, e2 *PoEntry) bool {
	if e1.MsgID != e2.MsgID {
		return false
	}
	if e1.MsgStr != e2.MsgStr {
		return false
	}
	if e1.MsgIDPlural != e2.MsgIDPlural {
		return false
	}
	if len(e1.MsgStrPlural) != len(e2.MsgStrPlural) {
		return false
	}
	for i := range e1.MsgStrPlural {
		if e1.MsgStrPlural[i] != e2.MsgStrPlural[i] {
			return false
		}
	}
	return true
}

// writeReviewInputPo writes the review input PO file with header and review entries.
// When outputPath is "-" or "" and entries is empty, writes nothing (for new-entries command).
func writeReviewInputPo(outputPath string, header []string, entries []*PoEntry) error {
	if (outputPath == "-" || outputPath == "") && len(entries) == 0 {
		return nil
	}

	var content strings.Builder

	// Write header
	// Header structure:
	// - Comments (if any) - lines starting with #
	// - msgid "" - line starting with msgid
	// - msgstr "" - line starting with msgstr
	// - Continuation lines - already wrapped in quotes (preserved from parsePoEntries)
	if len(entries) > 0 {
		for _, line := range header {
			content.WriteString(line)
			// Only add newline if the line doesn't already end with \n
			if !strings.HasSuffix(line, "\n") {
				content.WriteString("\n")
			}
		}

		// Add empty line after header
		content.WriteString("\n")
	}

	// Write entries
	for _, entry := range entries {
		for _, line := range entry.RawLines {
			content.WriteString(line)
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	data := []byte(content.String())
	if outputPath == "-" || outputPath == "" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}
