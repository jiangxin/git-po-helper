# Agent Commands Documentation

The `agent-run` and `agent-test` commands allow you to integrate code agents (like Claude, Gemini, etc.) into the git-po-helper workflow for automating localization tasks.

## Overview

These commands use configured code agents to automate various localization operations:

- **agent-run**: Execute agent commands for automation
- **agent-test**: Test agent commands with multiple runs and calculate average scores

## Configuration

Both commands read configuration from `git-po-helper.yaml` files. The configuration can be placed in:

1. **User home directory**: `~/.git-po-helper.yaml` (lower priority)
2. **Repository root**: `<repo-root>/git-po-helper.yaml` (higher priority, overrides user config)

The repository config takes precedence over the user config when both exist.

### Configuration File Format

```yaml
default_lang_code: "zh_CN"
prompt:
  update_pot: "update po/git.pot according to po/README.md"
  update_po: "update {source} according to po/README.md"
  translate: "translate {source} according to po/README.md"
  review: "review and improve {source} according to po/README.md"
agent-test:
  runs: 5
  pot_entries_before_update: null
  pot_entries_after_update: null
  po_entries_before_update: null
  po_entries_after_update: null
  po_new_entries_after_update: null
  po_fuzzy_entries_after_update: null
agents:
  claude:
    cmd: ["claude", "-p", "{prompt}"]
  gemini:
    cmd: ["gemini", "--prompt", "{prompt}"]
```

### Configuration Fields

#### Prompt Templates

- `prompt.update_pot`: Prompt for updating the POT file
- `prompt.update_po`: Prompt for updating a PO file (uses `{source}` placeholder)
- `prompt.translate`: Prompt for translating a PO file (uses `{source}` placeholder)
- `prompt.review`: Prompt for reviewing translations in a PO file (uses `{source}` placeholder)

#### Agent Test Configuration

- `agent-test.runs`: Default number of runs for `agent-test` (default: 5)
- `agent-test.pot_entries_before_update`: Expected POT entry count before update (null or 0 to disable)
- `agent-test.pot_entries_after_update`: Expected POT entry count after update (null or 0 to disable)
- `agent-test.po_entries_before_update`: Expected PO entry count before update (used by update-po)
- `agent-test.po_entries_after_update`: Expected PO entry count after update (used by update-po)
- `agent-test.po_new_entries_after_update`: Expected new PO entries after update (for future use)
- `agent-test.po_fuzzy_entries_after_update`: Expected fuzzy PO entries after update (for future use)

#### Agents

Each agent is defined with a name and a command. The command is a list of strings where placeholders are replaced:

- `{prompt}`: Replaced with the actual prompt text
- `{source}`: Replaced with the source file path (PO file)
- `{commit}`: Replaced with the commit ID (default: HEAD)

## Commands

## Commands

### agent-run update-pot

Update the `po/git.pot` template file using a configured agent.

**Usage:**
```bash
git-po-helper agent-run update-pot [--agent <agent-name>]
```

**Options:**
- `--agent <agent-name>`: Specify which agent to use (required if multiple agents are configured)

**Examples:**
```bash
# Use the default agent (if only one is configured)
git-po-helper agent-run update-pot

# Use a specific agent
git-po-helper agent-run update-pot --agent claude
```

**What it does:**
1. Loads configuration from `git-po-helper.yaml`
2. Selects an agent (auto-selects if only one, or uses `--agent` flag)
3. Performs pre-validation (if `pot_entries_before_update` is configured):
   - Counts entries in `po/git.pot`
   - Verifies count matches expected value
4. Executes the agent command with the configured prompt
5. Performs post-validation (if `pot_entries_after_update` is configured):
   - Counts entries in `po/git.pot`
   - Verifies count matches expected value
6. Validates POT file syntax using `msgfmt`

**Success Criteria:**
- Agent command exits with code 0
- `po/git.pot` file exists and is valid
- Pre-validation passes (if configured)
- Post-validation passes (if configured)

### agent-run update-po

Update a specific `po/XX.po` file using a configured agent.

**Usage:**
```bash
git-po-helper agent-run update-po [--agent <agent-name>] [po/XX.po]
```

**Options:**
- `--agent <agent-name>`: Specify which agent to use (required if multiple agents are configured)
- `po/XX.po`: Optional PO file path; if omitted, `default_lang_code` is used (e.g., `zh_CN` → `po/zh_CN.po`)

**Examples:**
```bash
# Use default_lang_code to locate PO file
git-po-helper agent-run update-po

# Explicitly specify the PO file
git-po-helper agent-run update-po po/zh_CN.po

# Use a specific agent
git-po-helper agent-run update-po --agent claude po/zh_CN.po
```

**What it does:**
1. Loads configuration from `git-po-helper.yaml`
2. Determines target PO file from CLI argument or `default_lang_code`
3. Selects an agent (auto-selects if only one, or uses `--agent` flag)
4. Performs pre-validation (if `po_entries_before_update` is configured):
   - Counts entries in the target `po/XX.po`
   - Verifies count matches expected value
5. Executes the agent command with the `prompt.update_po` template and `{source}` pointing to the PO file
6. Performs post-validation (if `po_entries_after_update` is configured):
   - Counts entries in the target `po/XX.po`
   - Verifies count matches expected value
7. Validates PO file syntax using `msgfmt`

**Success Criteria:**
- Agent command exits with code 0
- Target `po/XX.po` file exists and is valid
- Pre-validation passes (if configured)
- Post-validation passes (if configured)

### agent-run review

Review translations in a PO file using a configured agent. The agent reviews translations and generates a JSON report with issues and scores.

**Usage:**
```bash
git-po-helper agent-run review [--agent <agent-name>] [--commit <commit>] [--since <commit>] [po/XX.po]
```

**Options:**
- `--agent <agent-name>`: Specify which agent to use (required if multiple agents are configured)
- `--commit <commit>`: Review changes in the specified commit
- `--since <commit>`: Review changes since the specified commit
- `po/XX.po`: Optional PO file path; if omitted, `default_lang_code` is used (e.g., `zh_CN` → `po/zh_CN.po`)

**Note:** Exactly one of `--commit` or `--since` may be specified. If neither is provided, the command defaults to reviewing local changes (since HEAD).

**Examples:**
```bash
# Review local changes using default_lang_code
git-po-helper agent-run review

# Review local changes for a specific PO file
git-po-helper agent-run review po/zh_CN.po

# Review changes in a specific commit
git-po-helper agent-run review --commit abc123 po/zh_CN.po

# Review changes since a specific commit
git-po-helper agent-run review --since def456 po/zh_CN.po

# Use a specific agent
git-po-helper agent-run review --agent claude po/zh_CN.po
```

**What it does:**
1. Loads configuration from `git-po-helper.yaml`
2. Determines target PO file from CLI argument or `default_lang_code`
3. Selects an agent (auto-selects if only one, or uses `--agent` flag)
4. Determines review mode from `--range`, `--commit`, or `--since` (uses `prompt.review` with `{source}` placeholder)
5. Executes the agent command with the appropriate prompt template
6. Extracts JSON from agent output
7. Parses and validates the review JSON structure
8. Saves review JSON to `po/XX-reviewed.json`
9. Calculates review score (0-100) based on issues found

**Review JSON Format:**
The agent must output a JSON object with the following structure:
```json
{
  "total_entries": 2592,
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

**Scoring Model:**
- Each entry has a maximum of 3 points
- Critical issues (must fix) = 0 points
- Minor issues (needs adjustment) = 2 points
- Perfect entries = 3 points
- Final score = (total_score / (total_entries * 3)) * 100

**Success Criteria:**
- Agent command exits with code 0
- Agent output contains valid JSON matching `ReviewJSONResult` structure
- JSON file is successfully saved to `po/XX-reviewed.json`
- PO file exists and is valid

**Output:**
The command displays:
- Review score (0-100)
- Total entries reviewed
- Number of issues found (broken down by score: critical, minor, perfect)
- Path to saved JSON file

### agent-test update-pot

Test the `update-pot` operation multiple times and calculate an average score.

**Usage:**
```bash
git-po-helper agent-test update-pot [--agent <agent-name>] [--runs <n>]
```

**Options:**
- `--agent <agent-name>`: Specify which agent to use (required if multiple agents are configured)
- `--runs <n>`: Number of test runs (default: 5, or from config file)

**Examples:**
```bash
# Run 5 tests with default agent
git-po-helper agent-test update-pot

# Run 10 tests with a specific agent
git-po-helper agent-test update-pot --agent claude --runs 10
```

**What it does:**
1. Loads configuration from `git-po-helper.yaml`
2. Determines number of runs (from `--runs` flag, config file, or default to 5)
3. For each run:
   - Performs pre-validation (if configured)
   - Executes agent command (if pre-validation passed)
   - Performs post-validation (if configured)
   - Scores the run (100 for success, 0 for failure)
4. Calculates average score across all runs
5. Displays detailed results including validation status

**Scoring:**
- **If validation is configured**: Score based on validation results
  - Pre-validation failure: Score = 0 (agent not executed)
  - Post-validation failure: Score = 0 (even if agent succeeded)
  - Both validations pass: Score = 100
- **If validation is not configured**: Score based on agent exit code
  - Agent succeeds (exit code 0): Score = 100
  - Agent fails (non-zero exit code): Score = 0

**Output:**
The command displays:
- Individual run results with validation status
- Success/failure counts
- Average score
- Entry count validation results (if configured)

### agent-test update-po

Test the `update-po` operation multiple times and calculate an average score.

**Usage:**
```bash
git-po-helper agent-test update-po [--agent <agent-name>] [--runs <n>] [po/XX.po]
```

**Options:**
- `--agent <agent-name>`: Specify which agent to use (required if multiple agents are configured)
- `--runs <n>`: Number of test runs (default: 5, or from config file)
- `po/XX.po`: Optional PO file path; if omitted, `default_lang_code` is used (e.g., `zh_CN` → `po/zh_CN.po`)

**Examples:**
```bash
# Run tests using default_lang_code to locate PO file
git-po-helper agent-test update-po

# Run tests for a specific PO file
git-po-helper agent-test update-po po/zh_CN.po

# Run 10 tests with a specific agent and PO file
git-po-helper agent-test update-po --agent claude --runs 10 po/zh_CN.po
```

**What it does:**
1. Loads configuration from `git-po-helper.yaml`
2. Determines number of runs (from `--runs` flag, config file, or default to 5)
3. For each run:
   - Restores `po/` directory to `HEAD` for a clean state
   - Calls `agent-run update-po` logic via `RunAgentUpdatePo`
   - Applies PO entry-count validation (if `po_entries_before_update` / `po_entries_after_update` are configured)
   - Scores the run (100 for success, 0 for failure)
4. Calculates average score across all runs
5. Displays detailed results including validation status and entry counts

**Scoring:**
- **With validation enabled**: Score is 0 if any enabled validation fails, otherwise 100
- **With validation disabled**: Score is 100 if agent command and PO syntax validation succeed, otherwise 0

**Output:**
The command displays:
- Individual run results with validation and agent execution status
- Success/failure counts
- Average score
- Entry count validation results (if configured)

### agent-test review

Test the `review` operation multiple times and calculate an average score.

**Usage:**
```bash
git-po-helper agent-test review [--agent <agent-name>] [--runs <n>] [--commit <commit>] [--since <commit>] [po/XX.po]
```

**Options:**
- `--agent <agent-name>`: Specify which agent to use (required if multiple agents are configured)
- `--runs <n>`: Number of test runs (default: 5, or from config file)
- `--commit <commit>`: Review changes in the specified commit
- `--since <commit>`: Review changes since the specified commit
- `po/XX.po`: Optional PO file path; if omitted, `default_lang_code` is used (e.g., `zh_CN` → `po/zh_CN.po`)

**Note:** Exactly one of `--commit` or `--since` may be specified. If neither is provided, the command defaults to reviewing local changes (since HEAD).

**Examples:**
```bash
# Run 5 tests using default_lang_code to locate PO file
git-po-helper agent-test review

# Run 5 tests for a specific PO file
git-po-helper agent-test review po/zh_CN.po

# Run 10 tests with a specific agent
git-po-helper agent-test review --agent claude --runs 10 po/zh_CN.po

# Run tests reviewing changes since a specific commit
git-po-helper agent-test review --since abc123 po/zh_CN.po
```

**What it does:**
1. Loads configuration from `git-po-helper.yaml`
2. Determines number of runs (from `--runs` flag, config file, or default to 5)
3. For each run:
   - Calls `agent-run review` logic
   - Saves results to `output/<agent-name>/<iteration-number>/`:
     - `XX-reviewed.po`: The reviewed PO file
     - `review.log`: Execution log (stdout + stderr)
     - `XX-reviewed.json`: Review JSON result
   - Parses JSON and calculates score using `CalculateReviewScore()`
   - Records score for this run
4. Calculates average score: `(sum of scores) / number of runs`
5. Displays results:
   - Individual run results (including scores and issue counts)
   - Average score
   - Summary statistics (success count, failure count)

**Scoring:**
- Each run produces a JSON file with review results
- Score is calculated from JSON using `CalculateReviewScore()`
- Success = score > 0 (agent executed and produced valid JSON)
- Failure = score = 0 (agent failed or produced invalid JSON)
- Average = sum of scores / number of runs

**Output:**
The command displays:
- Individual run results with status (PASS/FAIL) and score
- Agent execution status and review status
- Summary statistics (total runs, successful runs, failed runs, average score)
- Results are saved to `output/<agent-name>/<iteration-number>/` for later review

## Entry Count Validation

Entry count validation is a critical feature for ensuring agents update files correctly. Validation can be enabled or disabled per stage.

### Validation Rules

1. **Null or Zero Values**: If a validation field is `null` or `0`, validation is **disabled** for that stage.

2. **Non-Zero Values**: If a validation field has a non-zero value, validation is **enabled** and the system will:
   - Count entries in `po/git.pot` at the specified stage
   - Compare the actual count with the expected value
   - Mark the operation as failed (score = 0) if counts don't match
   - Mark the operation as successful (score = 100) if counts match

### Pre-Validation (Before Agent Execution)

**When**: `pot_entries_before_update` is configured (not null and not 0)

**Process**:
1. Count entries in `po/git.pot` before agent execution
2. Compare with `pot_entries_before_update`
3. If mismatch: Return error immediately, do not execute agent (score = 0)
4. If match: Continue to agent execution

**Use Case**: Ensures the POT file is in the expected state before the agent runs.

### Post-Validation (After Agent Execution)

**When**: `pot_entries_after_update` is configured (not null and not 0)

**Process**:
1. Execute agent command (if pre-validation passed or was disabled)
2. Count entries in `po/git.pot` after agent execution
3. Compare with `pot_entries_after_update`
4. If mismatch: Mark as failed (score = 0)
5. If match: Mark as successful (score = 100)

**Use Case**: Verifies that the agent correctly updated the POT file with the expected number of entries.

### Example Scenarios

**Scenario 1: Both validations enabled**
```yaml
agent-test:
  pot_entries_before_update: 5000
  pot_entries_after_update: 5100
```
- Before agent: Verify 5000 entries (fail if not)
- After agent: Verify 5100 entries (fail if not)
- Success only if both match

**Scenario 2: Only post-validation enabled**
```yaml
agent-test:
  pot_entries_before_update: null
  pot_entries_after_update: 5100
```
- Before agent: No validation
- After agent: Verify 5100 entries (fail if not)

**Scenario 3: Validation disabled**
```yaml
agent-test:
  pot_entries_before_update: null
  pot_entries_after_update: null
```
- No entry count validation
- Scoring based on agent exit code only

## Error Handling

All commands provide clear error messages with actionable hints:

- **Configuration errors**: Include file location hints
- **Agent selection errors**: List available agents
- **Validation errors**: Show expected vs actual values
- **File operation errors**: Include file paths and suggestions
- **Command execution errors**: Include exit codes and stderr output

## Logging

The commands use structured logging with different levels:

- **Debug logs**: Detailed information for troubleshooting (use `-v` flag)
- **Info logs**: Important operations and success messages
- **Error logs**: Error information with context
- **Warning logs**: Non-fatal issues (e.g., syntax validation failures)

Use the `-v` (verbose) flag to see debug logs, or `-q` (quiet) flag to suppress non-error messages.

## Examples

### Basic Setup

1. Create `git-po-helper.yaml` in your repository root:

```yaml
prompt:
  update_pot: "update po/git.pot according to po/README.md"
agents:
  my-agent:
    cmd: ["my-agent", "--prompt", "{prompt}"]
```

2. Run the agent:

```bash
git-po-helper agent-run update-pot --agent my-agent
```

### Testing with Validation

1. Configure validation in `git-po-helper.yaml`:

```yaml
prompt:
  update_pot: "update po/git.pot according to po/README.md"
agent-test:
  runs: 5
  pot_entries_before_update: 5000
  pot_entries_after_update: 5100
agents:
  my-agent:
    cmd: ["my-agent", "--prompt", "{prompt}"]
```

2. Run tests:

```bash
git-po-helper agent-test update-pot --agent my-agent
```

## Troubleshooting

### "no agents configured"

**Problem**: No agents are defined in the configuration file.

**Solution**: Add at least one agent to `git-po-helper.yaml` in the `agents` section.

### "multiple agents configured, please specify --agent"

**Problem**: Multiple agents are configured but no agent was specified.

**Solution**: Use the `--agent` flag to specify which agent to use, or configure only one agent.

### "agent 'X' not found in configuration"

**Problem**: The specified agent name doesn't exist in the configuration.

**Solution**: Check the `agents` section in `git-po-helper.yaml` for available agent names.

### "pre-validation failed" or "post-validation failed"

**Problem**: Entry count validation failed.

**Solution**:
- Check that the POT file exists and has the expected number of entries
- Adjust the validation values in `git-po-helper.yaml` if needed
- Disable validation by setting values to `null` or `0` if you don't want validation

### "POT file validation failed"

**Problem**: The POT file has syntax errors.

**Solution**:
- Check the POT file syntax using `msgfmt --check-format po/git.pot`
- Fix any syntax errors reported
- Ensure the agent command correctly updates the POT file

## See Also

- [Design Document](design/agent-run-update-pot.md) - Detailed design and implementation notes
- [Main README](../README.md) - General git-po-helper documentation
