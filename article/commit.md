# Log of commit 1

docs(l10n): add AI agent instructions for translating PO files

Add a new "Translating po/XX.po" section to po/AGENTS.md with detailed
workflow and procedures for AI agents to translate language-specific PO
files. Users can invoke AI-assisted translation in coding tools with a
prompt such as:

    "Translate po/XX.po by referencing @po/AGENTS.md"

Translation results serve as a reference; human contributors must
review and approve before submission.

To address the low translation efficiency of some LLMs, batch
translation replaces entry-by-entry translation. git-po-helper
implements a gettext JSON format for translation files, replacing PO
format during translation to enable batch processing.

Evaluation with the Qwen model:

    git-po-helper agent-run --agent=qwen translate po/zh_CN.po

Test translation (127 entries, 50 per batch):

    Initial state:  5998 translated, 91 fuzzy, 36 untranslated
    Final state:    6125 translated, 0 fuzzy, 0 untranslated

    Successfully translated: 127 entries (91 fuzzy + 36 untranslated)
    Success rate: 100%

Benchmark results (3-run average):

AI agent using gettext tools:

    | Metric           | Value                          |
    |------------------|--------------------------------|
    | Avg. turns       | 86 (176, 44, 40)               |
    | Avg. Exec. time  | 20m44s (39m56s, 14m38s, 7m38s) |
    | Successful runs  | 3/3                            |

AI agent using git-po-helper (JSON batch flow):

    | Metric           | Value                          |
    |------------------|--------------------------------|
    | Avg. turns       | 56 (68, 39, 63)                |
    | Avg. Exec. time  | 19m8s (28m55s, 9m1s, 19m28s)   |
    | Successful runs  | 3/3                            |

The git-po-helper flow reduces the number of turns (86 → 56) with
comparable execution time; the bottleneck appears to be LLM processing
rather than network interaction.


# Log of commit 2

docs(l10n): add AI agent instructions to review translations

Add a new "Reviewing po/XX.po" section to po/AGENTS.md that provides
comprehensive guidance for AI agents to review translation files.

Translation diffs lose context, especially for multi-line msgid and
msgstr entries. Some LLMs ignore context and cannot evaluate
translations accurately; others rely on scripts to search for context
in source files, making the review process time-consuming. To address
this, git-po-helper implements a compare subcommand that extracts new
or modified translations with full context (complete msgid/msgstr
pairs), significantly improving review efficiency.

A limitation is that extracted content lacks other already translated
content for reference, which may affect terminology consistency. This
is mitigated by including a glossary in the PO file header.
git-po-helper-generated review files include the header entry and
glossary (if present) by default.

The review workflow leverages git-po-helper subcommands:

- git-po-helper compare: Extract new or changed entries between two PO
  file versions into a valid PO file for review. Supports multiple modes:

  * Compare HEAD with working tree (local changes)
  * Compare parent of commit with the commit (--commit)
  * Compare commit with working tree (--since)
  * Compare two arbitrary revisions (-r)

- git-po-helper msg-select: Split large review files into smaller
  batches by entry index range for manageable review sessions. Supports
  range formats like "-50" (first 50), "51-100", "101-" (to end).

Evaluation test using Qwen model:

    git-po-helper agent-run review --commit 2000abefba --agent qwen

Benchmark results:

    | Metric           | Value                            |
    |------------------|----------------------------------|
    | Num turns        | 22                               |
    | Input tokens     | 537263                           |
    | Output tokens    | 4397                             |
    | API duration     | 167.84 s                         |
    | Review score     | 96/100                           |
    | Total entries    | 63                               |
    | With issues      | 4 (1 critical, 2 major, 1 minor) |

Signed-off-by: Jiang Xin <worldhello.net@gmail.com>

