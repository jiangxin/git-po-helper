// Package util provides business logic for agent-run translate --use-local-orchestration.
package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/git-l10n/git-po-helper/config"
	"github.com/git-l10n/git-po-helper/repository"
	log "github.com/sirupsen/logrus"
)

const (
	l10nTodoBase   = "l10n-todo"
	l10nDoneBase   = "l10n-done"
	l10nMergedBase = "l10n-merged"
)

// RunAgentTranslateLocalOrchestration executes the local orchestration flow for translate.
// It uses msg-select, batch JSON files, agent per batch, msg-cat merge, and msgcat+msgfmt.
func RunAgentTranslateLocalOrchestration(cfg *config.AgentConfig, agentName, poFile string, batchSize int) (*AgentRunResult, error) {
	startTime := time.Now()
	result := &AgentRunResult{Score: 0}

	selectedAgent, err := SelectAgent(cfg, agentName)
	if err != nil {
		result.AgentError = err
		return result, err
	}

	poFile, err = GetPoFileAbsPath(cfg, poFile)
	if err != nil {
		return result, err
	}

	if !Exist(poFile) {
		return result, fmt.Errorf("PO file does not exist: %s\nHint: Ensure the PO file exists before running translate", poFile)
	}

	poDir := filepath.Dir(poFile)
	todoPO := filepath.Join(poDir, l10nTodoBase+".po")
	todoBase := filepath.Join(poDir, l10nTodoBase)
	doneBase := filepath.Join(poDir, l10nDoneBase)
	mergedPO := filepath.Join(poDir, l10nMergedBase+".po")

	filter := &EntryStateFilter{Untranslated: true, Fuzzy: true, NoObsolete: true}

	for {
		// Step 1: Condition check
		todoExists := Exist(todoPO)
		todoBatches, _ := globBatchPaths(todoBase, "todo")
		doneBatches, _ := globBatchPaths(doneBase, "done")

		if !todoExists {
			// Step 2: Generate pending file
			if err := generateTodoPO(poFile, todoPO, filter); err != nil {
				return result, err
			}
			entryCount, err := countContentEntries(todoPO)
			if err != nil {
				return result, err
			}
			if entryCount == 0 {
				log.Infof("no untranslated or fuzzy entries, translation complete")
				cleanupIntermediateFiles(poDir)
				result.Score = 100
				result.ExecutionTime = time.Since(startTime)
				return result, nil
			}
			continue
		}

		if len(todoBatches) > 0 {
			// Step 4: Translate each batch
			if err := translateBatches(cfg, selectedAgent, todoBase, doneBase, batchSize, result); err != nil {
				return result, err
			}
			continue
		}

		if len(doneBatches) > 0 {
			// Step 5: Merge batch results
			donePO := filepath.Join(poDir, l10nDoneBase+".po")
			if err := mergeDoneBatches(doneBatches, donePO); err != nil {
				return result, err
			}

			// Step 6: Complete translation
			if err := completeTranslation(donePO, poFile, mergedPO); err != nil {
				return result, err
			}

			cleanupStep6Files(poDir, todoPO, donePO, mergedPO)
			continue
		}

		// Step 3: Generate batch files
		if err := generateBatchJSONs(todoPO, todoBase, batchSize); err != nil {
			return result, err
		}
	}
}

func generateTodoPO(poFile, todoPO string, filter *EntryStateFilter) error {
	removeGlob(filepath.Dir(todoPO), l10nTodoBase+"-batch-*.json")
	removeGlob(filepath.Dir(todoPO), l10nDoneBase+"-batch-*.json")

	f, err := os.Create(todoPO)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", todoPO, err)
	}
	defer f.Close()

	if err := MsgSelect(poFile, "1-", f, false, filter); err != nil {
		os.Remove(todoPO)
		return fmt.Errorf("msg-select failed: %w", err)
	}
	log.Infof("generated %s", todoPO)
	return nil
}

func countContentEntries(poFile string) (int, error) {
	total, err := countMsgidEntriesInFile(poFile)
	if err != nil {
		return 0, err
	}
	if total > 0 {
		total-- // exclude header
	}
	return total, nil
}

func countMsgidEntriesInFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "msgid ") {
			count++
		}
	}
	return count, nil
}

func batchSizeFromFormula(entryCount, minBatchSize int) int {
	if entryCount <= minBatchSize*2 {
		return entryCount
	}
	if entryCount > minBatchSize*8 {
		return minBatchSize * 2
	}
	if entryCount > minBatchSize*4 {
		return minBatchSize + minBatchSize/2
	}
	return minBatchSize
}

func generateBatchJSONs(todoPO, todoBase string, minBatchSize int) error {
	entryCount, err := countContentEntries(todoPO)
	if err != nil {
		return err
	}
	if entryCount <= 0 {
		return nil
	}

	num := batchSizeFromFormula(entryCount, minBatchSize)
	batchCount := (entryCount + num - 1) / num
	poDir := filepath.Dir(todoPO)

	for i := 1; i <= batchCount; i++ {
		start := (i-1)*num + 1
		end := i * num
		if end > entryCount {
			end = entryCount
		}
		rangeSpec := formatTranslateRange(i, start, end, entryCount, num)
		batchPath := filepath.Join(poDir, fmt.Sprintf("%s-batch-%d.json", filepath.Base(todoBase), i))
		f, err := os.Create(batchPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", batchPath, err)
		}
		if err := WriteGettextJSONFromPOFile(todoPO, rangeSpec, f, nil); err != nil {
			f.Close()
			os.Remove(batchPath)
			return fmt.Errorf("write batch JSON %d: %w", i, err)
		}
		f.Close()
		log.Infof("prepared batch %d: entries %d-%d (of %d)", i, start, end, entryCount)
	}
	return nil
}

func formatTranslateRange(batchNum, start, end, entryCount, num int) string {
	if batchNum == 1 {
		return fmt.Sprintf("-%d", num)
	}
	if end >= entryCount {
		return fmt.Sprintf("%d-", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

func globBatchPaths(base string, kind string) ([]string, error) {
	dir := filepath.Dir(base)
	pattern := filepath.Join(dir, filepath.Base(base)+"-batch-*."+map[string]string{"todo": "json", "done": "json"}[kind])
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Slice(matches, func(i, j int) bool {
		ni := batchNumFromPath(matches[i])
		nj := batchNumFromPath(matches[j])
		return ni < nj
	})
	return matches, nil
}

func batchNumFromPath(path string) int {
	base := filepath.Base(path)
	// l10n-todo-batch-1.json or l10n-done-batch-1.json
	idx := strings.LastIndex(base, "-batch-")
	if idx < 0 {
		return 0
	}
	suffix := base[idx+7:]
	suffix = strings.TrimSuffix(suffix, ".json")
	n, _ := strconv.Atoi(suffix)
	return n
}

func translateBatches(cfg *config.AgentConfig, selectedAgent config.Agent, todoBase, doneBase string, batchSize int, result *AgentRunResult) error {
	prompt, err := GetRawPrompt(cfg, "local-orchestration-translation")
	if err != nil {
		return err
	}

	poDir := filepath.Dir(todoBase)
	todoBatches, err := globBatchPaths(todoBase, "todo")
	if err != nil || len(todoBatches) == 0 {
		return nil
	}

	workDir := repository.WorkDirOrCwd()

	for _, todoPath := range todoBatches {
		n := batchNumFromPath(todoPath)
		donePath := filepath.Join(poDir, fmt.Sprintf("%s-batch-%d.json", filepath.Base(doneBase), n))

		sourceRel, _ := filepath.Rel(workDir, todoPath)
		destRel, _ := filepath.Rel(workDir, donePath)
		if sourceRel == "" || sourceRel == "." {
			sourceRel = todoPath
		}
		if destRel == "" || destRel == "." {
			destRel = donePath
		}
		sourceRel = filepath.ToSlash(sourceRel)
		destRel = filepath.ToSlash(destRel)

		batchVars := PlaceholderVars{
			"prompt": prompt,
			"source": sourceRel,
			"dest":   destRel,
		}
		resolvedPrompt, err := ExecutePromptTemplate(prompt, batchVars)
		if err != nil {
			return fmt.Errorf("failed to resolve prompt template: %w", err)
		}
		batchVars["prompt"] = resolvedPrompt

		agentCmd, err := BuildAgentCommand(selectedAgent, batchVars)
		if err != nil {
			return fmt.Errorf("failed to build agent command: %w", err)
		}

		outputFormat := selectedAgent.Output
		if outputFormat == "" {
			outputFormat = "default"
		}
		outputFormat = normalizeOutputFormat(outputFormat)

		log.Infof("translating batch %d: %s -> %s (output=%s, streaming=%v)", n, sourceRel, destRel, outputFormat, outputFormat == "json")
		result.AgentExecuted = true

		var stderr []byte
		stdoutReader, stderrBuf, cmdProcess, execErr := ExecuteAgentCommandStream(agentCmd)
		if execErr != nil {
			return fmt.Errorf("agent command failed: %w\nHint: Check that the agent command is correct and executable", execErr)
		}
		defer stdoutReader.Close()
		// Parse stream to display agent output (agent writes to {{.dest}}; we don't use parsed stdout)
		_, streamResult, _ := parseStreamByKind(selectedAgent.Kind, stdoutReader)
		applyAgentDiagnostics(result, streamResult)
		waitErr := cmdProcess.Wait()
		stderr = stderrBuf.Bytes()
		if waitErr != nil {
			if len(stderr) > 0 {
				log.Debugf("agent stderr: %s", string(stderr))
			}
			result.AgentError = fmt.Errorf("agent command failed: %v (see logs for agent stderr output)", waitErr)
			return fmt.Errorf("agent failed for batch %d: %w", n, waitErr)
		}

		if !Exist(donePath) {
			return fmt.Errorf("agent did not create output file %s\nHint: The agent must write the translated JSON to {{.dest}}", destRel)
		}

		if _, err := ReadFileToGettextJSON(donePath); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", destRel, err)
		}

		os.Remove(todoPath)
		log.Infof("translated batch %d", n)
	}
	return nil
}

func mergeDoneBatches(donePaths []string, outputPath string) error {
	sources := make([]*GettextJSON, 0, len(donePaths))
	for _, p := range donePaths {
		j, err := ReadFileToGettextJSON(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		sources = append(sources, j)
	}
	merged := MergeGettextJSON(sources)
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outputPath, err)
	}
	defer f.Close()
	if err := WriteGettextJSONToPO(merged, f); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("write merged PO: %w", err)
	}
	for _, p := range donePaths {
		os.Remove(p)
	}
	log.Infof("merged %d batches to %s", len(donePaths), outputPath)
	return nil
}

func completeTranslation(donePO, targetPO, mergedPO string) error {
	cmd := exec.Command("msgcat", "--use-first", donePO, targetPO, "-o", mergedPO)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("msgcat failed: %w\n%s", err, string(out))
	}

	cmd = exec.Command("msgfmt", "--check", "-o", os.DevNull, mergedPO)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(mergedPO)
		return fmt.Errorf("msgfmt validation failed: %w\n%s", err, string(out))
	}

	if err := os.Rename(mergedPO, targetPO); err != nil {
		os.Remove(mergedPO)
		return fmt.Errorf("failed to replace %s: %w", targetPO, err)
	}
	log.Infof("merged translations into %s", targetPO)
	return nil
}

func cleanupStep6Files(poDir, todoPO, donePO, mergedPO string) {
	os.Remove(todoPO)
	os.Remove(donePO)
	os.Remove(mergedPO)
}

func cleanupIntermediateFiles(poDir string) {
	removeGlob(poDir, l10nTodoBase+".po")
	removeGlob(poDir, l10nTodoBase+"-batch-*.json")
	removeGlob(poDir, l10nDoneBase+"-batch-*.json")
	removeGlob(poDir, l10nDoneBase+".po")
	removeGlob(poDir, l10nMergedBase+".po")
}

func removeGlob(dir, pattern string) {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	for _, m := range matches {
		os.Remove(m)
	}
}
