// Package util provides gettext JSON format support for PO entry selection (msg-select --json).
package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

// GettextJSON is the top-level structure for msg-select --json output.
type GettextJSON struct {
	HeaderComment string         `json:"header_comment"`
	HeaderMeta    string         `json:"header_meta"`
	Entries       []GettextEntry `json:"entries"`
}

// GettextEntry represents one PO entry in the JSON format.
type GettextEntry struct {
	MsgID         string   `json:"msgid"`
	MsgStr        string   `json:"msgstr"`
	MsgIDPlural   string   `json:"msgid_plural,omitempty"`
	MsgStrPlural  []string `json:"msgstr_plural,omitempty"`
	Comments      []string `json:"comments,omitempty"`
	Fuzzy         bool     `json:"fuzzy"`
	Obsolete      bool     `json:"obsolete,omitempty"`       // True for #~ obsolete entries
	MsgIDPrevious string   `json:"msgid_previous,omitempty"` // For #~| format (gettext 0.19.8+)
}

// poEscape encodes a string for PO quoted output: backslash, quote, newline, tab, carriage return.
func poEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
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

// PoEntriesToGettextJSON builds a GettextJSON from header and PO entries (all entries, no range).
func PoEntriesToGettextJSON(headerComment, headerMeta string, entries []*PoEntry) *GettextJSON {
	out := &GettextJSON{
		HeaderComment: headerComment,
		HeaderMeta:    headerMeta,
		Entries:       make([]GettextEntry, 0, len(entries)),
	}
	for _, e := range entries {
		ent := GettextEntry{
			MsgID:         poUnescape(e.MsgID),
			MsgStr:        poUnescape(e.MsgStr),
			Fuzzy:         e.IsFuzzy,
			Obsolete:      e.IsObsolete,
			MsgIDPrevious: poUnescape(e.MsgIDPrevious),
		}
		// Strip fuzzy from comments; fuzzy state lives only in ent.Fuzzy. Drop empty lines.
		for _, c := range e.Comments {
			if stripped := StripFuzzyFromCommentLine(c); stripped != "" {
				ent.Comments = append(ent.Comments, stripped)
			}
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
	return out
}

// BuildGettextJSON builds the JSON object from header comment, header meta, and selected entries,
// and writes it to w. Entries should already be range-selected (e.g. from MsgSelect flow).
func BuildGettextJSON(headerComment, headerMeta string, entries []*PoEntry, w io.Writer) error {
	return WriteGettextJSONToJSON(PoEntriesToGettextJSON(headerComment, headerMeta, entries), w)
}

// WriteGettextJSONToJSON writes a GettextJSON value as JSON to w (same schema as --json output).
func WriteGettextJSONToJSON(j *GettextJSON, w io.Writer) error {
	if j == nil {
		j = &GettextJSON{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(j); err != nil {
		return fmt.Errorf("encode gettext JSON: %w", err)
	}
	return nil
}

// parseGettextJSONWithGjson parses gettext JSON using gjson, which can tolerate
// some malformed LLM output (e.g. missing colons). Returns nil if parsing fails.
func parseGettextJSONWithGjson(data []byte, err error) *GettextJSON {
	log.Warnf("fall back to gjson to fix gettext JSON: %v", err)
	headerComment := gjson.GetBytes(data, "header_comment").String()
	headerMeta := gjson.GetBytes(data, "header_meta").String()
	entriesResult := gjson.GetBytes(data, "entries")
	if !entriesResult.Exists() {
		return &GettextJSON{
			HeaderComment: headerComment,
			HeaderMeta:    headerMeta,
			Entries:       []GettextEntry{},
		}
	}
	var entries []GettextEntry
	for _, r := range entriesResult.Array() {
		ent := GettextEntry{
			MsgID:         r.Get("msgid").String(),
			MsgStr:        r.Get("msgstr").String(),
			Fuzzy:         r.Get("fuzzy").Bool(),
			Obsolete:      r.Get("obsolete").Bool(),
			MsgIDPrevious: r.Get("msgid_previous").String(),
			Comments:      []string{},
		}
		if r.Get("msgid_plural").Exists() {
			ent.MsgIDPlural = r.Get("msgid_plural").String()
		}
		if arr := r.Get("msgstr_plural").Array(); len(arr) > 0 {
			ent.MsgStrPlural = make([]string, len(arr))
			for i, v := range arr {
				ent.MsgStrPlural[i] = v.String()
			}
		}
		if arr := r.Get("comments").Array(); len(arr) > 0 {
			ent.Comments = make([]string, len(arr))
			for i, v := range arr {
				ent.Comments[i] = v.String()
			}
		}
		entries = append(entries, ent)
	}
	return &GettextJSON{
		HeaderComment: headerComment,
		HeaderMeta:    headerMeta,
		Entries:       entries,
	}
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
// Uses PrepareJSONForParse and gjson fallback for malformed LLM-generated JSON.
func ParseGettextJSONBytes(data []byte) (*GettextJSON, error) {
	var out GettextJSON
	if err := json.Unmarshal(data, &out); err != nil {
		prepared := PrepareJSONForParse(data, err)
		if err2 := json.Unmarshal(prepared, &out); err2 != nil {
			if parsed := parseGettextJSONWithGjson(prepared, err2); parsed != nil {
				return parsed, nil
			}
			return nil, fmt.Errorf("decode gettext JSON: %w", err)
		}
	}
	if out.Entries == nil {
		out.Entries = []GettextEntry{}
	}
	return &out, nil
}

// EntryRangeForJSON applies the same range semantics as ParseEntryRange to a JSON entries slice.
// maxEntry is len(entries). Returns indices in ascending order (1-based content indices).
func EntryRangeForJSON(spec string, maxEntry int) ([]int, error) {
	return ParseEntryRange(spec, maxEntry)
}

// entryKey returns a key for deduplication: same key means same logical entry (msgid + msgid_plural).
func entryKey(e GettextEntry) string {
	if e.MsgIDPlural != "" {
		return e.MsgID + "\x00" + e.MsgIDPlural
	}
	return e.MsgID + "\x00"
}

// MergeGettextJSON merges multiple GettextJSON sources. Header is taken from the first source.
// For entries, the first occurrence of each msgid (and msgid_plural for plurals) wins by file order.
func MergeGettextJSON(sources []*GettextJSON) *GettextJSON {
	if len(sources) == 0 {
		return &GettextJSON{}
	}
	seen := make(map[string]bool)
	var merged []GettextEntry
	for _, j := range sources {
		if j == nil {
			continue
		}
		for _, e := range j.Entries {
			k := entryKey(e)
			if seen[k] {
				continue
			}
			seen[k] = true
			merged = append(merged, e)
		}
	}
	return &GettextJSON{
		HeaderComment: sources[0].HeaderComment,
		HeaderMeta:    sources[0].HeaderMeta,
		Entries:       merged,
	}
}

// ClearFuzzyTagFromGettextJSON clears only the fuzzy marker from all entries.
// Sets entry.Fuzzy = false and strips "fuzzy" from #, flag lines in Comments.
// Translation content (msgstr, msgstr_plural) is preserved.
func ClearFuzzyTagFromGettextJSON(j *GettextJSON) {
	if j == nil {
		return
	}
	for i := range j.Entries {
		j.Entries[i].Fuzzy = false
		var newComments []string
		for _, c := range j.Entries[i].Comments {
			trimmed := strings.TrimSpace(c)
			if strings.HasPrefix(trimmed, "#,") {
				stripped := StripFuzzyFromFlagLine(c)
				if stripped != "" {
					newComments = append(newComments, stripped+"\n")
				}
			} else {
				newComments = append(newComments, c)
			}
		}
		j.Entries[i].Comments = newComments
	}
}

// ClearFuzzyFromGettextJSON clears the fuzzy marker and empties translation
// (msgstr, msgstr_plural) for entries that were fuzzy. msgid and msgid_plural
// are preserved. Non-fuzzy entries are unchanged.
func ClearFuzzyFromGettextJSON(j *GettextJSON) {
	if j == nil {
		return
	}
	for i := range j.Entries {
		wasFuzzy := j.Entries[i].Fuzzy
		j.Entries[i].Fuzzy = false
		var newComments []string
		for _, c := range j.Entries[i].Comments {
			trimmed := strings.TrimSpace(c)
			if strings.HasPrefix(trimmed, "#,") {
				stripped := StripFuzzyFromFlagLine(c)
				if stripped != "" {
					newComments = append(newComments, stripped+"\n")
				}
			} else {
				newComments = append(newComments, c)
			}
		}
		j.Entries[i].Comments = newComments
		if wasFuzzy {
			j.Entries[i].MsgStr = ""
			if len(j.Entries[i].MsgStrPlural) > 0 {
				for k := range j.Entries[i].MsgStrPlural {
					j.Entries[i].MsgStrPlural[k] = ""
				}
			}
		}
	}
}

// ReadFileToGettextJSON reads a single file (PO, POT, or gettext JSON) and returns GettextJSON.
// Format is detected by extension (.json) or by content (starts with '{' after whitespace).
func ReadFileToGettextJSON(path string) (*GettextJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return ParseGettextJSONBytes(data)
	}
	// PO/POT
	entries, header, err := ParsePoEntries(data)
	if err != nil {
		return nil, fmt.Errorf("parse PO %s: %w", path, err)
	}
	headerComment, headerMeta, err := SplitHeader(header)
	if err != nil {
		return nil, fmt.Errorf("split header %s: %w", path, err)
	}
	return PoEntriesToGettextJSON(headerComment, headerMeta, entries), nil
}

// SelectGettextJSONFromFile reads a gettext JSON file, applies state filter and range.
// If filter is nil, DefaultFilter() is used. Range applies to the filtered list.
func SelectGettextJSONFromFile(jsonFile, rangeSpec string, w io.Writer, useJSON bool, filter *EntryStateFilter) error {
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", jsonFile, err)
	}
	j, err := ParseGettextJSONBytes(data)
	if err != nil {
		return fmt.Errorf("failed to parse JSON %s: %w", jsonFile, err)
	}
	f := DefaultFilter()
	if filter != nil {
		f = *filter
	}
	filtered := FilterGettextEntries(j.Entries, f)
	maxEntry := len(filtered)
	indices, err := EntryRangeForJSON(rangeSpec, maxEntry)
	if err != nil {
		return fmt.Errorf("invalid range %q: %w", rangeSpec, err)
	}
	var selected []GettextEntry
	for _, idx := range indices {
		if idx > 0 && idx <= len(filtered) {
			selected = append(selected, filtered[idx-1])
		}
	}
	out := &GettextJSON{
		HeaderComment: j.HeaderComment,
		HeaderMeta:    j.HeaderMeta,
		Entries:       selected,
	}
	if useJSON {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(out); err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
		return nil
	}
	return WriteGettextJSONToPO(out, w)
}

// WriteGettextJSONToPO writes the GettextJSON object as valid PO content to w.
// Header comment is written as raw lines (split on newline); header meta is written
// as msgid "" / msgstr "" with PO-escaped continuation lines. Each entry is written
// with comments, msgid/msgstr (multi-line with PO escaping when needed), and #, fuzzy if set.
func WriteGettextJSONToPO(j *GettextJSON, w io.Writer) error {
	if j == nil {
		return nil
	}
	// Header comment: lines above first msgid ""
	if j.HeaderComment != "" {
		lines := strings.Split(strings.TrimSuffix(j.HeaderComment, "\n"), "\n")
		for _, line := range lines {
			if _, err := io.WriteString(w, line); err != nil {
				return err
			}
			if !strings.HasSuffix(line, "\n") {
				if _, err := io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
		}
	}
	// Header block: msgid "" and msgstr "" with continuation lines
	if _, err := io.WriteString(w, "msgid \"\"\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "msgstr \"\"\n"); err != nil {
		return err
	}
	if j.HeaderMeta != "" {
		parts := strings.Split(j.HeaderMeta, "\n")
		for i, part := range parts {
			var content string
			if i < len(parts)-1 {
				content = part + "\n"
			} else if part != "" {
				content = part
			} else {
				continue
			}
			if _, err := io.WriteString(w, "\""+poEscape(content)+"\"\n"); err != nil {
				return err
			}
		}
	}
	if len(j.Entries) > 0 || j.HeaderComment != "" || j.HeaderMeta != "" {
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	for ei, entry := range j.Entries {
		// Comments: restore fuzzy from entry.Fuzzy (merge into flag line or add standalone)
		wroteFuzzyFlag := false
		for _, c := range entry.Comments {
			trimmed := strings.TrimSpace(c)
			if strings.HasPrefix(trimmed, "#,") {
				line := MergeFuzzyIntoFlagLine(c, entry.Fuzzy)
				if entry.Fuzzy {
					wroteFuzzyFlag = true
				}
				if _, err := io.WriteString(w, line+"\n"); err != nil {
					return err
				}
			} else {
				if _, err := io.WriteString(w, c); err != nil {
					return err
				}
				if !strings.HasSuffix(c, "\n") {
					if _, err := io.WriteString(w, "\n"); err != nil {
						return err
					}
				}
			}
		}
		if entry.Fuzzy && !wroteFuzzyFlag {
			if _, err := io.WriteString(w, "#, fuzzy\n"); err != nil {
				return err
			}
		}
		prefix := ""
		if entry.Obsolete {
			prefix = "#~ "
		}
		// #~| msgid (previous untranslated string, gettext 0.19.8+)
		if entry.Obsolete && entry.MsgIDPrevious != "" {
			if err := writePoStringWithPrefix(w, "#~| ", "msgid", entry.MsgIDPrevious); err != nil {
				return err
			}
		}
		// msgid (single- or multi-line)
		if err := writePoStringWithPrefix(w, prefix, "msgid", entry.MsgID); err != nil {
			return err
		}
		if entry.MsgIDPlural != "" {
			if err := writePoStringWithPrefix(w, prefix, "msgid_plural", entry.MsgIDPlural); err != nil {
				return err
			}
		}
		if len(entry.MsgStrPlural) > 0 {
			for i, s := range entry.MsgStrPlural {
				if err := writePoStringWithPrefix(w, prefix, "msgstr["+strconv.Itoa(i)+"]", s); err != nil {
					return err
				}
			}
		} else {
			if err := writePoStringWithPrefix(w, prefix, "msgstr", entry.MsgStr); err != nil {
				return err
			}
		}
		if ei < len(j.Entries)-1 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

// writePoStringWithPrefix writes a keyword and value with optional prefix (e.g. "#~ " for obsolete).
func writePoStringWithPrefix(w io.Writer, prefix, keyword, value string) error {
	parts := strings.Split(value, "\n")
	if len(parts) == 1 {
		_, err := io.WriteString(w, prefix+keyword+" \""+poEscape(value)+"\"\n")
		return err
	}
	if _, err := io.WriteString(w, prefix+keyword+" \"\"\n"); err != nil {
		return err
	}
	for i, p := range parts {
		if i < len(parts)-1 {
			if _, err := io.WriteString(w, prefix+"\""+poEscape(p)+"\\n\"\n"); err != nil {
				return err
			}
		} else if p != "" {
			if _, err := io.WriteString(w, prefix+"\""+poEscape(p)+"\"\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
