# 开源社区的 AI 实践：Git 本地化引入 AI Agent 的探索

## 引言：Git 本地化的 AI 辅助需求

自 2012 年起，我作为 Git 项目本地化协调者，参与了从 Git 1.7.10 到 2.53.0 共 60+ 版本的本地化集成工作。Git 1.7.10 仅包含中文本地化，由当时我公司内部几人众包完成，到目前为止 Git 支持 19 种语言，其中保持活跃更新的约 10 种。

对于 l10n coordinator 而言，保持多语种贡献质量始终是核心挑战，主要体现在以下几个方面：

1. **提交说明质量**：确保贡献者遵循约定，如 subject 以 "l10n:" 开头、不含非 ASCII 字符，长度遵循 50/72 原则等。

2. **符合语法规范**：格式损坏或不规范的翻译文件可能破坏 Git 构建。例如，使用高版本 gettext 生成的 obsolete 条目格式与低版本不兼容，导致 Git 在部分系统上构建失败；又如 Git 开发者在标注时的冲突——例如某单词先以普通单词标注，后又以复数形式双重标注，造成冲突。gettext 工具包可以捕获占位符重排错误，但对类型相同的占位符的重排错误无能为力，直至运行时异常。

3. **翻译质量**：没有AI的时代，一个人面对10多种语言，无法进行语义级别判断，仅能通过简单的正则匹配捕获翻译破坏变量、命令的错误。我最担心的是在翻译中夹带私货（广告、涉政言论）。

解决前两个方面的问题，开发了 [git-po-helper](https://github.com/git-l10n/git-po-helper) 对 Git 本地化翻译、提交进行质量检查，并集成到 GitHub Actions 流水线。

针对第三个问题，需要通过 AI 解决，而且引入 AI 辅助翻译和质量检查，可以提升效率，让本地化贡献者从打字员进化到审核员。春节前向 Git 社区提交了第一个设计，引发了一些激烈讨论。要在 Git 本地化中引入 AI，需要用数据说话。例如：

- 是在 `po/README.md` 中增加 AI Agent 指令，还是创建新的 `po/AGENTS.md` 文件？
- 增加更多的翻译质量规范，是否真的能提升翻译质量？
- 如何让模型更好地遵从翻译和评审的流程编排？
- 如何解决 PO 文件 diff 文件上下文丢失，降低评审成功率的问题？

## AI Coding 工具集成

为实现评测自动化，在 `git-po-helper` 中集成了 AI coding 工具，实现 AI coding 工具调用、结果实时展示与分析。

### 主流 AI coding CLI 工具集成效果

`git-po-helper` 已适配主流的 AI coding CLI 工具：Claude Code、Gemini CLI、Codex、OpenCode、Qwen 等。新增的 `agent-run` 和 `agent-test` 子命令，可以驱动 AI coding CLI 工具完成 Git 本地化日常维护的主要任务 update-pot、update-po、translate、review。

- **update-pot**：执行 `make po/git.pot` 从源码提取可翻译字符串，生成 POT 模板文件。
- **update-po**：执行 `make po-update PO_FILE=po/XX.po` 将 POT 模板文件中新字符串合并到各语言对应的 PO 文件。
- **translate**：对包含待翻译字符串的 PO 文件执行翻译，翻译参考文件中 `msgid`、`msgid_plural`（含复数形式）包含的英文字符串翻译为目标语言，保存在 `msgstr` 或 `msgstr_plural` 字段中。
- **review**：可以针对本地文件的修改、某个提交的修改、两个提交之间改动的本地化翻译进行评审，并根据发现问题的严重程度打分。

以下是使用 `git-po-helper agent-run` 子命令执行最简单的 `update-pot` 任务的过程输出；

```shell
$ git-po-helper agent-run update-pot -q --agent=claude-qwen3-o

🤖 System Initialization
==========================================
**Session ID:** 29bc508a-dcff-4da0-b9f8-f6cbb4c4ed16
**Model:** qwen3-coder-plus
**Working Dir:** /Users/jiangxin/work/git/git-po
**Version:** 2.1.37
**Tools:** 18
**Agents:** 5
==========================================

🤖 I'll help you update the "po/git.pot" file according to the instructions in "po/AGENTS.md". Let me
   first check if the file exists and then follow the appropriate task steps.
   According to the AGENTS.md file, Task 1 involves generating or updating po/git.pot by executing
   the command `make po/git.pot`.
   First, let me check the current status of the po/git.pot file:
🔧 Bash: command=ls -la po/git.pot, description=Check if po/git.pot file exists
💬 ... 412 bytes ...
🤖 The po/git.pot file doesn't exist yet. According to the AGENTS.md instructions for Task 1, I need
   to run the command `make po/git.pot` to generate or update the file.
🔧 Bash: command=make po/git.pot, description=Generate po/git.pot file
💬 ... 471 bytes ...
🤖 Great! I have successfully generated the po/git.pot file according to the instructions in
   po/AGENTS.md. The file was created with the command `make po/git.pot` as specified in Task 1 of
   the AGENTS.md document.
   Let me verify the file exists and check its content:
🔧 Read: file_path=/Users/jiangxin/work/git/git-po/po/git.pot
💬 ... 717 bytes ...
🤖 The po/git.pot file has been successfully generated. The file is quite large (over 592KB), which
   is normal for a template file containing all the translatable strings for Git.
   According to the instructions in po/AGENTS.md Task 1, I only needed to execute the command
   `make po/git.pot`, which has been completed successfully. The file generation was confirmed by the
   output showing "GEN po/git.pot" and the Git version information.
   The task is now complete - the po/git.pot file has been updated according to the instructions
   in po/AGENTS.md.
```

执行完毕显示报告和诊断信息：

```
✅ Final Result
==========================================
The po/git.pot file has been successfully generated. The file is quite large (over 592KB), which is normal for a template file containing all the translatable s
trings for Git.

According to the instructions in po/AGENTS.md Task 1, I only needed to execute the command `make po/git.pot`, which has been completed successfully. The file ge
neration was confirmed by the output showing "GEN po/git.pot" and the Git version information.

The task is now complete - the po/git.pot file has been updated according to the instructions in po/AGENTS.md.
==========================================

📊 Agent Diagnostics
==========================================
**Num turns:** 4
**Input tokens:** 66359
**Output tokens:** 431
**API duration:** 24.54 s
==========================================

Summary:
  Execution time: 33.407s
```

### 工具集成要点

通过命令行调用各 AI coding CLI 工具，命令和参数如下表所示：

| 工具         | 命令            | YOLO 类 参数（免确认/自动执行）      | 输出格式参数（Stream JSON 流式输出）        | 提示词参数      |
|-------------|----------------|----------------------------------|-----------------------------------------|----------------|
| Claude Code | `claude`       | `--dangerously-skip-permissions` | `--verbose --output-format stream-json` | `-p "<提示词>"` |
| Gemini CLI  | `gemini`       | `--yolo`                         | `--output-format stream-json`           | `"<提示词>"`    |
| Codex       | `codex exec`   | `--yolo`                         | `--json`                                | `"<提示词>"`    |
| OpenCode    | `opencode run` |                                  | `--format json`                         | `"<提示词>"`    |

说明：
- **YOLO 模式**：为减少人工确认环节，评测时通常启用工具的 "yolo" 或类似模式，让 Agent 自主执行命令，提高自动化程度。
- **Stream JSON 流式解析**：主流 AI coding CLI 工具支持 JSONL 格式的流式输出，即每一行是一个压缩的 JSON。不同的工具输出的 JSON 字段各异，同一个工具的不同会话类型的 JSON 字段各异。
- **诊断数据**：Claude 等工具在会话最后以 JSON 格式给出诊断信息。如果工具不在最后提供诊断信息，可以在会话过程中收集。`Num turns` 是 Claude 提供的诊断数据之一，表示模型与环境的交互轮次。轮次越少，说明指令越清晰、执行越高效，是评测的主要依据之一。

AI coding 流式输出 JSONL 数据解析放在以前工作量相当大，但是使用 AI coding 工具，通过提供工具文档、JSONL 输出示例，整个开发过程是非常愉悦的。

### 使用配置文件自定义 Agent 调用命令行

支持通过 YAML 配置文件，实现用户自定义 Agent。付费购买了阿里云[百炼 Coding Plan](https://www.aliyun.com/benefit/scene/codingplan)，设置了四个自定义 Agent，实现对主流开源大模型的接入。示例如下：

```yaml
agents:
    claude-qwen3:
        cmd:
            - claude
            - --dangerously-skip-permissions
            - --settings
            - /Users/jiangxin/.claude/settings-aliyun/settings.json-qwen3-coder-plus
            - -p
            - "{{.prompt}}"
        kind: claude
    claude-qwen3.5:
        cmd:
            - claude
            - --dangerously-skip-permissions
            - --settings
            - /Users/jiangxin/.claude/settings-aliyun/settings.json-qwen3.5-plus
            - -p
            - "{{.prompt}}"
        kind: claude
    claude-glm:
        cmd:
            - claude
            - --dangerously-skip-permissions
            - --settings
            - /Users/jiangxin/.claude/settings-aliyun/settings.json-glm-5
            - -p
            - "{{.prompt}}"
        kind: claude
    claude-minimax:
        cmd:
            - claude
            - --dangerously-skip-permissions
            - --settings
            - /Users/jiangxin/.claude/settings-aliyun/settings.json-MiniMax-M2.5
            - -p
            - "{{.prompt}}"
        kind: claude
```

说明：
- 命令行参数中的 `{{.prompt}}` 作为占位符，会在运行时用提示词替换。
- 运行时会根据 kind 设置的工具类型，自动在命令行中增加适配的 Stream JSON 相关参数。

## Agent 效果评估

如上文展示的 `git-po-helper agent-run` 子命令，将 Git 本地化的常用任务的执行和诊断输出封装为一条单独的任务。
在此基础上开发的 `git-po-helper agent-test` 子命令增加了重复执行、结果汇总分析。

利用这个工具我们解决了社区提出的疑问：将 AI Agent 指令放在 `po/README.md` 文件，还是放到单独的 `po/AGENTS.md` 文件？

**实验设计**：
- Before：将指令写在 po/README.md，prompt 引用 po/README.md
- After：将指令写在 po/AGENTS.md，使用引用 po/AGENTS.md 的 built-in prompt

使用 qwen 模型，各运行 5 次取平均。

### update-pot 任务

| 指标      | Before (po/README.md) | After (po/AGENTS.md) | 提升   |
|----------|------------------------|----------------------|-------|
| Turns    | 17                     | 3                    | -82%  |
| 执行时间   | 34s                   | 8s                   |  -76% |
| Turn 范围 | 3-36                   | 3-3                  | 更稳定 |

### update-po 任务

| 指标      | Before (po/README.md) | After (po/AGENTS.md) | 提升   |
|----------|------------------------|----------------------|-------|
| Turns    | 22                     | 4                    | -82%  |
| 执行时间  | 38s                    | 9s                    | -76% |
| Turn 范围 | 17-39                  | 3-9                  | 更稳定 |

结论：将 Agent 专用指令放在 po/AGENTS.md 中，带来了明显优势：

- **更聚焦、更简洁**：po/README.md 面向人类读者，内容庞杂；po/AGENTS.md 面向 AI，可针对任务做精简优化
- **更少冗余**：模型不必在冗长文档中筛选无关信息，直接执行指令
- **更一致的行为**：turn 范围从 3-36 收敛到 3-3，说明指令遵从性显著提升

这一数据支撑了我们在 Git 社区中采用 `po/AGENTS.md` 的决策。

## 流程编排

### 两套翻译流程

我们整合了两套翻译流程编排：

**流程一：基于 gettext 工具包**。数据流大致为：

```
po/XX.po → msgattrib 提取 → l10n-pending.po → awk 按条数裁剪 → l10n-todo.po
    → AI 翻译 → l10n-done.po → msgcat 合并 → po/XX.po
```

使用 `msgattrib` 提取未翻译和 fuzzy 条目，用 `msgcat` 合并，用 `awk` 裁剪过大的 PO 文件。问题在于：gettext 裁剪时需在 msgid/msgstr 组合边界处拆分，shell 脚本复杂；且 benchmark 发现指令遵从性不稳定，单条翻译的性能损耗较大。

**流程二：基于 git-po-helper**。引入 GETTEXT JSON 格式，将待翻译数据放在 JSON 中，便于批量处理。`git-po-helper msg-select` 支持按条目索引范围（如 `--range "-50"`、`--range "51-100"`）精确拆分，比 gettext 裁剪更简洁。详见 po/AGENTS.md 中的 shell 脚本。

### 流程路由：用代码决定

两套流程并存时，存在路由选择问题。若让 AI Agent 根据文档先做路由选择（根据工具存在与否选择流程），成功率不一定高。更稳妥的做法是：**用代码合并**——在脚本中自动检测 `git-po-helper` 是否存在，存在则走 JSON 流程，否则回退到 gettext 流程。po/AGENTS.md 中的 `po_extract_pending` 即采用此策略：

```shell
if command -v git-po-helper >/dev/null 2>&1
then
    git-po-helper msg-select --untranslated --fuzzy --no-obsolete -o "$PENDING" "$PO_FILE"
else
    msgattrib --untranslated --no-obsolete "$PO_FILE" >"${PENDING}.untranslated"
    msgattrib --only-fuzzy --no-obsolete --clear-fuzzy --empty "$PO_FILE" >"${PENDING}.fuzzy"
    # ... gettext 流程
fi
```

这样 Agent 无需决策，只需按步骤执行。

### 结构化数据返回

对于评审任务，需要大模型返回有问题的翻译条目及严重级别，便于打分。**结构化返回** 至关重要。我们定义了 Review result JSON 格式：

```json
{
  "issues": [
    {
      "msgid": "commit",
      "msgstr": "委托",
      "suggest_msgstr": "提交",
      "score": 0,
      "description": "Terminology error: 'commit' should be translated as '提交'"
    }
  ]
}
```

其中 score 0-3 表示严重程度（0=critical, 1=major, 2=minor, 3=perfect）。

### 错误处理的三级措施

大模型返回的 JSON 常有格式问题。git-po-helper 采用三级修复措施：

1. **检查 \`\`\`json 包裹**：脱掉 markdown 代码块标记
2. **不合法 JSON**：因遗漏引号、冒号等导致解析失败时，使用 gjson 尝试部分提取
3. **返回错误信息**：引导大模型修复后重试

### 上下文微调：如何让大模型不迷路

- **背景知识中嵌入 task 入口**：在 "Background knowledge" 中明确指向各 Task，避免模型在长文档中迷失
- **引言措辞**：不写 "translate"，而写 "housekeeping"，降低模型对任务性质的误判

## 翻译与评审的 Benchmark 数据

### 翻译任务

测试场景：po/zh_CN.po，127 条待翻译（91 fuzzy + 36 untranslated），每批 50 条。

| 流程 | 平均 Turns | 平均执行时间 | 成功率 |
|------|------------|--------------|--------|
| gettext 工具 | 86 | 20m44s | 3/3 |
| git-po-helper (JSON 批处理) | 56 | 19m8s | 3/3 |

git-po-helper 流程将 turns 从 86 降至 56（-35%），执行时间相近。瓶颈主要在 LLM 处理，而非网络交互。

### 评审任务

使用 `git-po-helper agent-run review --commit 2000abefba --agent qwen` 评测：

| 指标 | 值 |
|------|-----|
| Num turns | 22 |
| Input tokens | 537,263 |
| Output tokens | 4,397 |
| API duration | 167.84s |
| Review score | 96/100 |
| Total entries | 63 |
| With issues | 4 (1 critical, 2 major, 1 minor) |

评审工作流利用 `git-po-helper compare` 提取变更条目的完整上下文（完整 msgid/msgstr），避免 `git diff` 对多行条目的碎片化，显著提升了评审效率。

### 性能汇总

| 任务 | Before | After | 改进 |
|------|--------|-------|------|
| update-pot | 17 turns, 34s | 3 turns, 8s | -82% turns, -76% 时间 |
| update-po | 22 turns, 38s | 4 turns, 9s | -82% turns, -76% 时间 |
| translate | 86 turns | 56 turns | -35% turns（git-po-helper 流程） |
| review | N/A | 96/100 分 | 新工作流已文档化 |

## 社区对 AI Coding 的态度

### 开源贡献者承诺：Signed-off-by

引入 AI 辅助后，是否影响 Signed-off-by 的语义？社区对此有讨论。一种做法是在 commit-msg 过滤中识别并过滤 AI Agent 的签名，确保最终提交仍由人类背书。Junio（Git 维护者）等对此有独到见解：AI 是工具，人类对提交内容负责。

### 本地化翻译使用 AI？

**定位**：仅作为辅助。po/AGENTS.md 开篇即声明："Use of AI is optional; many successful l10n teams work well without it."

**反对声音**：例如葡萄牙语社区关注 pt_PT 与 pt_BR 的差异，担心全自动化会抹平地域特色。全自动化也容易引发社区恐慌。

**共识**：AI 生成的输出应视为草稿，必须经过理解技术语境和目标语言的人类审核、编辑和批准。最佳实践是结合 AI 效率与人类判断、文化洞察和社区参与。

## 总结

本文介绍了在 Git 本地化中引入 AI Agent 的实践：从质量保障需求出发，在 git-po-helper 中集成 AI coding 工具，通过 Benchmark 数据决策采用 po/AGENTS.md，并设计了可自动路由的流程编排和结构化返回格式。数据表明，po/AGENTS.md 可将 update-pot、update-po 的交互轮次降低约 82%，翻译任务降低约 35%，同时保持人类对翻译质量的最终把控。这一实践为开源社区探索 AI 辅助工作流提供了可复制的参考。
