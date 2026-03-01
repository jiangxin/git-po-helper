# Instructions for AI Agents

This file gives specific instructions for AI agents that perform
housekeeping tasks for Git l10n. Use of AI is optional; many successful
l10n teams work well without it.

The section "Housekeeping tasks for localization workflows" documents the
most commonly used housekeeping tasks:

1. Generating or updating po/git.pot
2. Updating po/XX.po
3. Translating po/XX.po
4. Review translation quality


## Background knowledge for localization workflows

Essential background for the workflows below; understand these concepts before
performing any housekeeping tasks in this document.

### Language code and notation (XX, ll, ll\_CC)

XX is a placeholder for the language code. The code is either `ll` (ISO 639)
or `ll_CC` (e.g. `de`, `zh_CN` for Simplified Chinese). It appears in the PO
file's header entry metadata (e.g. `"Language: zh_CN\n"`) and is typically used
as the filename: `po/XX.po`.


### Header Entry

Every PO file (`po/XX.po`) contains a special entry called the "header entry"
at the beginning of the file. This entry has an empty `msgid` and contains
metadata about the translation in its `msgstr`:

```po
msgid ""
msgstr ""
"Project-Id-Version: Git\n"
"Report-Msgid-Bugs-To: Git Mailing List <git@vger.kernel.org>\n"
"POT-Creation-Date: 2026-02-14 13:38+0800\n"
"PO-Revision-Date: 2026-02-14 11:41+0800\n"
"Last-Translator: Teng Long <dyroneteng@gmail.com>\n"
"Language-Team: GitHub <https://github.com/dyrone/git/>\n"
"Language: zh_CN\n"
"MIME-Version: 1.0\n"
"Content-Type: text/plain; charset=UTF-8\n"
"Content-Transfer-Encoding: 8bit\n"
"Plural-Forms: nplurals=2; plural=(n != 1);\n"
"X-Generator: Gtranslator 42.0\n"
```

**CRITICAL**: Do not modify the header's `msgstr` during translation. Extracted
files (e.g. `po/XX.po.pending`) include this header; preserve it exactly.

The header provides: translation metadata (translator, language, dates);
pluralization rules (`Plural-Forms`); encoding and MIME type; project/version.


### Glossary Section

PO files may have a glossary in comments before the header entry (first
`msgid ""`), giving terminology guidelines:

```po
# Git glossary for Chinese translators
#
#   English                          |  Chinese
#   ---------------------------------+--------------------------------------
#   3-way merge                      |  三路合并
#   branch                           |  分支
#   commit                           |  提交
#   ...
```

**IMPORTANT**: Read and use the glossary when translating or reviewing. It is
in `#` comments and is preserved when extracting with `msgattrib`.


### Single-line vs Multi-line Entries

**Single-line entries**:
```po
msgid "commit message"
msgstr "提交说明"
```

**Multi-line entries** (first line of `msgid` and `msgstr` is empty string):
```po
msgid ""
"Line 1\n"
"Line 2"
msgstr ""
"行 1\n"
"行 2"
```

**CRITICAL** for multi-line: first line is `msgid ""` / `msgstr ""`; following
lines are quoted strings; use `\n` for line breaks. Preserve quotes and
structure exactly.


### Locating untranslated, fuzzy, and obsolete entries

**The commands below are used in "Task 3: translating po/XX.po".** For
translation tasks, follow Task 3 steps strictly; do not run these commands in
isolation.

This section describes how to locate untranslated, fuzzy, and obsolete entries.
Do **not** use `grep '^msgstr ""$'`—it matches multi-line entries and causes
false positives. Use `msgattrib`:

- **Untranslated**: `msgattrib --untranslated --no-obsolete po/XX.po`
- **Fuzzy**: `msgattrib --only-fuzzy --no-obsolete po/XX.po`
- **Obsolete** (`#~`): `msgattrib --obsolete --no-wrap po/XX.po`

To get only message IDs:
`msgattrib --untranslated --no-obsolete po/XX.po | sed -n '/^msgid /,/^$/p'`
(Same pattern for fuzzy with `--only-fuzzy`.)

When counting entries, the header is included; subtract 1 to exclude it.


### Preserving Special Characters

Preserve escape sequences (`\n`, `\"`, `\\`, `\t`), placeholders (`%s`, `%d`,
etc.), and quotes exactly as in `msgid`. Only reorder placeholders with
positional syntax when needed (see Placeholder Reordering below).

**Correct**: `msgstr "行 1\n行 2"` (keep `\n` as escape).
**Wrong**: `"行 1\\n行 2"` or actual line breaks inside the string.


### Placeholder Reordering

When reordering placeholders from the original `msgid`, use positional syntax
(`%n$`) so each argument maps to the correct value. Keep width/precision
modifiers and put the position before them.

**Example 1** (precision):
```po
#, c-format
msgid "missing environment variable '%s' for configuration '%.*s'"
msgstr "配置 '%3$.*2$s' 缺少环境变量 '%1$s'"
```
`%s` → argument 1 → `%1$s`. `%.*s` needs precision (arg 2) and string (arg 3) →
`%3$.*2$s`.

**Example 2** (multi-line, four `%s` reordered):
```po
#, c-format
msgid ""
"the 'submodule.%s.gitdir' config does not exist for module '%s'. Please "
"ensure it is set, for example by running something like: 'git config "
"submodule.%s.gitdir .git/modules/%s'. For details see the "
"extensions.submodulePathConfig documentation."
msgstr ""
"模块 '%2$s' 的 'submodule.%1$s.gitdir' 配置不存在。请确保已设置，例如运行类"
"似：'git config submodule.%3$s.gitdir .git/modules/%4$s'。详细信息请参见 "
"extensions.submodulePathConfig 文档。"
```

Original order 1,2,3,4; in translation 2,1,3,4. Each line must be a complete
quoted string.

**Rules**: Use `%n$` (n = 1-based position); place position before
width/precision; for `%.*s` map both precision and string; verify all
placeholders are mapped.


### Validating PO File Format

Validate any PO file (e.g. `po/XX.po`, `po/XX.po.pending`):

```shell
msgfmt --check -o /dev/null po/XX.po
```

Common validation errors include:
- Unclosed quotes
- Missing escape sequences
- Invalid placeholder syntax
- Malformed multi-line entries
- Incorrect line breaks in multi-line strings

**Handling validation errors with automatic repair**:
When `msgfmt` reports an error, it provides the line number where the error
was detected. Use this information to locate and fix the issue.


#### Extracting full context for review

**This subsection is part of "Task 4: review translation quality".** Do not
run it in isolation. Follow the full review flow strictly according to the
steps in Task 4.

Plain `git diff` or `git show` can fragment and lose PO context, or mistakenly
treat the whole file as the review scope, which does not match the intended
review. **You MUST** use `git-po-helper compare` to generate the source file
for translation review (redirect output to a file):

```shell
# Review local changes (HEAD vs working tree)
git-po-helper compare po/XX.po -o po/review.po

# Review changes in a specific commit (parent vs commit)
git-po-helper compare --commit <commit> po/XX.po -o po/review.po

# Review changes since a commit (commit vs working tree)
git-po-helper compare --since <commit> po/XX.po -o po/review.po

# Review between two commits
git-po-helper compare -r <commit1>..<commit2> po/XX.po -o po/review.po

# Compare two worktree files
git-po-helper compare po/XX-old.po po/XX-new.po -o po/review.po
```

**Options summary**

| Option              | Meaning                                        |
|---------------------|------------------------------------------------|
| (none)              | Compare HEAD with working tree (local changes) |
| `--commit <commit>` | Compare parent of commit with the commit       |
| `--since <commit>`  | Compare commit with working tree               |
| `-r x..y`           | Compare revision x with revision y             |
| `-r x..`            | Compare revision x with working tree           |
| `-r x`              | Compare parent of x with x                     |

Output is empty when there are no new or changed entries; otherwise it
includes a valid PO header.


#### Splitting large PO file for review

When the review file is too large, use `git-po-helper msg-select` to split by
entry index: 0 = header (default; use `--no-header` to omit), 1,2,3,... =
content entries. Ranges: `--range "1-50"`, `--range "-50"` (first 50),
`--range "51-"` (from 51 to end).

```shell
git-po-helper msg-select --range "-50" po/review.po -o po/review-batch1.po
git-po-helper msg-select --range "51-100" po/review.po -o po/review-batch2.po
git-po-helper msg-select --range "101-" po/review.po -o po/review-batch3.po
git-po-helper msg-select --range "1-50" --no-header po/review.po -o po/review-fragment.po
```


### Quality checklist

- **Accuracy**: Faithful to original meaning; no omissions or distortions.
- **Terminology**: Consistent with glossary or domain standards.
- **Grammar and fluency**: Correct and natural in the target language.
- **Placeholders**: Preserve variables (`%s`, `{name}`, `$1`) exactly; use
  positional parameters when reordering (see above).
- **Plurals and gender**: Correct forms and agreement.
- **Context fit**: Suitable for UI space, tone, and use (e.g. error vs. tooltip).
- **Cultural appropriateness**: No offensive or ambiguous content.
- **Consistency**: Match prior translations of the same source.
- **Technical integrity**: Do not translate code, paths, commands, brands, or
  proper nouns.
- **Readability**: Clear, concise, and user-friendly.


## Housekeep tasks for localization workflows

This section describes the four housekeeping tasks listed in the introduction.
Read "Background knowledge for localization workflows" above before performing
any task.


### Task 1: Generating or updating po/git.pot

When asked to "update po/git.pot" or similar requests:

1. **Directly execute** the command `make po/git.pot` without checking
   if the file exists beforehand.

2. **Do not verify** the generated file after execution. Simply run the
   command and consider the task complete.

The command will handle all necessary steps including file creation or
update automatically.


### Task 2: Updating po/XX.po

When asked to "update po/XX.po" or similar requests (where XX is a
language code):

1. **Directly execute** the command `make po-update PO_FILE=po/XX.po`
   without reading or checking the file content beforehand.

2. **Do not verify, translate or review** the updated file after execution. Simply run the
   command and consider the task complete.

The command will handle all necessary steps including generating
"po/git.pot" and merging new translatable strings into "po/XX.po"
automatically.


### Task 3: Translating po/XX.po

When asked to translate `po/XX.po`, follow the steps below. The workflow
**automatically selects** the tool based on availability: use `git-po-helper`
if present, otherwise use gettext tools. Translate every untranslated and
fuzzy entry; do not stop before the loop completes.

1. **Extract entries to translate**: Generate `po/XX.po.pending` with
   untranslated and fuzzy messages. Run the appropriate extraction based on
   tool availability:

   ```shell
   po_extract_pending () {
       test $# -ge 1 || { echo "Usage: po_extract_pending <po-file>" >&2; exit 1; }
       PO_FILE="$1"
       PENDING="${PO_FILE}.pending"
       rm -f po/l10n-todo.json po/l10n-done.json

       if command -v git-po-helper >/dev/null 2>&1
       then
           git-po-helper msg-select --untranslated --fuzzy --no-obsolete -o "$PENDING" "$PO_FILE" &&
       else
           msgattrib --untranslated --no-obsolete "$PO_FILE" >"${PO_FILE}.untranslated"
           msgattrib --only-fuzzy --no-obsolete --clear-fuzzy --empty "$PO_FILE" >"${PO_FILE}.fuzzy"
           msgattrib --only-fuzzy --no-obsolete "$PO_FILE" >"${PO_FILE}.fuzzy.reference"
           msgcat --use-first "${PO_FILE}.untranslated" "${PO_FILE}.fuzzy" >"$PENDING"
           rm -f "${PO_FILE}.untranslated" "${PO_FILE}.fuzzy"
       fi
       if test ! -s "$PENDING"
       then
           rm -f "$PENDING"
       fi

   }
   # Run the extraction. Example: po_extract_pending po/zh_CN.po
   po_extract_pending po/XX.po
   ```

   If the generated `po/XX.po.pending` file is empty or does not exist, you
   **MUST** skip to the last step (clean up). Translation is complete;
   do not run further translation steps.

2. **Prepare one batch for translation**: **BEFORE translating**, run the
   script below. It prepares a single batch per execution (aligned with
   gettext complexity; no need to split into multiple files at once).

   Output: `po/l10n-todo.json` (git-po-helper) or `po/l10n-todo.po` (gettext only).

   ```shell
   l10n_one_batch () {
       test $# -ge 1 || { echo "Usage: l10n_one_batch <po-file> [min_batch_size]" >&2; exit 1; }
       PO_FILE="$1"
       min_batch_size=${2:-50}
       PENDING="${PO_FILE}.pending"
       rm -f po/l10n-todo.json po/l10n-done.json po/l10n-todo.po po/l10n-done.po

       ENTRY_COUNT=$(grep -c '^msgid ' "$PENDING" 2>/dev/null || true)
       ENTRY_COUNT=$((ENTRY_COUNT > 0 ? ENTRY_COUNT - 1 : 0))

       if test "$ENTRY_COUNT" -gt $((min_batch_size * 2))
       then
           if test "$ENTRY_COUNT" -gt $((min_batch_size * 8))
           then
               NUM=$((min_batch_size * 2))
           elif test "$ENTRY_COUNT" -gt $((min_batch_size * 4))
           then
               NUM=$((min_batch_size + min_batch_size / 2))
           else
               NUM=$min_batch_size
           fi
           BATCHING=1
       else
           NUM=$ENTRY_COUNT
           BATCHING=
       fi

       if command -v git-po-helper >/dev/null 2>&1
       then
           if test -n "$BATCHING"
           then
               git-po-helper msg-select --json --range "-$NUM" "$PENDING" -o po/l10n-todo.json
               echo "Processing batch of $NUM entries (out of $ENTRY_COUNT remaining)"
           else
               git-po-helper msg-select --json "$PENDING" -o po/l10n-todo.json
               echo "Processing all $ENTRY_COUNT entries at once"
           fi
       else
           if test -n "$BATCHING"
           then
               awk -v num="$NUM" '/^msgid / && count++ > num {exit} 1' "$PENDING" |
                   tac | awk '/^$/ {found=1} found' | tac >po/l10n-todo.po
               echo "Processing batch of $NUM entries (out of $ENTRY_COUNT remaining)"
           else
               cp "$PENDING" po/l10n-todo.po
               echo "Processing all $ENTRY_COUNT entries at once"
           fi
       fi
   }
   # Run the extraction. Example: l10n_one_batch po/zh_CN.po 50
   l10n_one_batch po/XX.po 50
   ```

3. **Reference glossary**: Read the glossary from the pending file header
   (see "Glossary Section" above) and use it for consistent terminology.

4. **Translate entries**: Translate the batch file into the target language;
   write results to the corresponding done file:

   - **JSON** (`po/l10n-todo.json`): Write to `po/l10n-done.json`. See
     "GETTEXT JSON format and translation notes (Task 3)" below for format
     details and translation rules.
   - **PO** (`po/l10n-todo.po`): Write to `po/l10n-done.po`. Optionally
     reference `po/XX.po.fuzzy.reference` for fuzzy context.

   **LLM must**: Preserve formatting (quotes, line breaks, escape sequences);
   only modify `msgstr` content; use context and glossary; no regex or string
   replacement. For plurals, fill all `msgstr[n]` per `Plural-Forms`. See
   "Placeholder Reordering" above for `%n$` when reordering.

5. **Validate and merge**:

   ```shell
   l10n_merge_batch () {
       test $# -ge 1 || { echo "Usage: l10n_merge_batch <po-file>" >&2; exit 1; }
       PO_FILE="$1"
       MERGED="${PO_FILE}.merged"

       if test -f po/l10n-done.json
       then
           git-po-helper msg-cat --unset-fuzzy -o po/l10n-done.po po/l10n-done.json || exit 1
       fi

       if test -f po/l10n-done.po
       then
           msgfmt --check -o /dev/null po/l10n-done.po || exit 1
           msgcat --use-first po/l10n-done.po "$PO_FILE" >"$MERGED" || exit 1
           mv "$MERGED" "$PO_FILE"
           rm -f po/l10n-todo.json po/l10n-done.json po/l10n-todo.po po/l10n-done.po
       else
           echo "Error: no po/l10n-done.json or po/l10n-done.po found." >&2
           exit 1
       fi
   }
   l10n_merge_batch po/XX.po
   ```

   If validation fails: do not merge; fix errors per "Validating PO File
   Format"; re-extract and re-translate if repair fails.

6. **Repeat steps 1–5** until `po/XX.po.pending` is empty (or does not exist).
   Do not stop early.

7. **Final verification**:

   ```shell
   # Final check
   UNTRANS=$(msgattrib --untranslated --no-obsolete po/XX.po 2>/dev/null | grep -c '^msgid ' || true)
   UNTRANS=$((UNTRANS > 0 ? UNTRANS - 1 : 0))
   FUZZY=$(msgattrib --only-fuzzy --no-obsolete po/XX.po 2>/dev/null | grep -c '^msgid ' || true)
   FUZZY=$((FUZZY > 0 ? FUZZY - 1 : 0))
   if test "$UNTRANS" -eq 0 && test "$FUZZY" -eq 0
   then
       echo "Translation complete! All entries translated."
   else
       echo "WARNING: Still have $UNTRANS untranslated + $FUZZY fuzzy entries."
       echo "Do not clean up. Continue with step 1."
       exit 1
   fi
   ```

8. **Clean up** (only after step 7 passes):

   ```shell
   po_cleanup () {
       rm -f "po/XX.po.pending"
       rm -f "po/XX.po.untranslated"
       rm -f "po/XX.po.fuzzy"
       rm -f "po/XX.po.fuzzy.reference"
       rm -f "po/XX.po.merged"
       rm -f "po/l10n-todo.json"
       rm -f "po/l10n-done.json"
       rm -f "po/l10n-todo.po"
       rm -f "po/l10n-done.po"
       echo "Cleanup complete. Translation finished successfully."
   }
   # Run cleanup
   po_cleanup
   ```

#### GETTEXT JSON format and translation notes (Task 3)

When `l10n_one_batch` produces `po/l10n-todo.json`, the file uses the
**GETTEXT JSON** format. `git-po-helper msg-select --json` and
`git-po-helper msg-cat` read and write this format.

**Top-level structure**:

```json
{
  "header_comment": "string",
  "header_meta": "string",
  "entries": [ /* array of entry objects */ ]
}
```

| Field            | Description                                                                    |
|------------------|--------------------------------------------------------------------------------|
| `header_comment` | Lines above the first `msgid ""` (comments, glossary). Joined with `\n`.       |
| `header_meta`    | Decoded `msgstr` of the header entry (Project-Id-Version, Plural-Forms, etc.). |
| `entries`        | List of PO entries. Order matches source.                                      |

**Entry object** (each element of `entries`):

| Field           | Type     | Description                                           |
|-----------------|----------|-------------------------------------------------------|
| `msgid`         | string   | Singular message ID. PO escapes decoded.              |
| `msgstr`        | string   | Singular message string. Empty for plural entries.    |
| `msgid_plural`  | string   | Plural form of msgid. Omit for non-plural.            |
| `msgstr_plural` | []string | Array of msgstr[0], msgstr[1], … Omit for non-plural. |
| `comments`      | []string | Comment lines (`#`, `#.`, `#:`, `#,`, etc.).          |
| `fuzzy`         | bool     | True if entry has fuzzy flag.                         |
| `obsolete`      | bool     | True for `#~` obsolete entries. Omit if false.        |

**Example (single-line entry)**:

```json
{
  "header_comment": "# Glossary:\n# term1\tTranslation 1\n#\n",
  "header_meta": "Project-Id-Version: git\nContent-Type: text/plain; charset=UTF-8\n",
  "entries": [
    {
      "msgid": "Hello",
      "msgstr": "你好",
      "comments": ["#. Comment for translator\n", "#: src/file.c:10\n"],
      "fuzzy": false
    }
  ]
}
```

**Example (plural entry)**:

```json
{
  "msgid": "One file",
  "msgstr": "",
  "msgid_plural": "%d files",
  "msgstr_plural": ["一个文件", "%d 个文件"],
  "comments": ["#, c-format\n"],
  "fuzzy": false
}
```

**Example (fuzzy entry before translation)**:

```json
{
  "msgid": "Old message",
  "msgstr": "旧翻译",
  "comments": ["#, fuzzy\n"],
  "fuzzy": true
}
```

**Translation notes for GETTEXT JSON**:

- **Preserve structure**: Keep `header_comment`, `header_meta`, `comments`,
  `msgid`, `msgid_plural` unchanged. Only modify `msgstr` and `msgstr_plural`.
- **Fuzzy entries**: Entries extracted from fuzzy PO entries have `"fuzzy": true`.
  After translating, **remove the `fuzzy` field** or set it to `false` in the
  output (`po/l10n-done.json`). The merge step uses `--unset-fuzzy`; keeping
  `fuzzy: true` in JSON may cause the merged PO to retain the fuzzy flag.
- **Special characters**: JSON strings use decoded values (e.g. real newlines,
  not `\n`). Preserve them when writing `msgstr` and `msgstr_plural`.
- **Placeholders**: Preserve `%s`, `%d`, etc. exactly; use `%n$` when
  reordering (see "Placeholder Reordering" above).

For the full schema reference, see the "GETTEXT JSON File Format (Reference)"
section in the msg-select JSON design document.


### Task 4: Review translation quality

Review can target the full `po/XX.po`, a specific commit, or changes since a
commit. When asked to review, follow the steps below. **Note**: Review uses
`git-po-helper compare`; if `git-po-helper` is not available, this task
cannot be performed.

1. **Check for existing review**: Run in order:

   - If `po/review.po` does **not** exist, go to step 2 regardless of any other
     files (e.g. batch or JSON files).
   - If both `po/review.po` and `po/review.json` exist, go directly to the
     final step (Merge and summary) and display the report. Do **not** check
     for batch or other temporary files; no further review steps are needed.
   - If `po/review.po` exists but `po/review.json` does **not** exist, go to
     step 4 (Check batch files and select current batch) to continue the
     previous unfinished review.

2. **Extract entries**: Run `git-po-helper compare` with the desired range and
   redirect output to `po/review.po`. Do not use `git show` or `git diff`—they
   can fragment or lose PO context, or treat the whole file as the review scope
   (see "Extracting full context for review" above).

3. **Prepare review batches**: Run the script below to clean up any leftover
   files from previous reviews and split `po/review.po` into one or multiple
   `po/review-batch-<N>.po` files (dynamic batch sizing). Run as a single
   script (define the function, then call it):

   ```shell
   review_split_batches () {
       min_batch_size=${1:-50}
       rm -f po/review-batch-*.po
       rm -f po/review-batch-*.json
       rm -f po/review.json

       ENTRY_COUNT=$(grep -c '^msgid ' po/review.po 2>/dev/null || true)
       ENTRY_COUNT=$((ENTRY_COUNT > 0 ? ENTRY_COUNT - 1 : 0))

       if test "$ENTRY_COUNT" -gt $((min_batch_size * 2))
       then
           if test "$ENTRY_COUNT" -gt $((min_batch_size * 8))
           then
               NUM=$((min_batch_size * 2))
           elif test "$ENTRY_COUNT" -gt $((min_batch_size * 4))
           then
               NUM=$((min_batch_size + min_batch_size / 2))
           else
               NUM=$min_batch_size
           fi
           BATCH_COUNT=$(( (ENTRY_COUNT + NUM - 1) / NUM ))
           for i in $(seq 1 "$BATCH_COUNT")
           do
               START=$(((i - 1) * NUM + 1))
               END=$((i * NUM))
               if test "$END" -gt "$ENTRY_COUNT"
               then
                   END=$ENTRY_COUNT
               fi
               if test "$i" -eq 1
               then
                   git-po-helper msg-select --range "-$NUM" po/review.po -o "po/review-batch-$i.po"
               elif test "$END" -ge "$ENTRY_COUNT"
               then
                   git-po-helper msg-select --range "$START-" po/review.po -o "po/review-batch-$i.po"
               else
                   git-po-helper msg-select --range "$START-$END" po/review.po -o "po/review-batch-$i.po"
               fi
           done
       else
           cp po/review.po po/review-batch-1.po
       fi
   }
   review_split_batches 50
   ```

4. **Check batch files and select current batch**: If no `po/review-batch-*.po`
   exist, proceed to step 9. Otherwise take the **first** remaining file
   (smallest <N>) as the current batch; in steps 5–8 "current batch file"
   means `po/review-batch-<N>.po`. Enables resume after an unexpected stop.

5. **Read context**: Use "Background knowledge for localization workflows"
   for PO format, placeholder rules, and terminology. If the current batch
   file has a glossary section, add it to your context.

6. **Review entries**:
   - Do not review or modify the header entry (empty `msgid`, metadata in
     `msgstr`).
   - For all other entries in the current batch file, check against "Quality
     checklist" above.
   - Apply corrections to `po/review.po` (not the batch file); the human
     translator decides whether to apply them to `po/XX.po`.
   - **Do NOT** remove `po/review.po` or `po/*.json`.

7. **Generate review report**:
   - Save the report for the current batch to `po/review-batch-<N>.json`.
   - Use the review JSON format below.
   - Only include entries with issues you found, perfect entries with score 3
     should not be included.
   - Optionally provide inline suggestions or a human-readable report.

8. **Repeat or finish**: After saving the JSON, delete
   `po/review-batch-<N>.po`. If no `po/review-batch-*.po` remain, proceed to
   step 9; otherwise repeat from step 4.

9. **Merge and summary**: Run `git-po-helper agent-run report` to merge all
   `po/review-batch-*.json` into `po/review.json` and display the result. Show
   the command output to the user. Do **not** open or read any JSON files; the
   user will refer to them as needed.

   ```shell
   git-po-helper agent-run report
   ```

**Review JSON format.** Use the following structure:

```json
{
  "issues": [
    {
      "msgid": "commit",
      "msgstr": "承诺",
      "score": 0,
      "description": "术语错误：'commit'应译为'提交'",
      "suggestion": "提交"
    },
    {
      "msgid": "repository",
      "msgstr": "仓库",
      "score": 2,
      "description": "一致性问题：其他地方使用'版本库'",
      "suggestion": "版本库"
    }
  ]
}
```

- `issues`: Array of issues. Each issue: `msgid`, `msgstr`, `score`, `description`, `suggestion`.
- `score`: 0 = critical (must fix before release), 1 = major (should fix), 2 = minor (improve later), 3 = perfect.


## Human translators remain in control

Git translation is human-driven; language team leaders and contributors are
responsible for:

- Understanding technical context of Git commands and messages
- Making linguistic and cultural decisions for the target language
- Maintaining translation quality and consistency
- Ensuring translations follow Git l10n conventions and standards
- Building and maintaining language glossaries
- Reviewing and approving all changes before submission

AI tools, if used, only accelerate routine tasks:

- First-draft translations for new or updated messages
- Finding untranslated or fuzzy entries
- Checking consistency with glossary and existing translations
- Detecting technical errors (placeholders, formatting)
- Reviewing against quality criteria

AI-generated output should always be treated as rough drafts requiring human
review, editing, and approval by someone who understands both the technical
context and the target language.  The best results come from combining AI
efficiency with human judgment, cultural insight, and community engagement.
