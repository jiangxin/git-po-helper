// Package util provides PO file parsing utilities.
package util

import (
	"fmt"
	"strings"
)

// PoEntry represents a single PO file entry.
type PoEntry struct {
	Comments     []string
	MsgID        string
	MsgStr       string
	MsgIDPlural  string
	MsgStrPlural []string
	RawLines     []string // Original lines for the entry
}

// ParsePoEntries parses PO file entries and returns entries and header.
// The header includes comments, the empty msgid/msgstr block, and any continuation lines.
// Entries are 1-based for content (header entry with empty msgid is not included).
func ParsePoEntries(data []byte) (entries []*PoEntry, header []string, err error) {
	lines := strings.Split(string(data), "\n")
	var currentEntry *PoEntry
	var inHeader = true
	var headerLines []string
	var entryLines []string
	var msgidValue strings.Builder
	var msgstrValue strings.Builder
	var msgidPluralValue strings.Builder
	var msgstrPluralValues []strings.Builder
	var inMsgid, inMsgstr, inMsgidPlural bool
	var currentPluralIndex int = -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for header (empty msgid entry)
		if inHeader && strings.HasPrefix(trimmed, "msgid ") {
			value := strings.TrimPrefix(trimmed, "msgid ")
			value = strings.Trim(value, `"`)
			if value == "" {
				// This is the header entry
				inHeader = true
				headerLines = append(headerLines, line)
				entryLines = append(entryLines, line)
				// Continue to collect header
				continue
			}
		}

		// Check for header msgstr (empty msgstr after empty msgid)
		if inHeader && strings.HasPrefix(trimmed, "msgstr ") {
			value := strings.TrimPrefix(trimmed, "msgstr ")
			value = strings.Trim(value, `"`)
			if msgidValue.Len() == 0 && value == "" {
				// This is the header msgstr line
				headerLines = append(headerLines, line)
				// Continue collecting header (including continuation lines starting with ")
				// Header ends when we encounter an empty line or a new msgid entry
				continue
			}
		}

		// Collect header lines (including continuation lines after msgstr "")
		if inHeader {
			// Check if this is a continuation line of header msgstr (starts with ")
			// Only collect as header if we're still in header mode and haven't started parsing an entry
			// Also check that we're not in the middle of parsing a msgid or msgstr (which would indicate an entry)
			if strings.HasPrefix(trimmed, `"`) {
				// If we're already parsing an entry (currentEntry exists or inMsgid/inMsgstr is set),
				// this continuation line belongs to the entry, not the header
				if currentEntry != nil || inMsgid || inMsgstr || inMsgidPlural {
					// This is a continuation line of an entry, not header
					// Don't process it here, let it be handled by entry parsing logic below
				} else {
					// For header continuation lines, keep the quotes
					headerLines = append(headerLines, trimmed)
					continue
				}
			}
			// Check if this is an empty line - end of header
			if trimmed == "" {
				inHeader = false
				msgidValue.Reset()
				msgstrValue.Reset()
				continue
			}
			// Check if this is a new msgid entry - end of header
			if strings.HasPrefix(trimmed, "msgid ") {
				value := strings.TrimPrefix(trimmed, "msgid ")
				value = strings.Trim(value, `"`)
				if value != "" {
					// This is a real entry, not header
					inHeader = false
					msgidValue.Reset()
					msgstrValue.Reset()
					// Don't continue, let it be processed as a normal entry
				} else {
					// This is a duplicate empty msgid after header - this should not happen
					// in a valid PO file, but if it does, end the header and start a new entry
					inHeader = false
					msgidValue.Reset()
					msgstrValue.Reset()
					// Don't continue, let it be processed as a normal entry
				}
			} else {
				// Other header lines (comments, etc.)
				headerLines = append(headerLines, line)
				continue
			}
		}

		// Parse entry
		if strings.HasPrefix(trimmed, "#") {
			// Comment line
			if currentEntry == nil {
				currentEntry = &PoEntry{}
				entryLines = []string{}
			}
			currentEntry.Comments = append(currentEntry.Comments, line)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, "msgid ") {
			// Start of new entry
			// Save previous entry if we have one and it has content
			// (either msgid with continuation lines or msgstr)
			if currentEntry != nil && (msgidValue.Len() > 0 || msgstrValue.Len() > 0) {
				// Save previous entry
				currentEntry.MsgID = msgidValue.String()
				currentEntry.MsgStr = msgstrValue.String()
				currentEntry.RawLines = entryLines
				entries = append(entries, currentEntry)
			}
			// Start new entry (or continue existing entry if it only has comments)
			if currentEntry == nil {
				// Create a new entry
				currentEntry = &PoEntry{}
				entryLines = []string{}
			} else if msgidValue.Len() > 0 || msgstrValue.Len() > 0 {
				// Previous entry was saved, create new entry
				currentEntry = &PoEntry{}
				entryLines = []string{}
			}
			// If currentEntry has comments but no msgid/msgstr, keep it and continue
			// entryLines already contains the comments, so we don't reset it
			msgidValue.Reset()
			msgstrValue.Reset()
			msgidPluralValue.Reset()
			msgstrPluralValues = []strings.Builder{}
			inMsgid = true
			inMsgstr = false
			inMsgidPlural = false
			currentPluralIndex = -1

			value := strings.TrimPrefix(trimmed, "msgid ")
			value = strings.Trim(value, `"`)
			msgidValue.WriteString(value)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, "msgid_plural ") {
			inMsgid = false
			inMsgidPlural = true
			value := strings.TrimPrefix(trimmed, "msgid_plural ")
			value = strings.Trim(value, `"`)
			msgidPluralValue.WriteString(value)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, "msgstr[") {
			// Plural form
			inMsgid = false
			inMsgidPlural = false
			inMsgstr = true
			// Extract index
			idxStr := strings.TrimPrefix(trimmed, "msgstr[")
			idxStr = strings.Split(idxStr, "]")[0]
			var idx int
			fmt.Sscanf(idxStr, "%d", &idx)
			// Extend slice if needed
			for len(msgstrPluralValues) <= idx {
				msgstrPluralValues = append(msgstrPluralValues, strings.Builder{})
			}
			currentPluralIndex = idx
			value := strings.TrimPrefix(trimmed, fmt.Sprintf("msgstr[%d] ", idx))
			value = strings.Trim(value, `"`)
			msgstrPluralValues[idx].WriteString(value)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, "msgstr ") {
			inMsgid = false
			inMsgidPlural = false
			inMsgstr = true
			value := strings.TrimPrefix(trimmed, "msgstr ")
			value = strings.Trim(value, `"`)
			msgstrValue.WriteString(value)
			entryLines = append(entryLines, line)
		} else if strings.HasPrefix(trimmed, `"`) && (inMsgid || inMsgstr || inMsgidPlural) {
			// Continuation line
			value := strings.Trim(trimmed, `"`)
			if inMsgid {
				msgidValue.WriteString(value)
			} else if inMsgidPlural {
				msgidPluralValue.WriteString(value)
			} else if inMsgstr {
				if currentPluralIndex >= 0 {
					msgstrPluralValues[currentPluralIndex].WriteString(value)
				} else {
					msgstrValue.WriteString(value)
				}
			}
			entryLines = append(entryLines, line)
		} else if trimmed == "" {
			// Empty line - end of entry (only if we have a current entry)
			// For entries with msgid starting with empty string, we need to check
			// if we have collected any continuation lines (msgidValue.Len() > 0)
			// or if we have a complete entry with msgstr
			if currentEntry != nil && (msgidValue.Len() > 0 || msgstrValue.Len() > 0) {
				currentEntry.MsgID = msgidValue.String()
				currentEntry.MsgStr = msgstrValue.String()
				if msgidPluralValue.Len() > 0 {
					currentEntry.MsgIDPlural = msgidPluralValue.String()
					currentEntry.MsgStrPlural = make([]string, len(msgstrPluralValues))
					for i, b := range msgstrPluralValues {
						currentEntry.MsgStrPlural[i] = b.String()
					}
				}
				currentEntry.RawLines = entryLines
				entries = append(entries, currentEntry)
			}
			currentEntry = nil
			entryLines = []string{}
			msgidValue.Reset()
			msgstrValue.Reset()
			msgidPluralValue.Reset()
			msgstrPluralValues = []strings.Builder{}
			inMsgid = false
			inMsgstr = false
			inMsgidPlural = false
			currentPluralIndex = -1
		} else {
			// Other lines (continuation, etc.)
			if currentEntry != nil {
				entryLines = append(entryLines, line)
			} else if !inHeader {
				// If we're not in header and not in an entry, this might be a continuation
				// of a previous entry or a new entry starting
				entryLines = append(entryLines, line)
			}
		}
	}

	// Handle last entry
	// For entries with msgid starting with empty string, we need to check
	// if we have collected any continuation lines (msgidValue.Len() > 0)
	// or if we have a complete entry with msgstr
	if currentEntry != nil && (msgidValue.Len() > 0 || msgstrValue.Len() > 0) {
		currentEntry.MsgID = msgidValue.String()
		currentEntry.MsgStr = msgstrValue.String()
		if msgidPluralValue.Len() > 0 {
			currentEntry.MsgIDPlural = msgidPluralValue.String()
			currentEntry.MsgStrPlural = make([]string, len(msgstrPluralValues))
			for i, b := range msgstrPluralValues {
				currentEntry.MsgStrPlural[i] = b.String()
			}
		}
		currentEntry.RawLines = entryLines
		entries = append(entries, currentEntry)
	}

	return entries, headerLines, nil
}
