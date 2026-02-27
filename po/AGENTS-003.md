# Instructions for AI Agents

This section provides specific instructions for AI agents when handling
translation-related tasks.

We will use XX as an alias to refer to the language translation code in
the following paragraphs, for example we use "po/XX.po" to refer to the
translation file for a specific language. But this doesn't mean that
the language code has only two letters. The language code can be in one
of two forms: "ll" or "ll\_CC". Here "ll" is the ISO 639 two-letter
language code and "CC" is the ISO 3166 two-letter code for country names
and subdivisions. For example: "de" for German language code, "zh\_CN"
for Simplified Chinese language code.


## Generating or updating po/git.pot

When asked to "update po/git.pot" or similar requests:

1. **Directly execute** the command `make po/git.pot` without checking
   if the file exists beforehand.

2. **Do not verify** the generated file after execution. Simply run the
   command and consider the task complete.

The command will handle all necessary steps including file creation or
update automatically.


## Updating po/XX.po

When asked to "update po/XX.po" or similar requests (where XX is a
language code):

1. **Directly execute** the command `make po-update PO_FILE=po/XX.po`
   without reading or checking the file content beforehand.

2. **Do not verify** the updated file after execution. Simply run the
   command and consider the task complete.

The command will handle all necessary steps including generating
"po/git.pot" and merging new translatable strings into "po/XX.po"
automatically.


## Locating untranslated, fuzzy, and obsolete entries

Do not use `grep '^msgstr ""$'` to locate untranslated entries, as this
approach is unreliable. For multi-line `msgstr` entries, the first line is
always empty (`msgstr ""`), which would cause false positives. Use GNU gettext
tools to parse the PO structure reliably instead.

- Untranslated entries:

  ```shell
  msgattrib --untranslated --no-obsolete po/XX.po
  ```

- Fuzzy entries:

  ```shell
  msgattrib --only-fuzzy --no-obsolete po/XX.po
  ```

- Obsolete entries (marked with `#~`):

  ```shell
  msgattrib --obsolete --no-wrap po/XX.po
  ```

If you only want the message IDs, you can pipe to:

```shell
msgattrib --untranslated --no-obsolete po/XX.po | sed -n '/^msgid /,/^$/p'
```

```shell
msgattrib --only-fuzzy --no-obsolete po/XX.po | sed -n '/^msgid /,/^$/p'
```


## Translating po/XX.po

When asked to translate "po/XX.po" or similar requests:

**IMPORTANT: You must complete the translation of ALL untranslated and fuzzy
entries. Do not stop early or report partial progress. Continue iterating
through steps 1-6 until `po/XX.po.pending` is empty (contains no entries).**

1. **Extract entries to translate**: Use `msgattrib` to extract
   untranslated and fuzzy messages separately, then combine them. The
   `--clear-fuzzy --empty` options for fuzzy entries clear their `msgstr`
   values, treating them as untranslated entries:

   ```shell
   msgattrib --untranslated --no-obsolete po/XX.po -o po/XX.po.untranslated
   msgattrib --only-fuzzy --no-obsolete --clear-fuzzy --empty po/XX.po -o po/XX.po.fuzzy
   msgcat --use-first po/XX.po.untranslated po/XX.po.fuzzy -o po/XX.po.pending
   ```

   If `po/XX.po.pending` is empty, skip to step 7 (clean up) as the
   translation is complete.

2. **Check file size and truncate if needed**: **BEFORE translating**, check
   if the `po/XX.po.pending` file is too large. Count the number of entries
   (msgid blocks) in the file. If it contains more than 200 entries, truncate
   it to process in small batches. Set the batch size using variable `NUM`
   (default: 11 entries per batch):

   ```shell
   # Count entries in po/XX.po.pending
   ENTRY_COUNT=$(grep -c '^msgid ' po/XX.po.pending)

   # If more than 200 entries, truncate to NUM entries (default: 11)
   if [ "$ENTRY_COUNT" -gt 200 ]; then
       NUM=11  # Adjust NUM as needed based on file size
       awk -v num="$NUM" '/^msgid / && ++count > num {exit} 1' po/XX.po.pending > po/XX.po.pending.tmp
       mv po/XX.po.pending.tmp po/XX.po.pending
   fi
   ```

3. **Reference glossary**: Read the glossary section from the header of
   `po/XX.po.pending` (if present, before the first `msgid`) and use it for
   consistent terminology during translation.

4. **Translate entries**: Translate entries in `po/XX.po.pending` from English
   (msgid) to the target language (msgstr), and write the translation results
   directly into `po/XX.po.pending`:
   - **Translation approach**: **MANDATORY**: Use a large language model
     (LLM) to translate the `po/XX.po.pending` file as a whole, rather than
     translating entry by entry. **DO NOT use pattern matching, glossary-based
     string replacement, or batch substitution** - these approaches produce
     poor-quality translations. The LLM must understand context and generate
     natural, fluent translations in the target language.
   - **For untranslated entries**: Translate the English `msgid` into the
     target language in `msgstr`.
   - **For fuzzy entries**: Since fuzzy entries have been cleared (empty
     `msgstr`) in step 1, treat them the same as untranslated entries:
     translate the English `msgid` into the target language in `msgstr`.
     The `#, fuzzy` tag will be automatically removed when the entry is
     merged back into `po/XX.po`.
   - **For plural entries**: For entries with `msgid_plural`, provide all
     required `msgstr[n]` forms based on the `Plural-Forms` header in
     `po/XX.po.pending`. The number of plural forms required depends on the
     target language's plural rules.
   - **Placeholder reordering**: When a translation reorders placeholders,
     mark them with positional parameter syntax (`%n$`) so each argument
     maps to the correct source value. Keep the width/precision modifiers
     intact and place the position specifier before them.

     Example: The `msgid` has two placeholders (`%s` and `%.*s`). The
     `%.*s` placeholder requires two arguments: argument 2 (precision value)
     and argument 3 (string). In the translation, the placeholders are
     reordered: `%1$s` refers to argument 1 (the `%s` value), while
     `%3$.*2$s` uses argument 3 (the string from `%.*s`) with precision
     from argument 2 (the precision value from `%.*s`):

     ```po
     msgid "missing environment variable '%s' for configuration '%.*s'"
     msgstr "配置 '%3$.*2$s' 缺少环境变量 '%1$s'"
     ```

5. **Merge translated entries**: Merge the translated `po/XX.po.pending`
   back into `po/XX.po`:

   ```shell
   msgcat --use-first po/XX.po.pending po/XX.po > po/XX.po.merged
   mv po/XX.po.merged po/XX.po
   ```

6. **Repeat until complete**: **MANDATORY**: You MUST repeat steps 1-5
   (extract, truncate if needed, translate, merge) until `po/XX.po.pending`
   is completely empty. Do not report partial progress or stop early. The
   task is only complete when there are zero untranslated and zero fuzzy
   entries remaining in `po/XX.po`.

7. **Clean up**: Only after confirming that `po/XX.po.pending` is empty
   (no untranslated or fuzzy entries remain), remove all temporary files
   (`po/XX.po.pending`, `po/XX.po.untranslated`, `po/XX.po.fuzzy`,
   `msgid_lines.txt` if created). Do not clean up if there are still
   untranslated or fuzzy entries remaining.
