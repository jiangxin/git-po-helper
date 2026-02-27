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
through steps 1-8 until `po/XX.po.pending` is empty (contains no entries).**

**NOTE: Translation may take a long time for large files. You can safely
interrupt the process at the following points:**
- After completing a batch (after step 7 completes)
- After validation passes (after step 5 completes)

**To resume after interruption**: Re-run the translation command. The system
will continue from the last progress checkpoint.

1. **Extract entries to translate and record initial state**: Use `msgattrib`
   to extract untranslated and fuzzy messages separately, then combine them.
   Also record the initial total count for progress tracking. For fuzzy
   entries, extract them first with their original translations as reference
   (saved to a separate file), then clear their `msgstr` values:

   ```shell
   # Extract untranslated entries
   msgattrib --untranslated --no-obsolete po/XX.po -o po/XX.po.untranslated

   # Extract fuzzy entries with original translations as reference
   msgattrib --only-fuzzy --no-obsolete po/XX.po -o po/XX.po.fuzzy.reference

   # Extract fuzzy entries with cleared msgstr for translation
   msgattrib --only-fuzzy --no-obsolete --clear-fuzzy --empty po/XX.po -o po/XX.po.fuzzy

   # Combine untranslated and fuzzy entries
   msgcat --use-first po/XX.po.untranslated po/XX.po.fuzzy -o po/XX.po.pending

   # Record initial total count for progress tracking (first time only)
   if test ! -f po/XX.po.total
   then
       UNTRANS_INIT=$(msgattrib --untranslated --no-obsolete po/XX.po 2>/dev/null | grep -c '^msgid ' || true)
       UNTRANS_INIT=$((UNTRANS_INIT > 0 ? UNTRANS_INIT - 1 : 0))
       FUZZY_INIT=$(msgattrib --only-fuzzy --no-obsolete po/XX.po 2>/dev/null | grep -c '^msgid ' || true)
       FUZZY_INIT=$((FUZZY_INIT > 0 ? FUZZY_INIT - 1 : 0))
       echo $((UNTRANS_INIT + FUZZY_INIT)) > po/XX.po.total
   fi
   ```

   If `po/XX.po.pending` is empty, skip to step 9 (clean up) as the
   translation is complete.

   **Note**: The `po/XX.po.fuzzy.reference` file contains fuzzy entries with
   their original translations. You can reference these during translation to
   preserve good translations or understand context, but always provide fresh
   translations in `msgstr` fields.

2. **Check file size and truncate with dynamic batch size**: **BEFORE
   translating**, check if the `po/XX.po.pending` file is too large. Count
   the number of entries (msgid blocks) in the file. If it contains more than
   100 entries, truncate it to process in batches. Use dynamic batch sizing
   based on remaining entry count for optimal efficiency:

   ```shell
   # Count entries in po/XX.po.pending
   ENTRY_COUNT=$(grep -c '^msgid ' po/XX.po.pending 2>/dev/null || true)
   ENTRY_COUNT=$((ENTRY_COUNT > 0 ? ENTRY_COUNT - 1 : 0))

   # Dynamic batch size selection based on remaining entries
   if test "$ENTRY_COUNT" -gt 100
   then
       # Dynamic batch size:
       # - Very large files (>500 entries): NUM=100 (larger batches for efficiency)
       # - Large files (200-500 entries): NUM=75 (medium batches)
       # - Medium files (100-200 entries): NUM=50 (default batch size)
       # - Small files (<100 entries): process all at once (skip truncation)
       if test "$ENTRY_COUNT" -gt 500
       then
           NUM=100
       elif test "$ENTRY_COUNT" -gt 200
       then
           NUM=75
       else
           NUM=50
       fi
       # Use "count++" because we extract additional header entry
       awk -v num="$NUM" '/^msgid / && count++ > num {exit} 1' po/XX.po.pending > po/XX.po.pending.tmp
       mv po/XX.po.pending.tmp po/XX.po.pending
       echo "Processing batch of $NUM entries (out of $ENTRY_COUNT remaining)"
   else
       echo "Processing all $ENTRY_COUNT entries at once"
   fi
   ```

3. **Reference glossary**: Read the glossary section from the header of
   `po/XX.po.pending` (if present, before the first `msgid`) and use it for
   consistent terminology during translation.

4. **Translate entries**: Translate entries in `po/XX.po.pending` from English
   (msgid) to the target language (msgstr), and write the translation results
   directly into `po/XX.po.pending`. Optionally reference `po/XX.po.fuzzy.reference`
   for fuzzy entries that had previous translations:

   - **Translation approach**: **MANDATORY**: Use a large language model
     (LLM) to translate the `po/XX.po.pending` file. The LLM must:
     * Read and understand the complete PO file format, including all
       structural elements (comments, flags, msgid, msgstr, etc.)
     * Preserve ALL formatting exactly as-is: quotes, line breaks, escape
       sequences, multi-line structures
     * Only modify the `msgstr` field content, keeping all format markers
       unchanged
     * Understand context from surrounding entries and glossary
     * Generate natural, fluent translations in the target language
     * For fuzzy entries: Optionally reference `po/XX.po.fuzzy.reference` to
       understand previous translation context, but always provide fresh,
       accurate translations

     **CRITICAL**: **DO NOT use pattern matching, regular expressions, string
     replacement, or batch substitution** - these approaches will break PO file
     format, especially for multi-line entries. The LLM must work with the file
     as a structured document, not as plain text.

   - **Batch optimization**: For efficiency, you can process simple entries
     (single-line, no placeholders, no special formatting) in larger batches,
     while complex entries (multi-line, with placeholders, formatted) should
     be handled more carefully with smaller batches or individually.

   - **For single-line entries**: These have a simple format:
     ```po
     msgid "English text"
     msgstr ""
     ```
     Translate by filling in the `msgstr` field while keeping the exact format.

   - **For multi-line entries**: These require special care:
     ```po
     msgid ""
     "Line 1\n"
     "Line 2"
     msgstr ""
     ```
     When translating:
     * Keep the `msgid ""` format
     * Keep all quote marks and line structure
     * Keep `\n` escape sequences within quotes
     * Only modify the content inside quotes after `msgstr ""`
     * Maintain the same multi-line structure in the translation

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

5. **Validate and merge translated entries**: **OPTIMIZED**: Combine validation
   and merging in a single step for better performance. Validate the PO file
   format while merging - if validation fails, the merge will also fail and
   the original file remains unchanged:

   ```shell
   # Validate and merge in one step (validation happens during merge)
   if msgcat --use-first po/XX.po.pending po/XX.po > po/XX.po.merged 2>&1
   then
       # Validation and merge successful
       mv po/XX.po.merged po/XX.po
       echo "Batch merged successfully"
   else
       # Validation or merge failed
       echo "ERROR: PO file format is invalid or merge failed."
       echo "Do not proceed. Re-extract and retry translation."
       rm -f po/XX.po.merged
       # Optionally, re-extract the pending file:
       # rm -f po/XX.po.pending po/XX.po.untranslated po/XX.po.fuzzy
       # (Then repeat from step 1)
       exit 1
   fi
   ```

   If validation/merge fails:
   - **DO NOT** attempt to use the corrupted merged file
   - **DO NOT** continue modifying the file
   - Re-extract `po/XX.po.pending` by repeating step 1
   - Re-translate the entries
   - Re-validate and merge before proceeding

6. **Save progress checkpoint**: After successful merge, save progress state
   to enable recovery from interruptions:

   ```shell
   # Save progress checkpoint
   BATCH_NUM=$(cat po/XX.po.batch 2>/dev/null || echo 0)
   BATCH_NUM=$((BATCH_NUM + 1))
   echo $BATCH_NUM > po/XX.po.batch
   ```

7. **Report detailed progress and repeat**: After merging, report detailed
   progress including percentage, remaining batches estimate, and repeat the
   process:

   ```shell
   # Get current remaining counts
   UNTRANS=$(msgattrib --untranslated --no-obsolete po/XX.po 2>/dev/null | grep -c '^msgid ' || true)
   UNTRANS=$((UNTRANS > 0 ? UNTRANS - 1 : 0))
   FUZZY=$(msgattrib --only-fuzzy --no-obsolete po/XX.po 2>/dev/null | grep -c '^msgid ' || true)
   FUZZY=$((FUZZY > 0 ? FUZZY - 1 : 0))
   REMAINING=$((UNTRANS + FUZZY))

   # Get initial total (from step 1)
   TOTAL_ENTRIES=$(cat po/XX.po.total 2>/dev/null || echo 0)
   if test "$TOTAL_ENTRIES" -eq 0
   then
       TOTAL_ENTRIES=$REMAINING
       echo $TOTAL_ENTRIES > po/XX.po.total
   fi

   # Calculate progress
   PROCESSED=$((TOTAL_ENTRIES - REMAINING))
   if test "$TOTAL_ENTRIES" -gt 0
   then
       PROGRESS=$((100 * PROCESSED / TOTAL_ENTRIES))
   else
       PROGRESS=100
   fi

   # Estimate remaining batches (using average batch size)
   BATCH_NUM=$(cat po/XX.po.batch 2>/dev/null || echo 0)
   if test "$BATCH_NUM" -gt 0 && test "$REMAINING" -gt 0
   then
       AVG_BATCH=$((PROCESSED / BATCH_NUM))
       if test "$AVG_BATCH" -gt 0
       then
           EST_BATCHES=$((REMAINING / AVG_BATCH + 1))
       else
           EST_BATCHES="?"
       fi
   else
       EST_BATCHES="?"
   fi

   # Report detailed progress
   echo "========================================="
   echo "Translation Progress Report"
   echo "========================================="
   echo "Progress: $PROGRESS% ($PROCESSED/$TOTAL_ENTRIES entries processed)"
   echo "Remaining: $UNTRANS untranslated + $FUZZY fuzzy = $REMAINING total"
   echo "Batches completed: $BATCH_NUM"
   if test "$EST_BATCHES" != "?"
   then
       echo "Estimated remaining batches: ~$EST_BATCHES"
   fi
   echo "========================================="
   ```

   **MANDATORY**: You MUST repeat steps 1-7 (extract, truncate if needed,
   translate, validate/merge, save checkpoint, report) until `po/XX.po.pending`
   is completely empty. Do not report partial progress or stop early. The
   task is only complete when there are zero untranslated and zero fuzzy
   entries remaining in `po/XX.po`.

8. **Final verification**: Before cleanup, perform a final verification to
   ensure all entries are translated:

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

9. **Clean up**: Only after confirming that translation is complete (step 8
   passes), remove all temporary files and progress tracking files:

   ```shell
   # Remove temporary files
   rm -f po/XX.po.pending
   rm -f po/XX.po.untranslated
   rm -f po/XX.po.fuzzy
   rm -f po/XX.po.fuzzy.reference
   rm -f po/XX.po.total
   rm -f po/XX.po.batch
   rm -f po/XX.po.merged
   rm -f msgid_lines.txt  # if created
   echo "Cleanup complete. Translation finished successfully."
   ```


## Error Handling and Recovery

When translating `po/XX.po`, you may encounter errors. Follow these
recovery procedures:


### Format Validation Errors

If `msgcat` reports format errors (e.g., "end-of-line within string"):

1. **Stop immediately**: Do not attempt to fix the corrupted file
2. **Re-extract**: Remove the corrupted `po/XX.po.pending` and re-extract:
   ```shell
   rm -f po/XX.po.pending po/XX.po.untranslated po/XX.po.fuzzy
   # Then repeat step 1 of the translation process
   ```
3. **Re-translate**: Start fresh with the newly extracted file
4. **Validate early**: Validate format immediately after translation


### Merge Failures

If merging fails:

1. **Check format**: Validate `po/XX.po.pending` format first
2. **Check source**: Ensure `po/XX.po` is not corrupted
3. **Re-extract if needed**: If issues persist, re-extract both files


### Common Format Issues

- **Multi-line entries**: Ensure all quote marks are properly closed
- **Escape sequences**: Keep `\n`, `\"`, etc. within quotes
- **Line breaks**: Multi-line strings must have each line in quotes
- **Empty msgstr**: Use `msgstr ""` for empty translations (not `msgstr`)


### Best Practices

- Always validate format before merging (now combined with merge step)
- Work with clean extracted files
- Avoid manual text editing of PO files
- Use GNU gettext tools (`msgattrib`, `msgcat`) for all operations
- If using scripts, prefer PO file libraries (e.g., Python's `polib`) over
  regular expressions
- Use dynamic batch sizing for optimal efficiency
- Reference fuzzy translations when available, but always provide fresh
  translations
- Save progress checkpoints to enable recovery from interruptions
- Report detailed progress to keep users informed


### Performance Tips

- For very large files (>500 entries), use larger batch sizes (100 entries)
- For medium files (200-500 entries), use medium batch sizes (75 entries)
- For smaller files (<200 entries), use default batch size (50 entries)
- Combine validation and merge steps to reduce file I/O operations
- Process simple entries (single-line, no placeholders) in larger batches
- Handle complex entries (multi-line, with placeholders) more carefully
