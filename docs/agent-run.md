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
  review_since: "review changes of {source} since commit {commit} according to po/README.md"
  review_commit: "review changes of commit {commit} according to po/README.md"
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
- `prompt.review_since`: Prompt for reviewing changes since a commit (uses `{source}` and `{commit}` placeholders)
- `prompt.review_commit`: Prompt for reviewing a specific commit (uses `{source}` and `{commit}` placeholders)

#### Agent Test Configuration

- `agent-test.runs`: Default number of runs for `agent-test` (default: 5)
- `agent-test.pot_entries_before_update`: Expected entry count before update (null or 0 to disable)
- `agent-test.pot_entries_after_update`: Expected entry count after update (null or 0 to disable)
- `agent-test.po_entries_before_update`: Expected PO entry count before update (for future use)
- `agent-test.po_entries_after_update`: Expected PO entry count after update (for future use)
- `agent-test.po_new_entries_after_update`: Expected new PO entries after update (for future use)
- `agent-test.po_fuzzy_entries_after_update`: Expected fuzzy PO entries after update (for future use)

#### Agents

Each agent is defined with a name and a command. The command is a list of strings where placeholders are replaced:

- `{prompt}`: Replaced with the actual prompt text
- `{source}`: Replaced with the source file path (PO file)
- `{commit}`: Replaced with the commit ID (default: HEAD)

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
