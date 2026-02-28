// Package util provides gettext JSON format support for PO entry selection (msg-select --json).
package util

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// GettextJSON is the top-level structure for msg-select --json output.
type GettextJSON struct {
	HeaderComment string         `json:"header_comment"`
	HeaderMeta    string         `json:"header_meta"`
	Entries       []GettextEntry `json:"entries"`
}

// GettextEntry represents one PO entry in the JSON format.
type GettextEntry struct {
	MsgID        string   `json:"msgid"`
	MsgStr       string   `json:"msgstr"`
	MsgIDPlural  string   `json:"msgid_plural,omitempty"`
	MsgStrPlural []string `json:"msgstr_plural,omitempty"`
	Comments     []string `json:"comments,omitempty"`
	Fuzzy        bool     `json:"fuzzy"`
}

// poUnescape decodes PO escape sequences in s into real characters.
// PO uses \n (newline), \t (tab), \r (carriage return), \" (quote), \\ (backslash).
func poUnescape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
			case 't':
				b.WriteByte('\t')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			case '"':
				b.WriteByte('"')
				i++
			case '\\':
				b.WriteByte('\\')
				i++
			default:
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// SplitHeader splits header lines from ParsePoEntries into header_comment and header_meta.
// headerComment is lines before the first "msgid "" (after trim), joined with "\n".
// headerMeta is the decoded msgstr value of the header entry (unescaped).
func SplitHeader(header []string) (headerComment, headerMeta string, err error) {
	if len(header) == 0 {
		return "", "", nil
	}
	var commentLines []string
	var i int
	for i = 0; i < len(header); i++ {
		trimmed := strings.TrimSpace(header[i])
		if strings.HasPrefix(trimmed, "msgid ") {
			value := strings.TrimPrefix(trimmed, "msgid ")
			value = strings.TrimSpace(value)
			value = strDeQuote(value)
			if value == "" {
				break
			}
		}
		commentLines = append(commentLines, header[i])
	}
	if len(commentLines) > 0 {
		headerComment = strings.Join(commentLines, "\n") + "\n"
	}
	if i >= len(header) {
		return headerComment, "", nil
	}
	// Collect msgstr "" and continuation lines for header_meta
	var msgstrLines []string
	for i++; i < len(header); i++ {
		trimmed := strings.TrimSpace(header[i])
		if strings.HasPrefix(trimmed, "msgstr ") {
			value := strings.TrimPrefix(trimmed, "msgstr ")
			value = strings.TrimSpace(value)
			value = strDeQuote(value)
			msgstrLines = append(msgstrLines, value)
		} else if strings.HasPrefix(trimmed, `"`) {
			value := strDeQuote(trimmed)
			msgstrLines = append(msgstrLines, value)
		} else {
			break
		}
	}
	if len(msgstrLines) > 0 {
		headerMeta = poUnescape(strings.Join(msgstrLines, ""))
	}
	return headerComment, headerMeta, nil
}

// BuildGettextJSON builds the JSON object from header comment, header meta, and selected entries,
// and writes it to w. Entries should already be range-selected (e.g. from MsgSelect flow).
func BuildGettextJSON(headerComment, headerMeta string, entries []*PoEntry, w io.Writer) error {
	out := GettextJSON{
		HeaderComment: headerComment,
		HeaderMeta:    headerMeta,
		Entries:       make([]GettextEntry, 0, len(entries)),
	}
	for _, e := range entries {
		ent := GettextEntry{
			MsgID:    poUnescape(e.MsgID),
			MsgStr:   poUnescape(e.MsgStr),
			Comments: e.Comments,
			Fuzzy:    e.IsFuzzy,
		}
		if e.MsgIDPlural != "" {
			ent.MsgIDPlural = poUnescape(e.MsgIDPlural)
		}
		if len(e.MsgStrPlural) > 0 {
			ent.MsgStrPlural = make([]string, len(e.MsgStrPlural))
			for i, s := range e.MsgStrPlural {
				ent.MsgStrPlural[i] = poUnescape(s)
			}
		}
		if ent.Comments == nil {
			ent.Comments = []string{}
		}
		out.Entries = append(out.Entries, ent)
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode gettext JSON: %w", err)
	}
	return nil
}

// ParseGettextJSON decodes gettext JSON from r into GettextJSON.
func ParseGettextJSON(r io.Reader) (*GettextJSON, error) {
	var out GettextJSON
	if err := json.NewDecoder(r).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode gettext JSON: %w", err)
	}
	return &out, nil
}

// ParseGettextJSONBytes decodes gettext JSON from data.
func ParseGettextJSONBytes(data []byte) (*GettextJSON, error) {
	var out GettextJSON
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode gettext JSON: %w", err)
	}
	return &out, nil
}

// EntryRangeForJSON applies the same range semantics as ParseEntryRange to a JSON entries slice.
// maxEntry is len(entries). Returns indices in ascending order (1-based content indices).
func EntryRangeForJSON(spec string, maxEntry int) ([]int, error) {
	return ParseEntryRange(spec, maxEntry)
}
