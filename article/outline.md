**题目：《开源社区的 AI 实践》**

# 引言：Git 本地化的 AI辅助需求

从 2012 年起我作为 Git 项目本地化协调者，参与了从 1.7.10~2.53.0  64个Git版本的本地化集成。
git 1.7.10 只包含中文本地化翻译，是我当时自己的公司几个人众包完成。在 2021 年之后将中文本地化 leader 工作陆续移交给两任的中文本地化负责人，我的主要工作是多语种的评审和集成。目前有 19 个语言翻译，保持活跃更新的10个。

对于 l10n coodinator 来说，如何帮助做质量保障
提交说明质量
不会破坏 Git 构建流水线
 - 翻译文件使用高版本 gettext，obsolete 条目格式和低版本不兼容，导致 git 在部分系统中构建失败。
 - Git 开发者在标注时的冲突，例如 commit 这个单词，以单数形式标注，后又以 plural 方式标注造成冲突
 - gettext 工具包可以捕获占位符错误的问题，但是类型相同的占位符无能为力。

 翻译质量，甚至翻译中是否夹杂私货？
    git-po-helper 做质量检查，以及 GitHub Actions 流水线
    一直有个想法，引入 AI 辅助翻译和质量检查。
    春节前向 Git 社区发了第一版的代码，有一些激烈的碰撞。想要在 Git 本地化引入 AI，需要数据说话。
    例如
        是在 po/README.md 中增加 AI Agent 指令，还是创建新的 po/AGENTS.md？
        对翻译的质量标准提供详细说明是否会提升翻译质量？
        如何让模型更好地遵从翻译和评审的流程编排？如何更加科学编排，解决 diff 文件上下文丢失问题？

# AI coding 工具集成
    为实现评测自动化，在 git-po-helper 中集成了 AI coding 工具，工具调用、结果的实时展示和分析。
    效果展示：运行时实时展示 AI coding 的思维连、工具调用
    已经适配主流 ai coding 工具：claude code、gemini-cli、codex、opencode、qwen
    集成的要点
        yolo 模式
        stream json 流式数据解析
        claude 结果数据返回包含的 Num turns, 理解为模型交互次数，是评测的主要依据之一。
    支持通过配置文件自定义 agent
        通过自定义 agent，实现一个 agent，对接不同模型
        例如 claude-qwen3，claude-qwen3.5
        实现对不同模型的测试。
        阿里云百炼 codeplan：
            https://www.aliyun.com/benefit/scene/codingplan
            利用百炼的 codeplan，测试了 glm5、minimax 等模型

# 使用Benchmark数据决策采用 po/AGENTS.md
    解答社区的疑问：po/README.md 还是 po/AGENTS.md
    先针对两个简单的本地化任务：生成 po/git.pot ，用 po/git.pot 更新本地化文件 po/XX.po。放在 README.md 中
    最初的想法，因为 po/README.md 文件中写了很多本地化开发者的背景知识，相比另起炉灶（po/AGENTS.md）更好。
    实时的验证结果出乎意料。

# 流程编排
    利用脚本整合两套翻译流程编排
        第一个实现的流程编码是使用 gettext 工具包，辅助翻译
            画一个简单的流程图
        在 benchmark 时发现不稳定，一个是指令遵从性，一个是单条翻译的性能损耗。
        另一个流程是利用 git-po-helper 开发的新的子命令能力，解决传统翻译流程问题
            GETTEXT JSON 格式：将待翻译数据放在 JSON 里面，实现自动化翻译。
            git-po-helper msg-select 实现更加简单的筛选
                使用 gettext 工具包裁剪过大的 PO 文件，恰好在一个 msgid/mgstr 的组合之后拆分文件
                shell 脚本参见 po/AGENTS.md
                使用 git-po-helper msg-select 更加简洁
        两个流程的存在路由选择的问题，要让 AI Agent 根据文档先走路由选择，根据工具存在与否选择指定的翻译流程，但是成功率不一定高。为什么不将两个流程合并在同一个流程？用代码来决定？
            流程中的代码示例：自动根据 git-po-helper 存在与否进行分支选择
    使用结构化数据返回
        对于评审任务，需要大模型返回有问题的翻译条目，以及严重级别，便于打分，结构化返回很重要
        定义了返回的 JSON 数据结构
        对返回数据的错误处理
            在 git-po-helepr 代码中采用三级措施对代码修复
            检查 ```json 包裹字符，脱掉
            不合法的 json，因为丢掉引号和冒号。使用 gjson
            返回错误信息，引导大模型修复
    上下文微调
        如何让大模型不迷路
            背景知识中，嵌入 task 入口
            引言，不写 translate，写 housekeeping

# 社区对 AI coding 的态度
    开源贡献者承诺：s-o-b
        引入是否有问题
        commit-msg 过滤 ai agent 的签名
        junio 的见解
    本地化翻译使用 AI？
        仅作为辅助
        反对声音
            葡萄牙语的 pt_PT 和 pt_BR 的差异
            全自动化，引发社区恐慌
