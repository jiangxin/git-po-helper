#!/bin/sh

test_description="test git-po-helper agent-run review and agent-test review"

. ./lib/test-lib.sh

HELPER="po-helper --no-special-gettext-versions"

# Create a mock agent script that outputs review JSON
create_mock_review_agent() {
	cat >"$1" <<\EOF
#!/bin/sh
# Mock agent that outputs review JSON
# Usage: mock-review-agent --prompt "<prompt>" [<source>] [<commit>]

# Output a valid review JSON
cat <<JSON
{
  "total_entries": 100,
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
    },
    {
      "msgid": "The file has been modified",
      "msgstr": "文件已被修改了",
      "score": 2,
      "description": "风格问题：表达冗余",
      "suggestion": "文件已修改"
    }
  ]
}
JSON
exit 0
EOF
	chmod +x "$1"
}

test_expect_success "setup" '
	git clone "$PO_HELPER_TEST_REPOSITORY" workdir &&
	test -f workdir/po/git.pot &&
	test -f workdir/po/zh_CN.po &&
	# Create mock review agent
	create_mock_review_agent "$PWD/mock-review-agent" &&
	# Create config file
	cat >workdir/git-po-helper.yaml <<-EOF &&
default_lang_code: "zh_CN"
prompt:
  review_since: "review changes of {source} since commit {commit} according to po/AGENTS.md"
  review_commit: "review changes of commit {commit} according to po/AGENTS.md"
agents:
  mock:
    cmd: ["$PWD/mock-review-agent", "--prompt", "{prompt}"]
EOF
	# Replace $PWD with actual path in config
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak
'

test_expect_success "agent-run review: success with default mode (local changes)" '
	cat >workdir/git-po-helper.yaml <<-EOF &&
default_lang_code: "zh_CN"
prompt:
  review_since: "review changes of {source} since commit {commit} according to po/AGENTS.md"
  review_commit: "review changes of commit {commit} according to po/AGENTS.md"
agents:
  mock:
    cmd: ["$PWD/mock-review-agent", "--prompt", "{prompt}"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak &&

	git -C workdir $HELPER agent-run review >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should complete successfully
	grep "review completed successfully" actual &&
	# Should mention score
	grep "score.*100" actual &&
	# Should mention total entries
	grep "total entries.*100" actual &&
	# Should mention issues
	grep "issues.*3" actual &&
	# Should create JSON file
	test -f workdir/po/zh_CN-reviewed.json
'

test_expect_success "agent-run review: verify JSON file format" '
	test -f workdir/po/zh_CN-reviewed.json &&
	# Verify JSON is valid
	grep "\"total_entries\"" workdir/po/zh_CN-reviewed.json &&
	grep "\"issues\"" workdir/po/zh_CN-reviewed.json &&
	# Verify issues array has 3 items
	ISSUE_COUNT=$(grep -c "\"msgid\"" workdir/po/zh_CN-reviewed.json) &&
	test "$ISSUE_COUNT" = "3"
'

test_expect_success "agent-run review: success with explicit PO file" '
	git -C workdir $HELPER agent-run review po/zh_CN.po >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should complete successfully
	grep "review completed successfully" actual
'

test_expect_success "agent-run review: success with --commit flag" '
	cat >workdir/git-po-helper.yaml <<-EOF &&
default_lang_code: "zh_CN"
prompt:
  review_since: "review changes of {source} since commit {commit} according to po/AGENTS.md"
  review_commit: "review changes of commit {commit} according to po/AGENTS.md"
agents:
  mock:
    cmd: ["$PWD/mock-review-agent", "--prompt", "{prompt}"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak &&

	# Get a valid commit hash
	COMMIT=$(git -C workdir rev-parse HEAD) &&
	git -C workdir $HELPER agent-run review --commit "$COMMIT" po/zh_CN.po >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should complete successfully
	grep "review completed successfully" actual
'

test_expect_success "agent-run review: success with --since flag" '
	# Get a valid commit hash
	COMMIT=$(git -C workdir rev-parse HEAD~1 2>/dev/null || git -C workdir rev-parse HEAD) &&
	git -C workdir $HELPER agent-run review --since "$COMMIT" po/zh_CN.po >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should complete successfully
	grep "review completed successfully" actual
'

test_expect_success "agent-run review: error with both --commit and --since" '
	test_must_fail git -C workdir $HELPER agent-run review --commit HEAD --since HEAD~1 po/zh_CN.po >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should mention that only one can be specified
	grep "only one of.*--commit.*--since" actual
'

test_expect_success "agent-run review: agent command failure" '
	# Create a failing mock agent
	cat >"$PWD/failing-review-agent" <<-EOF &&
#!/bin/sh
echo "Agent failed" >&2
exit 1
EOF
	chmod +x "$PWD/failing-review-agent" &&

	cat >workdir/git-po-helper.yaml <<-EOF &&
default_lang_code: "zh_CN"
prompt:
  review_since: "review changes of {source} since commit {commit} according to po/AGENTS.md"
  review_commit: "review changes of commit {commit} according to po/AGENTS.md"
agents:
  failing:
    cmd: ["$PWD/failing-review-agent"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak &&

	test_must_fail git -C workdir $HELPER agent-run review >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should mention agent command failure
	grep "agent command failed" actual
'

test_expect_success "agent-run review: invalid JSON output" '
	# Create an agent that outputs invalid JSON
	cat >"$PWD/invalid-json-agent" <<-EOF &&
#!/bin/sh
echo "This is not valid JSON"
exit 0
EOF
	chmod +x "$PWD/invalid-json-agent" &&

	cat >workdir/git-po-helper.yaml <<-EOF &&
default_lang_code: "zh_CN"
prompt:
  review_since: "review changes of {source} since commit {commit} according to po/AGENTS.md"
  review_commit: "review changes of commit {commit} according to po/AGENTS.md"
agents:
  invalid:
    cmd: ["$PWD/invalid-json-agent"]
EOF
	sed -i.bak "s|\$PWD|$PWD|g" workdir/git-po-helper.yaml &&
	rm -f workdir/git-po-helper.yaml.bak &&

	test_must_fail git -C workdir $HELPER agent-run review >out 2>&1 &&
	make_user_friendly_and_stable_output <out >actual &&

	# Should mention JSON extraction or parsing failure
	grep -E "(extract JSON|parse.*JSON)" actual
'

test_done
