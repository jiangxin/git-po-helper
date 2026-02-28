package util

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSplitHeader_NoComment(t *testing.T) {
	header := []string{
		"msgid \"\"",
		"msgstr \"\"",
		"\"Project-Id-Version: git\\n\"",
		"\"Content-Type: text/plain; charset=UTF-8\\n\"",
	}
	comment, meta, err := SplitHeader(header)
	if err != nil {
		t.Fatalf("SplitHeader: %v", err)
	}
	if comment != "" {
		t.Errorf("header_comment: expected empty, got %q", comment)
	}
	expectedMeta := "Project-Id-Version: git\nContent-Type: text/plain; charset=UTF-8\n"
	if meta != expectedMeta {
		t.Errorf("header_meta: got %q, want %q", meta, expectedMeta)
	}
}

func TestSplitHeader_CommentOnly(t *testing.T) {
	header := []string{
		"# Glossary:",
		"# term1\tTranslation 1",
		"#",
	}
	comment, meta, err := SplitHeader(header)
	if err != nil {
		t.Fatalf("SplitHeader: %v", err)
	}
	expectedComment := "# Glossary:\n# term1\tTranslation 1\n#\n"
	if comment != expectedComment {
		t.Errorf("header_comment: got %q, want %q", comment, expectedComment)
	}
	if meta != "" {
		t.Errorf("header_meta: expected empty, got %q", meta)
	}
}

func TestSplitHeader_CommentAndHeaderBlock(t *testing.T) {
	header := []string{
		"# Glossary:",
		"# term1\tTranslation 1",
		"#",
		"msgid \"\"",
		"msgstr \"\"",
		"\"Project-Id-Version: git\\n\"",
		"\"Content-Type: text/plain; charset=UTF-8\\n\"",
	}
	comment, meta, err := SplitHeader(header)
	if err != nil {
		t.Fatalf("SplitHeader: %v", err)
	}
	expectedComment := "# Glossary:\n# term1\tTranslation 1\n#\n"
	if comment != expectedComment {
		t.Errorf("header_comment: got %q, want %q", comment, expectedComment)
	}
	expectedMeta := "Project-Id-Version: git\nContent-Type: text/plain; charset=UTF-8\n"
	if meta != expectedMeta {
		t.Errorf("header_meta: got %q, want %q", meta, expectedMeta)
	}
}

func TestSplitHeader_MultiLineHeaderMeta(t *testing.T) {
	header := []string{
		"msgid \"\"",
		"msgstr \"\"",
		"\"Project-Id-Version: git\\n\"",
		"\"Content-Type: text/plain; charset=UTF-8\\n\"",
		"\"Plural-Forms: nplurals=2; plural=(n != 1);\\n\"",
	}
	_, meta, err := SplitHeader(header)
	if err != nil {
		t.Fatalf("SplitHeader: %v", err)
	}
	expectedMeta := "Project-Id-Version: git\nContent-Type: text/plain; charset=UTF-8\nPlural-Forms: nplurals=2; plural=(n != 1);\n"
	if meta != expectedMeta {
		t.Errorf("header_meta: got %q, want %q", meta, expectedMeta)
	}
}

func TestSplitHeader_Empty(t *testing.T) {
	comment, meta, err := SplitHeader(nil)
	if err != nil {
		t.Fatalf("SplitHeader: %v", err)
	}
	if comment != "" || meta != "" {
		t.Errorf("expected both empty, got comment=%q meta=%q", comment, meta)
	}
}

func TestBuildGettextJSON_RoundTrip(t *testing.T) {
	entries := []*PoEntry{
		{
			MsgID:    "Hello",
			MsgStr:   "你好",
			Comments: []string{"#. Comment\n", "#: src/file.c:10\n"},
			IsFuzzy:  false,
		},
		{
			MsgID:        "One file",
			MsgStr:       "",
			MsgIDPlural:  "%d files",
			MsgStrPlural: []string{"一个文件", "%d 个文件"},
			Comments:     []string{"#, c-format\n"},
			IsFuzzy:      false,
		},
	}
	var buf bytes.Buffer
	err := BuildGettextJSON("", "Project-Id-Version: git\n", entries, &buf)
	if err != nil {
		t.Fatalf("BuildGettextJSON: %v", err)
	}
	var decoded GettextJSON
	if err := json.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if decoded.HeaderMeta != "Project-Id-Version: git\n" {
		t.Errorf("HeaderMeta: got %q", decoded.HeaderMeta)
	}
	if len(decoded.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(decoded.Entries))
	}
	e0 := decoded.Entries[0]
	if e0.MsgID != "Hello" || e0.MsgStr != "你好" || e0.Fuzzy != false {
		t.Errorf("entry 0: msgid=%q msgstr=%q fuzzy=%v", e0.MsgID, e0.MsgStr, e0.Fuzzy)
	}
	e1 := decoded.Entries[1]
	if e1.MsgID != "One file" || e1.MsgStr != "" || e1.MsgIDPlural != "%d files" ||
		len(e1.MsgStrPlural) != 2 || e1.MsgStrPlural[0] != "一个文件" || e1.MsgStrPlural[1] != "%d 个文件" {
		t.Errorf("entry 1: msgid=%q msgstr=%q msgid_plural=%q msgstr_plural=%v",
			e1.MsgID, e1.MsgStr, e1.MsgIDPlural, e1.MsgStrPlural)
	}
}

func TestBuildGettextJSON_EmptyEntries(t *testing.T) {
	var buf bytes.Buffer
	err := BuildGettextJSON("# comment\n", "meta\n", nil, &buf)
	if err != nil {
		t.Fatalf("BuildGettextJSON: %v", err)
	}
	var decoded GettextJSON
	if err := json.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if decoded.HeaderComment != "# comment\n" || decoded.HeaderMeta != "meta\n" || len(decoded.Entries) != 0 {
		t.Errorf("got header_comment=%q header_meta=%q entries len=%d",
			decoded.HeaderComment, decoded.HeaderMeta, len(decoded.Entries))
	}
}

func TestPoUnescape_InMsgidMsgstr(t *testing.T) {
	entries := []*PoEntry{
		{
			MsgID:   "Line one\nLine two\twith tab",
			MsgStr:  "第一行\n第二行\t带制表符",
			IsFuzzy: false,
		},
	}
	var buf bytes.Buffer
	err := BuildGettextJSON("", "", entries, &buf)
	if err != nil {
		t.Fatalf("BuildGettextJSON: %v", err)
	}
	var decoded GettextJSON
	if err := json.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	e := decoded.Entries[0]
	wantMsgid := "Line one\nLine two\twith tab"
	wantMsgstr := "第一行\n第二行\t带制表符"
	if e.MsgID != wantMsgid {
		t.Errorf("msgid: got %q, want %q", e.MsgID, wantMsgid)
	}
	if e.MsgStr != wantMsgstr {
		t.Errorf("msgstr: got %q, want %q", e.MsgStr, wantMsgstr)
	}
}

func TestEntryRangeForJSON(t *testing.T) {
	indices, err := EntryRangeForJSON("1,3", 5)
	if err != nil {
		t.Fatalf("EntryRangeForJSON: %v", err)
	}
	if len(indices) != 2 || indices[0] != 1 || indices[1] != 3 {
		t.Errorf("got %v", indices)
	}
}

func TestSplitHeader_RealPOFromDesign(t *testing.T) {
	poContent := `# Glossary:
# term1	Translation 1
#
msgid ""
msgstr ""
"Project-Id-Version: git\n"
"Content-Type: text/plain; charset=UTF-8\n"

#. Comment for translator
#: src/file.c:10
msgid "Hello"
msgstr "你好"
`
	entries, header, err := ParsePoEntries([]byte(poContent))
	if err != nil {
		t.Fatalf("ParsePoEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	comment, meta, err := SplitHeader(header)
	if err != nil {
		t.Fatalf("SplitHeader: %v", err)
	}
	expectedComment := "# Glossary:\n# term1\tTranslation 1\n#\n"
	if comment != expectedComment {
		t.Errorf("header_comment: got %q", comment)
	}
	expectedMeta := "Project-Id-Version: git\nContent-Type: text/plain; charset=UTF-8\n"
	if meta != expectedMeta {
		t.Errorf("header_meta: got %q", meta)
	}
	var buf bytes.Buffer
	err = BuildGettextJSON(comment, meta, entries, &buf)
	if err != nil {
		t.Fatalf("BuildGettextJSON: %v", err)
	}
	var decoded GettextJSON
	if err := json.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Entries[0].MsgID != "Hello" || decoded.Entries[0].MsgStr != "你好" {
		t.Errorf("entry: %+v", decoded.Entries[0])
	}
	if len(decoded.Entries[0].Comments) != 2 {
		t.Errorf("comments: got %v", decoded.Entries[0].Comments)
	}
	if !strings.HasPrefix(decoded.Entries[0].Comments[0], "#.") ||
		!strings.HasPrefix(decoded.Entries[0].Comments[1], "#:") {
		t.Errorf("comments: %v", decoded.Entries[0].Comments)
	}
}
