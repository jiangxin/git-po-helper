package util

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestWriteGettextJSONToPO_Example2RoundTrip(t *testing.T) {
	jsonStr := `{
  "header_comment": "",
  "header_meta": "Project-Id-Version: git\nContent-Type: text/plain; charset=UTF-8\n",
  "entries": [
    {
      "msgid": "Line one\nLine two\twith tab, padding for line 2.",
      "msgstr": "第一行\n第二行\t带制表符, 第二行的填充。",
      "comments": ["#, c-format\n"],
      "fuzzy": false
    }
  ]
}`
	j, err := ParseGettextJSONBytes([]byte(jsonStr))
	if err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	var poBuf bytes.Buffer
	if err := WriteGettextJSONToPO(j, &poBuf); err != nil {
		t.Fatalf("WriteGettextJSONToPO: %v", err)
	}
	poBytes := poBuf.Bytes()
	entries, header, err := ParsePoEntries(poBytes)
	if err != nil {
		t.Fatalf("ParsePoEntries of converted PO: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after round-trip, got %d", len(entries))
	}
	e := entries[0]
	wantMsgid := "Line one\nLine two\twith tab, padding for line 2."
	wantMsgstr := "第一行\n第二行\t带制表符, 第二行的填充。"
	if poUnescape(e.MsgID) != wantMsgid {
		t.Errorf("msgid round-trip: got %q", poUnescape(e.MsgID))
	}
	if poUnescape(e.MsgStr) != wantMsgstr {
		t.Errorf("msgstr round-trip: got %q", poUnescape(e.MsgStr))
	}
	headerComment, headerMeta, _ := SplitHeader(header)
	var jsonBuf bytes.Buffer
	if err := BuildGettextJSON(headerComment, headerMeta, entries, &jsonBuf); err != nil {
		t.Fatalf("BuildGettextJSON: %v", err)
	}
	var j2 GettextJSON
	if err := json.Unmarshal(jsonBuf.Bytes(), &j2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if j2.Entries[0].MsgID != j.Entries[0].MsgID || j2.Entries[0].MsgStr != j.Entries[0].MsgStr {
		t.Errorf("round-trip JSON: msgid %q vs %q, msgstr %q vs %q",
			j2.Entries[0].MsgID, j.Entries[0].MsgID, j2.Entries[0].MsgStr, j.Entries[0].MsgStr)
	}
}

func TestWriteGettextJSONToPO_Example3PluralRoundTrip(t *testing.T) {
	jsonStr := `{
  "header_comment": "",
  "header_meta": "Project-Id-Version: git\nContent-Type: text/plain; charset=UTF-8\nPlural-Forms: nplurals=2; plural=(n != 1);\n",
  "entries": [
    {
      "msgid": "One file",
      "msgstr": "",
      "msgid_plural": "%d files",
      "msgstr_plural": ["一个文件", "%d 个文件"],
      "comments": ["#, c-format\n"],
      "fuzzy": false
    }
  ]
}`
	j, err := ParseGettextJSONBytes([]byte(jsonStr))
	if err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	var poBuf bytes.Buffer
	if err := WriteGettextJSONToPO(j, &poBuf); err != nil {
		t.Fatalf("WriteGettextJSONToPO: %v", err)
	}
	entries, _, err := ParsePoEntries(poBuf.Bytes())
	if err != nil {
		t.Fatalf("ParsePoEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.MsgID != "One file" || e.MsgStr != "" || e.MsgIDPlural != "%d files" ||
		len(e.MsgStrPlural) != 2 || e.MsgStrPlural[0] != "一个文件" || e.MsgStrPlural[1] != "%d 个文件" {
		t.Errorf("plural entry: msgid=%q msgstr=%q msgid_plural=%q msgstr_plural=%v",
			e.MsgID, e.MsgStr, e.MsgIDPlural, e.MsgStrPlural)
	}
}

func TestWriteGettextJSONToPO_SpecialChars(t *testing.T) {
	j := &GettextJSON{
		HeaderComment: "",
		HeaderMeta:    "",
		Entries: []GettextEntry{{
			MsgID:  "Quote \" and backslash \\ and tab\t and newline\n",
			MsgStr: "相同",
			Fuzzy:  false,
		}},
	}
	var buf bytes.Buffer
	if err := WriteGettextJSONToPO(j, &buf); err != nil {
		t.Fatalf("WriteGettextJSONToPO: %v", err)
	}
	entries, _, err := ParsePoEntries(buf.Bytes())
	if err != nil {
		t.Fatalf("ParsePoEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	want := "Quote \" and backslash \\ and tab\t and newline\n"
	if poUnescape(entries[0].MsgID) != want {
		t.Errorf("msgid: got %q", poUnescape(entries[0].MsgID))
	}
}

func TestRoundTrip_POToJSONToPOToJSON_Example2(t *testing.T) {
	poContent := `msgid ""
msgstr ""
"Project-Id-Version: git\n"
"Content-Type: text/plain; charset=UTF-8\n"

#, c-format
msgid ""
"Line one\n"
"Line two\twith tab, "
"padding for line 2."
msgstr ""
"第一行\n"
"第二行\t带制表符, 第二行的填充。"
`
	entries, header, err := ParsePoEntries([]byte(poContent))
	if err != nil {
		t.Fatalf("ParsePoEntries: %v", err)
	}
	headerComment, headerMeta, err := SplitHeader(header)
	if err != nil {
		t.Fatalf("SplitHeader: %v", err)
	}
	var json1Buf bytes.Buffer
	if err := BuildGettextJSON(headerComment, headerMeta, entries, &json1Buf); err != nil {
		t.Fatalf("BuildGettextJSON: %v", err)
	}
	j1, err := ParseGettextJSONBytes(json1Buf.Bytes())
	if err != nil {
		t.Fatalf("ParseGettextJSONBytes: %v", err)
	}
	var poBuf bytes.Buffer
	if err := WriteGettextJSONToPO(j1, &poBuf); err != nil {
		t.Fatalf("WriteGettextJSONToPO: %v", err)
	}
	entries2, header2, err := ParsePoEntries(poBuf.Bytes())
	if err != nil {
		t.Fatalf("ParsePoEntries (second): %v", err)
	}
	headerComment2, headerMeta2, _ := SplitHeader(header2)
	var json2Buf bytes.Buffer
	if err := BuildGettextJSON(headerComment2, headerMeta2, entries2, &json2Buf); err != nil {
		t.Fatalf("BuildGettextJSON (second): %v", err)
	}
	j2, err := ParseGettextJSONBytes(json2Buf.Bytes())
	if err != nil {
		t.Fatalf("ParseGettextJSONBytes (second): %v", err)
	}
	if len(j2.Entries) != len(j1.Entries) {
		t.Fatalf("entries count: %d vs %d", len(j2.Entries), len(j1.Entries))
	}
	if j2.Entries[0].MsgID != j1.Entries[0].MsgID || j2.Entries[0].MsgStr != j1.Entries[0].MsgStr {
		t.Errorf("round-trip: msgid %q vs %q, msgstr %q vs %q",
			j2.Entries[0].MsgID, j1.Entries[0].MsgID, j2.Entries[0].MsgStr, j1.Entries[0].MsgStr)
	}
}

func TestRoundTrip_PluralExample3(t *testing.T) {
	poContent := `msgid ""
msgstr ""
"Project-Id-Version: git\n"
"Content-Type: text/plain; charset=UTF-8\n"
"Plural-Forms: nplurals=2; plural=(n != 1);\n"

#, c-format
msgid "One file"
msgid_plural "%d files"
msgstr[0] "一个文件"
msgstr[1] "%d 个文件"
`
	entries, header, err := ParsePoEntries([]byte(poContent))
	if err != nil {
		t.Fatalf("ParsePoEntries: %v", err)
	}
	headerComment, headerMeta, _ := SplitHeader(header)
	var jsonBuf bytes.Buffer
	if err := BuildGettextJSON(headerComment, headerMeta, entries, &jsonBuf); err != nil {
		t.Fatalf("BuildGettextJSON: %v", err)
	}
	j, _ := ParseGettextJSONBytes(jsonBuf.Bytes())
	var poBuf bytes.Buffer
	if err := WriteGettextJSONToPO(j, &poBuf); err != nil {
		t.Fatalf("WriteGettextJSONToPO: %v", err)
	}
	entries2, _, err := ParsePoEntries(poBuf.Bytes())
	if err != nil {
		t.Fatalf("ParsePoEntries: %v", err)
	}
	if len(entries2) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries2))
	}
	e := entries2[0]
	if e.MsgID != "One file" || e.MsgIDPlural != "%d files" ||
		len(e.MsgStrPlural) != 2 || e.MsgStrPlural[0] != "一个文件" || e.MsgStrPlural[1] != "%d 个文件" {
		t.Errorf("plural round-trip: %+v", e)
	}
}

func TestWriteGettextJSONToPO_EmptyEntries(t *testing.T) {
	j := &GettextJSON{
		HeaderComment: "# empty\n",
		HeaderMeta:    "Project-Id-Version: git\n",
		Entries:       []GettextEntry{},
	}
	var buf bytes.Buffer
	if err := WriteGettextJSONToPO(j, &buf); err != nil {
		t.Fatalf("WriteGettextJSONToPO: %v", err)
	}
	entries, header, err := ParsePoEntries(buf.Bytes())
	if err != nil {
		t.Fatalf("ParsePoEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
	comment, meta, _ := SplitHeader(header)
	if comment != "# empty\n" || meta != "Project-Id-Version: git\n" {
		t.Errorf("header: comment=%q meta=%q", comment, meta)
	}
}

func TestSelectGettextJSONFromFile_JSONInputToPO(t *testing.T) {
	jsonContent := `{
  "header_comment": "",
  "header_meta": "Project-Id-Version: git\nContent-Type: text/plain; charset=UTF-8\n",
  "entries": [
    {
      "msgid": "Line one",
      "msgstr": "第一行",
      "comments": ["#, c-format\n"],
      "fuzzy": false
    }
  ]
}`
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "input.json")
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("write JSON file: %v", err)
	}
	var buf bytes.Buffer
	err := SelectGettextJSONFromFile(jsonFile, "1", &buf, false)
	if err != nil {
		t.Fatalf("SelectGettextJSONFromFile: %v", err)
	}
	entries, _, err := ParsePoEntries(buf.Bytes())
	if err != nil {
		t.Fatalf("ParsePoEntries of PO output: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].MsgID != "Line one" || entries[0].MsgStr != "第一行" {
		t.Errorf("entry: msgid=%q msgstr=%q", entries[0].MsgID, entries[0].MsgStr)
	}
}

func TestSelectGettextJSONFromFile_JSONInputToJSON(t *testing.T) {
	jsonContent := `{"header_comment":"","header_meta":"meta\n","entries":[{"msgid":"A","msgstr":"甲","fuzzy":false},{"msgid":"B","msgstr":"乙","fuzzy":false}]}`
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "input.json")
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("write JSON file: %v", err)
	}
	var buf bytes.Buffer
	err := SelectGettextJSONFromFile(jsonFile, "2", &buf, true)
	if err != nil {
		t.Fatalf("SelectGettextJSONFromFile: %v", err)
	}
	var decoded GettextJSON
	if err := json.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if len(decoded.Entries) != 1 || decoded.Entries[0].MsgID != "B" {
		t.Errorf("expected single entry B, got %d entries: %+v", len(decoded.Entries), decoded.Entries)
	}
}

func TestSelectGettextJSONFromFile_Range(t *testing.T) {
	jsonContent := `{"header_comment":"","header_meta":"","entries":[{"msgid":"One","msgstr":"一","fuzzy":false},{"msgid":"Two","msgstr":"二","fuzzy":false}]}`
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "input.json")
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("write JSON file: %v", err)
	}
	t.Run("range 1", func(t *testing.T) {
		var buf bytes.Buffer
		if err := SelectGettextJSONFromFile(jsonFile, "1", &buf, true); err != nil {
			t.Fatal(err)
		}
		var j GettextJSON
		if err := json.Unmarshal(buf.Bytes(), &j); err != nil {
			t.Fatal(err)
		}
		if len(j.Entries) != 1 || j.Entries[0].MsgID != "One" {
			t.Errorf("got %v", j.Entries)
		}
	})
	t.Run("range 1-2", func(t *testing.T) {
		var buf bytes.Buffer
		if err := SelectGettextJSONFromFile(jsonFile, "1-2", &buf, true); err != nil {
			t.Fatal(err)
		}
		var j GettextJSON
		if err := json.Unmarshal(buf.Bytes(), &j); err != nil {
			t.Fatal(err)
		}
		if len(j.Entries) != 2 {
			t.Errorf("got %d entries", len(j.Entries))
		}
	})
}
