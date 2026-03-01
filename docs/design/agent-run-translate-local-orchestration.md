# Design Document: agent-run translate --use-agent-md and --use-local-orchestration

## 1. Overview

This document describes the design for extending `agent-run translate` with two mutually exclusive modes:

1. **--use-agent-md**: Use the existing translation flow. The agent receives the full PO file (or extracted untranslated/fuzzy entries) and performs translation in one or more rounds. This is the current behavior.

2. **--use-local-orchestration**: Use the new translation flow from [po/AGENTS.md Task 4](https://github.com/git-l10n/git-po/blob/master/po/AGENTS.md). git-po-helper orchestrates the workflow locally using `msg-select` and `msg-cat`; the agent is invoked **only** for translating each batch JSON file. Like `agent-run review`, this mode uses a **separate prompt** maintained in `config/prompts/local-orchestration-translation.md` (distinct from `config/prompts/translate.txt` used by `--use-agent-md`).

## 2. Reference: AGENTS.md Task 4 Translation Flow

The new flow (AGENTS.md lines 452-555) uses:

1. **Condition check**: Determine next step based on existing files
2. **Generate pending file**: `msg-select --untranslated --fuzzy --no-obsolete` → `po/l10n-todo.po`
3. **Generate batch files**: Split `l10n-todo.po` into `po/l10n-todo-batch-<N>.json` via `msg-select --json --range`
4. **Translate each batch**: Agent reads `l10n-todo-batch-<N>.json`, writes `po/l10n-done-batch-<N>.json`
5. **Merge batch results**: `msg-cat -o po/l10n-done.po po/l10n-done-batch-*.json`
6. **Complete translation**: `msgcat --use-first po/l10n-done.po po/XX.po` → merge into target, validate with `msgfmt`

Batch size formula (from AGENTS.md):
- If `ENTRY_COUNT <= min_batch_size*2`: single batch
- If `ENTRY_COUNT > min_batch_size*8`: NUM = min_batch_size*2
- Else if `ENTRY_COUNT > min_batch_size*4`: NUM = min_batch_size + min_batch_size/2
- Else: NUM = min_batch_size

## 3. Command Interface

### 3.1 Flags

```bash
git-po-helper agent-run translate [--use-agent-md | --use-local-orchestration] [--agent <name>] [--batch-size <n>] [po/XX.po]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--use-agent-md` | false | Use existing flow: agent receives full/extracted PO, does translation |
| `--use-local-orchestration` | false | Use new flow: local orchestration, agent only translates batch JSON files |
| `--agent` | (from config) | Agent name when multiple agents configured |
| `--batch-size` | 50 | Min entries per batch (local-orchestration only) |

**Mutual exclusivity**: If both `--use-agent-md` and `--use-local-orchestration` are specified, return error. If neither is specified, default to `--use-agent-md` (preserve current behavior).

### 3.2 Examples

```bash
# Existing flow (default)
git-po-helper agent-run translate po/zh_CN.po

# Explicit existing flow
git-po-helper agent-run translate --use-agent-md po/zh_CN.po

# New flow: local orchestration, agent only for batch translation
git-po-helper agent-run translate --use-local-orchestration po/zh_CN.po

# With batch size
git-po-helper agent-run translate --use-local-orchestration --batch-size 30 po/zh_CN.po
```

## 4. Architecture

### 4.1 Flow Comparison

```
--use-agent-md (existing):
  [git-po-helper] → pre-validation → [Agent: full PO] → post-validation → done

--use-local-orchestration (new):
  [git-po-helper] → msg-select (todo.po) → msg-select (batch JSONs) →
  [Agent: batch-1.json] → [Agent: batch-2.json] → ... →
  [git-po-helper] → msg-cat (merge) → msgcat (into XX.po) → msgfmt (validate) → done
```

### 4.2 Reference: agent-run review Implementation

`agent-run review` provides the blueprint:

- **RunAgentReview** (local orchestration): Uses `config/prompts/review.txt`; `PrepareReviewData` → `prepareReviewBatches` (msg-select PO) → `runReviewBatched` (agent per batch) → `ReportReviewFromPathWithBatches` (merge JSONs)
- **RunAgentReviewUseAgentMd**: Single agent call with dynamically built prompt; agent does extraction, batching, review, and writes `review.json`

For translate local orchestration:
- Use **separate prompt** `config/prompts/local-orchestration-translation.md` (like review uses review.txt)
- Use **JSON batches** (not PO batches) as in AGENTS.md
- Agent receives `l10n-todo-batch-N.json`, writes `l10n-done-batch-N.json`
- Placeholders: `{{.source}}` = input JSON path, `{{.dest}}` = output JSON path (or derive from source)

## 5. Detailed Design

### 5.1 File Naming Convention

For `po/XX.po` (e.g. `po/zh_CN.po`), use base `po/l10n`:

| File | Purpose |
|------|---------|
| `po/l10n-todo.po` | Extracted untranslated + fuzzy entries |
| `po/l10n-todo-batch-<N>.json` | Batch N to translate (input to agent) |
| `po/l10n-done-batch-<N>.json` | Batch N translated (output from agent) |
| `po/l10n-done.po` | Merged translated entries |

### 5.2 RunAgentTranslateUseAgentMd (existing flow)

- Rename or alias current `RunAgentTranslate` logic as the "use-agent-md" path
- No structural change; just ensure it is invoked when `--use-agent-md` is set or when both flags are absent (default)

### 5.3 RunAgentTranslateLocalOrchestration (new flow)

Implement steps matching AGENTS.md Task 4:

**Step 1: Condition check**

- If `po/l10n-todo.po` does not exist → go to Step 2
- If `po/l10n-todo-batch-*.json` exist → go to Step 4 (translate batches)
- If `po/l10n-done-batch-*.json` exist (and no todo-batch) → go to Step 5 (merge)
- Otherwise → go to Step 3 (generate batches)

**Step 2: Generate pending file**

```go
// Remove any stale batch files
os.Remove("po/l10n-todo-batch-*.json")
os.Remove("po/l10n-done-batch-*.json")
// msg-select --untranslated --fuzzy --no-obsolete po/XX.po -o po/l10n-todo.po
MsgSelect(poFile, "", todoFile, false, &EntryStateFilter{Untranslated: true, Fuzzy: true, NoObsolete: true})
```

If `l10n-todo.po` is empty or has no content entries → translation complete; cleanup and return success.

**Step 3: Generate batch files**

- Count entries in `l10n-todo.po` (excluding header)
- Apply batch size formula to get `num` per batch
- For each batch: `msg-select --json --range "X-Y" po/l10n-todo.po -o po/l10n-todo-batch-<N>.json`
- Use `WriteGettextJSONFromPOFile` or equivalent to produce JSON batches

**Step 4: Translate each batch**

For each `po/l10n-todo-batch-<N>.json` (sorted by N):

- Build agent command with placeholders:
  - `{{.prompt}}`: from `prompt.local_orchestration_translation` (config/prompts/local-orchestration-translation.md)
  - `{{.source}}`: `po/l10n-todo-batch-<N>.json`
  - `{{.dest}}`: `po/l10n-done-batch-<N>.json`
- Execute agent
- Agent is expected to write translated JSON to `{{.dest}}`
- Validate output JSON exists and is parseable
- Delete `po/l10n-todo-batch-<N>.json`
- Repeat until no `l10n-todo-batch-*.json` remain

**Step 5: Merge batch results**

```go
// msg-cat -o po/l10n-done.po po/l10n-done-batch-*.json
MsgCat(glob("po/l10n-done-batch-*.json"), "po/l10n-done.po")
// Remove batch files
os.Remove("po/l10n-done-batch-*.json")
```

**Step 6: Complete translation**

```go
// msgcat --use-first po/l10n-done.po po/XX.po > po/l10n-merged.po
exec.Command("msgcat", "--use-first", "po/l10n-done.po", poFile).Output() → l10n-merged.po
// msgfmt --check -o /dev/null po/l10n-merged.po
exec.Command("msgfmt", "--check", "-o", "/dev/null", "po/l10n-merged.po")
// mv po/l10n-merged.po po/XX.po
os.Rename("po/l10n-merged.po", poFile)
// Cleanup
os.Remove("po/l10n-done.po")
os.Remove("po/l10n-todo.po")
```

Loop back to Step 1; Step 2 will regenerate `l10n-todo.po` from updated `po/XX.po`. If empty, translation is complete.

### 5.4 Prompt and Placeholders for Local Orchestration

**Separate prompt file**: Like `agent-run review`, when using programmatic/local orchestration, git-po-helper uses a **separate prompt** maintained in `config/prompts/local-orchestration-translation.md`. This is distinct from `config/prompts/translate.txt`, which is used only for the `--use-agent-md` flow.

The local-orchestration prompt should instruct the agent to:
- Read the gettext JSON from `{{.source}}`
- Translate each entry (msgid → msgstr, msgstr_plural for plurals)
- Write the translated gettext JSON to `{{.dest}}`

Config key: `prompt.local_orchestration_translation` (embedded from `config/prompts/local-orchestration-translation.md`).

Placeholders:
- `{{.prompt}}`: resolved prompt content
- `{{.source}}`: `po/l10n-todo-batch-<N>.json`
- `{{.dest}}`: `po/l10n-done-batch-<N>.json`

Config example (git-po-helper.yaml):

```yaml
prompt:
  translate: "Translate file {{.source}} according to @po/AGENTS.md."
  local_orchestration_translation: "..."  # loaded from config/prompts/local-orchestration-translation.md
```

The `local_orchestration_translation` prompt is loaded from the embedded file; users may override it in their config.

### 5.5 Resume Support

Similar to `agent-run review`:
- If `l10n-todo-batch-*.json` exist: resume from Step 4 (translate remaining batches)
- If only `l10n-done-batch-*.json` exist: run Step 5 (merge) then Step 6

### 5.6 Agent Output Handling

For local orchestration, the agent writes directly to `{{.dest}}`. git-po-helper does **not** parse stdout for JSON; it only checks that the output file exists and is valid gettext JSON. If the agent uses streaming JSON output, we may need a variant that writes to file (similar to review's `output` config).

## 6. Implementation Plan

### 6.1 Files to Modify/Create

| File | Changes |
|------|---------|
| `cmd/agent-run-translate.go` | Add `--use-agent-md`, `--use-local-orchestration`, `--batch-size`; dispatch to appropriate flow |
| `util/agent-run-translate.go` | Add `RunAgentTranslateLocalOrchestration`; refactor `RunAgentTranslate` as use-agent-md path |
| `util/agent-run-translate-local.go` | **New**: Local orchestration logic (steps 1–6, batch creation, merge, msgcat/msgfmt) |
| `config/prompts/local-orchestration-translation.md` | **New**: Separate prompt for local orchestration batch translation |
| `config/agent.go` | Add `local_orchestration_translation` prompt key and embed |

### 6.2 Development Steps (each step = one commit)

| Step | Commit | Description |
|------|--------|-------------|
| 1 | Add flags to translate command | Add `--use-agent-md`, `--use-local-orchestration`, `--batch-size` to `cmd/agent-run-translate.go`; implement mutual exclusivity and default to `--use-agent-md` |
| 2 | Add local-orchestration prompt | Create `config/prompts/local-orchestration-translation.md` with batch translation instructions; add `local_orchestration_translation` to `config/agent.go` (PromptConfig, embed, default); wire `GetRawPrompt` for action `local-orchestration-translation` |
| 3 | Create local orchestration module | Create `util/agent-run-translate-local.go` with `RunAgentTranslateLocalOrchestration` skeleton; implement Steps 1–6 (condition check, msg-select todo, batch JSON creation, agent per batch, msg-cat merge, msgcat+msgfmt) |
| 4 | Wire translate command to local flow | In `cmd/agent-run-translate.go`, when `--use-local-orchestration` is set, call `RunAgentTranslateLocalOrchestration`; ensure `--use-agent-md` (or default) calls existing `RunAgentTranslate` |
| 5 | Add integration tests | Add integration test for `agent-run translate --use-local-orchestration` (e.g. in `test/t0090-agent-run.sh` or new test file); verify batch flow, merge, and final PO |
| 6 | Update documentation | Update README, `docs/agent-commands.md` (or equivalent) with `--use-local-orchestration` usage and `config/prompts/local-orchestration-translation.md` reference |

### 6.3 Dependencies

- `util.MsgSelect` with `EntryStateFilter{Untranslated: true, Fuzzy: true, NoObsolete: true}` for Step 2
- `util.WriteGettextJSONFromPOFile` for batch JSON output (Step 3)
- `util.ReadFileToGettextJSON` + `util.MergeGettextJSON` + `util.WriteGettextJSONToPO` for Step 5 (or invoke `msg-cat` subprocess)
- External: `msgcat`, `msgfmt` (already required by project)
- `BuildAgentCommand` with `source` and `dest` placeholders

## 7. Testing

- Unit tests: batch creation logic, batch size formula, file naming
- Integration test: run `agent-run translate --use-local-orchestration` with a test agent (e.g. echo/copy) that copies JSON; verify merge and final PO
- Ensure `--use-agent-md` preserves existing behavior (no regression)

## 8. Open Questions

1. **Agent output**: If agent uses streaming JSON, should we support writing to file via config (like review)?
2. **Batch size default**: 50 (per AGENTS.md) or configurable only via `--batch-size`?
